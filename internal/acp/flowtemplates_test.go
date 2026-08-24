package acp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The board template and the seeded routes are two halves of one promise: open
// a board from the developer template and the routes already point at columns that
// exist, with a "Workflow" property whose options name them. Nothing in the
// build enforces that — a template is JSON nobody compiles — so the test reads
// the templates themselves and compares.

const templateBoardTitle = "Development"

// The property a card picks its route with. The template boards are written in
// English and translated on the way in (internal/boardadapter/templatetext.go),
// so this is the file's word and not the one on anybody's screen.
const templateRouteProperty = "Workflow"

type templateProperty struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Options []struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	} `json:"options"`
}

type templateBoard struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Fields struct {
		CardProperties []templateProperty `json:"cardProperties"`
		Properties     struct {
			Columns         []ColumnSpec `json:"columns"`
			Flows           []FlowEntry  `json:"flows"`
			Setup           BoardSetup   `json:"setup"`
			OutcomeProperty string       `json:"outcomeProperty"`
		} `json:"properties"`
	} `json:"fields"`
}

// templatesDir is where the boards this app ships live. They are read as files
// rather than through boardadapter, which imports the board server: this package
// is board-agnostic and must stay buildable without it.
const templatesDir = "../boardadapter/templates"

// readTemplateBoards reads every board this app ships as a template.
func readTemplateBoards(t *testing.T) []templateBoard {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no board templates under %s", templatesDir)
	}
	// And the ones only a paid edition embeds (internal/edition). Read off
	// disk rather than out of an embed, so they are checked whichever tag this
	// test binary was built with: a template is data, and data that only one
	// build validates is data nobody validates.
	editions, err := filepath.Glob(filepath.Join(templatesDir, "*", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, editions...)
	var boards []templateBoard
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1<<20), 1<<22)
		for sc.Scan() {
			var rec struct {
				Data templateBoard `json:"data"`
			}
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				continue
			}
			if rec.Data.Type == "board" {
				boards = append(boards, rec.Data)
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	return boards
}

// readTemplateBoard finds the board template the seeded routes are written for.
func readTemplateBoard(t *testing.T) templateBoard {
	t.Helper()
	for _, board := range readTemplateBoards(t) {
		if board.Title == templateBoardTitle {
			return board
		}
	}
	t.Fatalf("the %q template is gone from the archive", templateBoardTitle)
	return templateBoard{}
}

// property is one of the board's card fields, by name — for the checks that are
// about the field itself rather than about its options.
func (b templateBoard) property(name string) (templateProperty, bool) {
	for _, p := range b.Fields.CardProperties {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return templateProperty{}, false
}

func (b templateBoard) options(t *testing.T, property string) map[string]bool {
	t.Helper()
	for _, p := range b.Fields.CardProperties {
		if strings.EqualFold(p.Name, property) {
			out := make(map[string]bool, len(p.Options))
			for _, o := range p.Options {
				out[strings.ToLower(o.Value)] = true
			}
			return out
		}
	}
	t.Fatalf("the %q template has no %q property", templateBoardTitle, property)
	return nil
}

// optionOf finds the option a spec claims to be bound to and reports whether the
// binding holds: the property exists, it is the one named, and the option under
// that id is the column the spec means.
func (b templateBoard) optionOf(propertyID, optionID string) (property, value string, ok bool) {
	for _, p := range b.Fields.CardProperties {
		if propertyID != "" && p.ID != propertyID {
			continue
		}
		for _, o := range p.Options {
			if o.ID == optionID {
				return p.Name, o.Value, true
			}
		}
	}
	return "", "", false
}

// selectValues is every option a card can wear on this board — which is how a
// card picks its route (see resolveFlow).
func (b templateBoard) selectValues() map[string]bool {
	out := map[string]bool{}
	for _, p := range b.Fields.CardProperties {
		for _, o := range p.Options {
			out[strings.ToLower(o.Value)] = true
		}
	}
	return out
}

// Every template that ships automation has to ship one that runs: the stages
// must stand on options this very board has, and the route must be pickable —
// a card takes a route by wearing its name as an option. The everyday-life
// boards were written by hand into JSON the build never looks at, so nothing
// else would catch a typo in them before somebody dragged a card and got
// silence.
func TestEveryTemplateShipsWorkingAutomation(t *testing.T) {
	shipping := 0
	for _, board := range readTemplateBoards(t) {
		columns := board.Fields.Properties.Columns
		flows := board.Fields.Properties.Flows
		if len(columns) == 0 && len(flows) == 0 {
			continue // an upstream template: it brings no automation at all
		}
		shipping++
		t.Run(board.Title, func(t *testing.T) {
			for _, c := range columns {
				if _, err := validateColumn(c, nil, nil); err != nil {
					t.Errorf("column %q: %v", c.Column, err)
				}
				property, value, ok := board.optionOf(c.PropertyID, c.OptionID)
				switch {
				case !ok:
					t.Errorf("column %q is bound to option %q, which this board does not have", c.Column, c.OptionID)
				case !strings.EqualFold(property, c.Property):
					t.Errorf("column %q says it is on %q, but its option belongs to %q", c.Column, c.Property, property)
				case !strings.EqualFold(value, c.Column):
					t.Errorf("column %q is bound to the option named %q", c.Column, value)
				}
			}

			picks := board.selectValues()
			for _, f := range flows {
				// validateFlow is what the registry itself runs on a route taken
				// from a board, and it already rejects a dangling edge, an
				// unknown trigger and two stages on one column.
				if _, err := validateFlow(f, nil, nil, nil); err != nil {
					t.Errorf("route %q: %v", f.Name, err)
				}
				if !picks[strings.ToLower(f.Name)] {
					t.Errorf("route %q: no option of the board names it, so no card can pick it", f.Name)
				}
				for _, n := range f.Nodes {
					_, value, ok := board.optionOf("", n.OptionID)
					if !ok {
						t.Errorf("route %q stage %q stands on option %q, which this board does not have", f.Name, n.ID, n.OptionID)
						continue
					}
					if !strings.EqualFold(value, n.Column) {
						t.Errorf("route %q stage %q says %q but stands on the option named %q", f.Name, n.ID, n.Column, value)
					}
				}
			}
		})
	}
	if shipping == 0 {
		t.Fatal("no template ships automation any more — a board made from one arrives empty")
	}
}

func TestTemplateFlowsMatchTheBoardTemplate(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	board := readTemplateBoard(t)
	flows := TemplateFlows(cfg)

	// Every stage must have somewhere to move a card to. A column that is not
	// an option of the board is a route that fails on its first transition.
	columns := board.options(t, cfg.TriggerProperty)
	for _, f := range flows {
		for _, n := range f.Nodes {
			if !columns[strings.ToLower(n.Column)] {
				t.Errorf("flow %q: the %q template has no %q column — add the option or rename the stage",
					f.Name, templateBoardTitle, n.Column)
			}
		}
	}

	// And the property a card picks its route with must offer exactly the
	// routes that exist: an option naming nothing does nothing.
	picks := board.options(t, templateRouteProperty)
	names := make(map[string]bool, len(flows))
	for _, f := range flows {
		names[strings.ToLower(f.Name)] = true
		if !picks[strings.ToLower(f.Name)] {
			t.Errorf("flow %q is seeded but the template's %q property does not offer it", f.Name, templateRouteProperty)
		}
	}
	for option := range picks {
		if !names[option] {
			t.Errorf("the template offers the workflow %q, which no seeded route answers to", option)
		}
	}
}

// The board brings its own automation, and the Go side is where the shipped
// routes are written. Nothing in the build ties the two together, so the test
// does: what the template carries must be what TemplateFlows says, bound to the
// template's own option ids.
func TestTemplateBoardCarriesTheShippedAutomation(t *testing.T) {
	cfg := DefaultConfig(t.TempDir())
	board := readTemplateBoard(t)
	columns := board.Fields.Properties.Columns
	flows := board.Fields.Properties.Flows

	// The three columns that do something, each bound to a real option.
	options := map[string]string{} // lowercased name → option id
	for _, p := range board.Fields.CardProperties {
		if strings.EqualFold(p.Name, cfg.TriggerProperty) {
			for _, o := range p.Options {
				options[strings.ToLower(o.Value)] = o.ID
			}
		}
	}
	wantActions := map[string]string{
		strings.ToLower(TemplateWorkColumn):   FlowActionAgent,
		strings.ToLower(TemplateDeployColumn): FlowActionDeploy,
		strings.ToLower(TemplateTestColumn):   FlowActionTest,
	}
	if len(columns) != len(wantActions) {
		t.Fatalf("the template ships %d columns: %+v", len(columns), columns)
	}
	for _, c := range columns {
		lower := strings.ToLower(c.Column)
		if want := wantActions[lower]; want != c.Action {
			t.Errorf("column %q does %q, expected %q", c.Column, c.Action, want)
		}
		if c.OptionID == "" || c.OptionID != options[lower] {
			t.Errorf("column %q is not bound to the template's own option: %q", c.Column, c.OptionID)
		}
	}

	// The routes are the shipped ones, stage for stage, and every stage stands
	// on a column of this board.
	want := TemplateFlows(cfg)
	if len(flows) != len(want) {
		t.Fatalf("the template ships %d routes, TemplateFlows has %d", len(flows), len(want))
	}
	for i, f := range flows {
		if f.Name != want[i].Name || len(f.Nodes) != len(want[i].Nodes) || len(f.Edges) != len(want[i].Edges) {
			t.Fatalf("route %q differs from the shipped one: %+v", f.Name, f)
		}
		for j, n := range f.Nodes {
			if n.Column != want[i].Nodes[j].Column {
				t.Errorf("route %q stage %d: column %q, expected %q", f.Name, j+1, n.Column, want[i].Nodes[j].Column)
			}
			if n.OptionID != options[strings.ToLower(n.Column)] {
				t.Errorf("route %q stage %q is not bound to an option of the board", f.Name, n.Column)
			}
			// A stage that repeats what its column already says is noise.
			if n.Action == FlowActionAgent || n.Action == FlowActionDeploy || n.Action == FlowActionTest {
				t.Errorf("route %q stage %q repeats the column's action %q", f.Name, n.Column, n.Action)
			}
		}
	}
}

// A template says which questions the machine has to answer before its
// automation can run, and it may only ask for steps this build can carry out —
// a step nobody implements is a screen the wizard silently drops.
func TestEveryTemplateAsksForStepsThatExist(t *testing.T) {
	for _, board := range readTemplateBoards(t) {
		steps := board.Fields.Properties.Setup.Steps
		if len(steps) == 0 {
			t.Errorf("%q asks for nothing, so its setup is guessed from its columns", board.Title)
			continue
		}
		t.Run(board.Title, func(t *testing.T) {
			seen := map[string]bool{}
			for _, step := range steps {
				if _, ok := SetupStepDefinition(step.Kind); !ok {
					t.Errorf("asks for %q, which no build can do", step.Kind)
				}
				if seen[step.Kind] {
					t.Errorf("asks for %q twice", step.Kind)
				}
				seen[step.Kind] = true
			}

			// Nothing runs without a folder and an agent, whatever else the
			// board does or does not need.
			for _, required := range []string{SetupStepWorkdir, SetupStepAgent} {
				if !seen[required] {
					t.Errorf("does not ask for %q, without which nothing runs", required)
				}
			}
			if steps[len(steps)-1].Kind != SetupStepDone {
				t.Errorf("ends on %q rather than telling the person how to use what they set up", steps[len(steps)-1].Kind)
			}

			// And it must ask for what its own automation needs: a board with a
			// deploy stage and no deploy step is one whose first publish fails
			// with a registry nobody was asked to fill.
			for _, c := range board.Fields.Properties.Columns {
				switch c.Action {
				case FlowActionDeploy:
					if !seen[SetupStepDeploy] {
						t.Errorf("runs a deploy in %q but never asks where to deploy to", c.Column)
					}
				case FlowActionTest:
					if !seen[SetupStepBrowser] {
						t.Errorf("tests in %q but never asks what to test with", c.Column)
					}
				}
			}
		})
	}
}

// A machine stage's value has to have somewhere to land, and «Разработка» is
// where somebody meets that arrangement first. Without a property to write to,
// the preview address and the verdict die in a comment: the whole dataflow —
// the card's own fields, the arrow that branches on one, the brief the next
// stage opens with — starts from a stage declaring an output, and a board that
// declares none teaches that there is nothing to declare.
func TestTheDeveloperTemplateDeclaresWhatItsMachineStagesProduce(t *testing.T) {
	board := readTemplateBoard(t)

	wantWrites := map[string]string{
		strings.ToLower(TemplateDeployColumn): "Preview",
	}
	for _, c := range board.Fields.Properties.Columns {
		want, machine := wantWrites[strings.ToLower(c.Column)]
		if !machine {
			continue
		}
		if len(c.Writes) != 1 || c.Writes[0].Property != want {
			t.Errorf("column %q writes %+v, want just %q", c.Column, c.Writes, want)
			continue
		}

		// Not "required": that is the contract an agent's finish_work is
		// checked against, and these two values are written by the app itself.
		// The editor's picker for a machine stage cannot show the flag, so a
		// template carrying one would set something nobody can see or clear.
		if c.Writes[0].Required {
			t.Errorf("column %q marks its machine output required, which nothing reads", c.Column)
		}
	}

	// And the property itself, since a write naming a field the board has not
	// got is refused at the moment the stage reports — after the work.
	if _, ok := board.property("Preview"); !ok {
		t.Error("the template writes a preview address to a property it does not carry")
	}
}

// How a stage ended is a fact about the card. The template used to say it by
// having the card land in a checked, a failed and a blocked column — three
// columns whose whole content was one fact about the stage before them, and
// which then read as places where work happens, on the board and on the canvas
// alike. The outcome is a field the engine writes for every stage, and those
// columns are gone.
func TestTheDeveloperTemplateKeepsOutcomesOffTheBoard(t *testing.T) {
	board := readTemplateBoard(t)

	for _, gone := range []string{"Checked", "Failed", "Blocked"} {
		if board.options(t, DefaultTriggerProperty)[strings.ToLower(gone)] {
			t.Errorf("«%s» is still a column: an outcome is not a place work happens", gone)
		}
	}

	outcome, ok := board.property("Исход")
	if !ok {
		t.Fatal("the template has no field for a stage's outcome")
	}

	// A closed set, because an arrow asks "is it this value" and that is the
	// point of writing it — and the values are the engine's own (OutcomeValue).
	if outcome.Type != "select" {
		t.Errorf("the outcome is a %q, so no transition can ask what it is", outcome.Type)
	}
	// Two values, and only two: what the card carries here is whether the step
	// worked, and «заблокировано» is a reason rather than a third answer to
	// that. A reviewer marking a card by hand picks between two things.
	values := board.options(t, "Исход")
	for _, want := range []string{OutcomePassed, OutcomeFailed} {
		if !values[want] {
			t.Errorf("the outcome cannot be %q", want)
		}
	}
	if len(values) != 2 {
		t.Errorf("the outcome has %d values: %v", len(values), values)
	}

	// And the board says which field it is, or the engine writes nowhere.
	if board.Fields.Properties.OutcomeProperty != outcome.ID {
		t.Errorf("the board records %q as its outcome field, want %q",
			board.Fields.Properties.OutcomeProperty, outcome.ID)
	}
}

// A board keeps the routes it was made with, so a route written before the way
// onto a route existed has no arrow into its first stage — and the question
// asked on the way in is exactly the one such a board would never ask. It is
// repaired where the board is read.
func TestARouteWrittenWithoutAWayInGetsOne(t *testing.T) {
	flows := []FlowEntry{{
		Name:  "old",
		Nodes: []FlowNode{{ID: "agent"}, {ID: "review"}},
		Edges: []FlowEdge{{From: "agent", To: "review", On: TriggerSuccess}},
	}}
	if added := addEntryTransitions(flows); added != 1 {
		t.Fatalf("added %d ways in, expected 1", added)
	}
	edge := flows[0].EdgeBetween(EntryNode, "agent")
	if edge == nil {
		t.Fatalf("no way into the route's first stage: %+v", flows[0].Edges)
	}
	// Both questions, in the order they can be answered: a card coming from
	// off the route has usually named no folder, and until it has, how to work
	// that folder is about nothing.
	if len(edge.Ask) != 2 || edge.Ask[0].Kind != AskWorkdir || edge.Ask[1].Kind != AskWorkMode {
		t.Fatalf("the way in asks something else: %+v", edge.Ask)
	}
}

// And one somebody has since edited is left as it is: this fills in what is
// missing rather than restating what the shipped route says.
func TestARouteThatAlreadyHasAWayInIsLeftAlone(t *testing.T) {
	flows := []FlowEntry{{
		Name:  "edited",
		Nodes: []FlowNode{{ID: "agent"}},
		Edges: []FlowEdge{{From: EntryNode, To: "agent", Ask: []EdgeAsk{{ID: "mine", Kind: AskProperty, Property: "Приоритет"}}}},
	}}
	if added := addEntryTransitions(flows); added != 0 {
		t.Fatalf("added %d ways in to a route that had one", added)
	}
	if q := flows[0].Edges[0].Ask; len(q) != 1 || q[0].ID != "mine" {
		t.Fatalf("somebody's own question was overwritten: %+v", q)
	}
}

// A route with no stages has no first stage to lead into, and must not grow an
// edge pointing at nothing — which is what the engine refuses to save.
func TestARouteWithNoStagesGetsNoWayIn(t *testing.T) {
	flows := []FlowEntry{{Name: "empty"}}
	if added := addEntryTransitions(flows); added != 0 || len(flows[0].Edges) != 0 {
		t.Fatalf("a route with no stages was given a way in: %+v", flows[0])
	}
}
