package boardadapter

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// A template is one set of files and a catalogue of words beside it, so the two
// go out of step the moment somebody edits a board: a column added to a
// template is a column with no English name, and nothing on screen says so.
// This is what says so.
func TestEveryTemplateStringIsTranslated(t *testing.T) {
	strs := everyTemplateString(t)
	if len(strs) == 0 {
		t.Fatal("no strings read out of the templates at all")
	}
	for lang, cat := range catalogues(t) {
		for _, s := range strs {
			if strings.TrimSpace(cat[s]) == "" {
				t.Errorf("%s has no translation for %q", lang, short(s))
			}
		}
	}
}

// The other direction: a string somebody removed from a template leaves a line
// in every catalogue that no longer translates anything, and a translator has
// no way to tell it from work still to do.
func TestACatalogueSaysNothingTheTemplatesDoNot(t *testing.T) {
	strs := everyTemplateString(t)
	said := make(map[string]bool, len(strs))
	for _, s := range strs {
		said[s] = true
	}
	for lang, cat := range catalogues(t) {
		for s := range cat {
			if !said[s] {
				t.Errorf("%s translates %q, which no template says any more", lang, short(s))
			}
		}
	}
}

// What the app matches on by name is not a person's word to change: translating
// «Входящие» would have the next arriving card make a second inbox beside the
// translated one, and translating «прошло» would leave the engine writing an
// outcome the board has no option for.
func TestTheNamesTheAppMatchesOnAreNeverTranslated(t *testing.T) {
	for lang, cat := range catalogues(t) {
		for name := range appGivenNames {
			if _, ok := cat[name]; ok {
				t.Errorf("%s translates %q, which this app finds by name", lang, name)
			}
			if got := translateText(cat, name); got != name {
				t.Errorf("%s turned %q into %q", lang, name, got)
			}
		}
	}
}

// Everything that is not a word stays exactly as it was — the ids a board's own
// properties point at, and the millisecond timestamps a decoder is happy to
// hand back as 1.7e+12.
func TestTranslatingATemplateKeepsEverythingThatIsNotAWord(t *testing.T) {
	data, err := readShippedTemplate("templates/developer-tasks.jsonl")
	if err != nil {
		t.Fatalf("read the template: %v", err)
	}
	out, err := translateTemplate(data, templateCatalogue("ru"))
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	board := firstBoard(t, out)
	fields, _ := board["fields"].(map[string]any)
	props, _ := fields["properties"].(map[string]any)
	if props["outcomeProperty"] != "aoutcomefield000000000000001" {
		t.Errorf("the board points at %v as its outcome property", props["outcomeProperty"])
	}
	if props["template"] != "developer-tasks" {
		t.Errorf("the slug became %v", props["template"])
	}
	if strings.ContainsAny(string(out), "e+") && strings.Contains(string(out), "e+1") {
		t.Error("a timestamp came back in exponent form")
	}
	if !strings.Contains(string(out), `"aworkflowfeatureopt000000001"`) {
		t.Error("an option id was rewritten")
	}
}

// The words a person reads are translated wherever they stand, and a board is
// only usable if they agree: a route is picked by a card's option, and the
// option's value has to be the route's name.
func TestATranslatedBoardSaysTheSameNameInEveryPlace(t *testing.T) {
	data, err := readShippedTemplate("templates/developer-tasks.jsonl")
	if err != nil {
		t.Fatalf("read the template: %v", err)
	}
	out, err := translateTemplate(data, templateCatalogue("ru"))
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	board := firstBoard(t, out)
	if board["title"] != "Разработка" {
		t.Errorf("board title %v", board["title"])
	}
	fields, _ := board["fields"].(map[string]any)
	if !optionExists(fields, "Сценарий", "Фича") {
		t.Error("the route property has no «Фича» option")
	}
	if !optionExists(fields, "Исход", "прошло") {
		t.Error("the outcome field was translated")
	}
	props, _ := fields["properties"].(map[string]any)
	flows, _ := props["flows"].([]any)
	first, _ := flows[0].(map[string]any)
	if first["name"] != "Фича" {
		t.Errorf("the first route is called %v, and the card picks a route by that name", first["name"])
	}
	if first["property"] != "Статус" {
		t.Errorf("the route runs on %v", first["property"])
	}
	nodes, _ := first["nodes"].([]any)
	node, _ := nodes[0].(map[string]any)
	if node["column"] != "В работе" {
		t.Errorf("its first stage stands on %v", node["column"])
	}
}

// A language nobody has translated the templates into is not an error: the
// boards arrive as they are written, which is the whole of what an install had
// before any of this existed.
func TestAnUntranslatedLanguageLeavesTheTemplatesAlone(t *testing.T) {
	for _, lang := range []string{"", "en", "EN", "en_AU", "kk", "zh_Hans"} {
		if cat := templateCatalogue(lang); cat != nil {
			t.Errorf("%q found a catalogue", lang)
		}
	}
	if cat := templateCatalogue("ru_RU"); cat == nil {
		t.Error("ru_RU is Russian, and Russian is translated")
	}
}

// everyTemplateString reads the templates off the disk rather than out of the
// build: a catalogue is one file for every edition, and a base build embeds the
// paid edition's boards nowhere. Off the disk both are read whichever build the
// test runs in.
func everyTemplateString(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the templates: %v", err)
	}
	strs, err := templateStrings(files, os.ReadFile)
	if err != nil {
		t.Fatalf("read the templates: %v", err)
	}
	return strs
}

func catalogues(t *testing.T) map[string]map[string]string {
	t.Helper()
	files, err := fs.Glob(templateCatalogues, "templates/i18n/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no catalogues: %v", err)
	}
	out := map[string]map[string]string{}
	for _, file := range files {
		data, err := templateCatalogues.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		var cat map[string]string
		if err := json.Unmarshal(data, &cat); err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		out[strings.TrimSuffix(path.Base(file), ".json")] = cat
	}
	return out
}

func firstBoard(t *testing.T, jsonl []byte) map[string]any {
	t.Helper()
	for _, line := range strings.Split(string(jsonl), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("read a line back: %v", err)
		}
		data, _ := doc["data"].(map[string]any)
		if data["type"] == "board" {
			return data
		}
	}
	t.Fatal("no board in the template")
	return nil
}

func optionExists(fields map[string]any, property, value string) bool {
	list, _ := fields["cardProperties"].([]any)
	for _, item := range list {
		prop, _ := item.(map[string]any)
		if prop["name"] != property {
			continue
		}
		options, _ := prop["options"].([]any)
		for _, o := range options {
			opt, _ := o.(map[string]any)
			if opt["value"] == value {
				return true
			}
		}
	}
	return false
}

func short(s string) string {
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
