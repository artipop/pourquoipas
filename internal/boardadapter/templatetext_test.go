package boardadapter

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"
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

// Everything that is not a word stays exactly as it was — the ids a board's own
// properties point at, the slug, and the millisecond timestamps a decoder is
// happy to hand back as 1.7e+12.
func TestTranslatingACopyKeepsEverythingThatIsNotAWord(t *testing.T) {
	a, dev := installedDeveloperTemplate(t)
	bab := copyTemplate(t, a, dev.ID, "ru")
	board := bab.Boards[0]
	if board.Properties["branchProperty"] != "abranchfield0000000000000001" {
		t.Errorf("the board points at %v as its branch property", board.Properties["branchProperty"])
	}
	if board.Properties["template"] != "developer-tasks" {
		t.Errorf("the slug became %v", board.Properties["template"])
	}
	if !hasProperty(board, "arepositoriesfield0000000001") {
		t.Error("a property id was rewritten")
	}
	for _, block := range bab.Blocks {
		if block.CreateAt < 1e12 {
			t.Errorf("block %s was created at %d", block.ID, block.CreateAt)
		}
	}
}

// The words a person reads are translated wherever they stand: the board's
// title, its columns, and the cards that arrive standing in one of them.
func TestATranslatedCopySaysTheSameNameInEveryPlace(t *testing.T) {
	a, dev := installedDeveloperTemplate(t)
	board := copyTemplate(t, a, dev.ID, "ru").Boards[0]
	if board.Title != "Разработка" {
		t.Errorf("board title %v", board.Title)
	}
	if !boardOptionExists(board, "Статус", "В работе") {
		t.Error("the column property has no «В работе» option")
	}
}

// The page translates on display with the same fields the copy is translated
// by, so it is handed them rather than keeping a list of its own.
func TestThePageIsHandedTheFieldsAndTheWords(t *testing.T) {
	cat := TemplateCatalogueFor("ru_RU")
	if len(cat.Fields) != len(textFields) {
		t.Errorf("fields %v", cat.Fields)
	}
	if cat.Words["Development"] != "Разработка" {
		t.Errorf("Development is %q", cat.Words["Development"])
	}
	if en := TemplateCatalogueFor("en"); en.Words == nil || len(en.Words) != 0 {
		t.Errorf("English has words %v", en.Words)
	}
}

func installedDeveloperTemplate(t *testing.T) (*app.App, *model.Board) {
	t.Helper()
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}
	a.SetTemplateText(TranslateTemplateCopy)
	return a, installedTemplates(t, a)["developer-tasks"]
}

func hasProperty(board *model.Board, id string) bool {
	for _, prop := range board.CardProperties {
		if prop["id"] == id {
			return true
		}
	}
	return false
}

func boardOptionExists(board *model.Board, property, value string) bool {
	for _, prop := range board.CardProperties {
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
// build, so a template added to the directory is checked before it is embedded.
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
	return withInboxWords(strs)
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

func short(s string) string {
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
