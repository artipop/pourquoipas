package boardadapter

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/artipop/pourquoipas/server/model"
)

// The templates are written once, in English, and stored that way: the page
// translates them on display, in the language of whoever is reading, and a
// board made from one is translated once, as it is copied — from then on it is
// the person's data and no language switch touches it. Installing them in a
// language instead made the language the install's: on a server two people
// reading in two languages reinstalled them for each other at every page load.
// The alternative was a copy of each board per language, and a copy is a second
// description of the same template: a column added to one of them is a column
// missing from the other, and nothing says so until somebody opens the board.
// Here the file is the shape and the catalogue is the words, and a string with
// no translation stays as the file wrote it rather than disappearing.
//
// English is the source for the same reason the page's own strings are: a
// message id there is an English default and `ru.json` translates it, so the
// two halves of the product are written the same way round. A catalogue is
// English → the other language, keyed by the very string the template file
// carries — so there is no third name to invent and keep in step.
// TestEveryTemplateStringIsTranslated is what fails when a template gains a
// string the catalogue has not got.

//go:embed templates/i18n/*.json
var templateCatalogues embed.FS

// SourceLanguage is what the template files themselves are written in. A UI in
// that language translates nothing.
const SourceLanguage = "en"

// textFields are the JSON keys that hold something a person reads: the block
// titles (a text block keeps its markdown in `title`), the board's description,
// the name of a card property and the value of one of its options, and the
// automation's own words — a stage's column, the property a route runs on, a
// setup step's hint, a prompt.
//
// Translating by key rather than by "any string that is in the catalogue" is
// what keeps ids out of it: `propertyId`, `optionId` and `template` hold
// generated ids and a slug, and none of them is read here.
var textFields = map[string]bool{
	"title":       true,
	"description": true,
	"name":        true,
	"value":       true,
	"column":      true,
	"property":    true,
	"hint":        true,
	"prompt":      true,
}

// templateCatalogue is the words for one language, or nil when the templates
// are already in it and when nobody has translated them into it. Nil is a
// working catalogue: everything looked up in it comes back unchanged.
func templateCatalogue(lang string) map[string]string {
	lang = templateLanguage(lang)
	if lang == SourceLanguage {
		return nil
	}
	data, err := templateCatalogues.ReadFile("templates/i18n/" + lang + ".json")
	if err != nil {
		return nil
	}
	var cat map[string]string
	if json.Unmarshal(data, &cat) != nil {
		return nil
	}
	return cat
}

// templateLanguage is the language a UI setting names, reduced to the part a
// catalogue is filed under: the page keeps «en_AU» and «zh_Hans», and a
// translation of the templates is per language rather than per region.
func templateLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		lang = lang[:i]
	}
	if lang == "" {
		return SourceLanguage
	}
	return lang
}

// translateText is one lookup, and the place the exceptions above are honoured.
func translateText(cat map[string]string, s string) string {
	if cat == nil {
		return s
	}
	if out, ok := cat[s]; ok && out != "" {
		return out
	}
	return s
}

// TranslateTemplateCopy puts a board copied from one of the install's own
// templates into a language, before it is stored. Each board and block goes
// through its JSON — the shape the template file has it in — so the fields
// translated are the ones the catalogue was written against.
func TranslateTemplateCopy(bab *model.BoardsAndBlocks, lang string) error {
	cat := templateCatalogue(lang)
	if cat == nil || bab == nil {
		return nil
	}
	for _, board := range bab.Boards {
		if err := translateThrough(board, cat); err != nil {
			return fmt.Errorf("translate the board: %w", err)
		}
	}
	for _, block := range bab.Blocks {
		if err := translateThrough(block, cat); err != nil {
			return fmt.Errorf("translate block %s: %w", block.ID, err)
		}
	}
	return nil
}

func translateThrough(v any, cat map[string]string) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// UseNumber, or a card's createAt — a millisecond timestamp — comes back
	// out as 1.7e+12 and the board arrives with a broken date.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return err
	}
	out, err := json.Marshal(translateValue(doc, cat, ""))
	if err != nil {
		return err
	}
	return json.Unmarshal(out, v)
}

// TemplateCatalogue is what the page translates the install's own templates
// and the inbox with on display: the words for one language, and which JSON
// keys hold words, so the page walks a board the way TranslateTemplateCopy
// does and the two cannot disagree.
type TemplateCatalogue struct {
	Fields []string          `json:"fields"`
	Words  map[string]string `json:"words"`
}

// TemplateCatalogueFor is that catalogue; a language nobody has translated the
// templates into has no words, and the page shows the English.
func TemplateCatalogueFor(lang string) TemplateCatalogue {
	fields := make([]string, 0, len(textFields))
	for k := range textFields {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	words := templateCatalogue(lang)
	if words == nil {
		words = map[string]string{}
	}
	return TemplateCatalogue{Fields: fields, Words: words}
}

// translateValue walks a decoded line. The key a string arrived under travels
// with it through the lists, so an option's `value` inside `options` is
// translated and a `properties` map keyed by property id is not.
func translateValue(v any, cat map[string]string, key string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, item := range t {
			t[k] = translateValue(item, cat, k)
		}
		return t
	case []any:
		for i, item := range t {
			t[i] = translateValue(item, cat, key)
		}
		return t
	case string:
		if textFields[key] {
			return translateText(cat, t)
		}
		return t
	default:
		return v
	}
}

// TemplateStrings is what a template says to a person, in the source language:
// every string the walk above would translate. It is what a translator is
// handed, and what the guard test reads a catalogue against.
func TemplateStrings() ([]string, error) {
	files, err := shippedTemplates()
	if err != nil {
		return nil, err
	}
	strs, err := templateStrings(files, readShippedTemplate)
	if err != nil {
		return nil, err
	}
	return withInboxWords(strs), nil
}

// withInboxWords adds the inbox's words, which no template file carries: the
// app makes that board in code and translates it through the same catalogue.
func withInboxWords(strs []string) []string {
	seen := make(map[string]bool, len(strs))
	for _, s := range strs {
		seen[s] = true
	}
	for _, s := range inboxWords {
		if !seen[s] {
			strs = append(strs, s)
		}
	}
	return strs
}

// templateStrings takes the files and the way to read one, because a test reads
// them off the disk rather than out of the build: a catalogue covers every
// edition's templates, and a base build embeds only its own.
func templateStrings(files []string, read func(string) ([]byte, error)) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		data, err := read(file)
		if err != nil {
			return nil, err
		}
		for _, line := range bytes.Split(data, []byte("\n")) {
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			dec := json.NewDecoder(bytes.NewReader(line))
			dec.UseNumber()
			var doc any
			if err := dec.Decode(&doc); err != nil {
				return nil, fmt.Errorf("read %s: %w", file, err)
			}
			collectStrings(doc, "", seen, &out)
		}
	}
	return out, nil
}

func collectStrings(v any, key string, seen map[string]bool, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, item := range t {
			collectStrings(item, k, seen, out)
		}
	case []any:
		for _, item := range t {
			collectStrings(item, key, seen, out)
		}
	case string:
		if !textFields[key] || strings.TrimSpace(t) == "" || seen[t] {
			return
		}
		seen[t] = true
		*out = append(*out, t)
	}
}

// saidAs is every way a source string has been said: itself and its
// translation in each catalogue there is. It is what tells a name the app gave
// from one a person chose.
func saidAs(source string) map[string]bool {
	out := map[string]bool{source: true}
	files, err := fs.Glob(templateCatalogues, "templates/i18n/*.json")
	if err != nil {
		return out
	}
	for _, file := range files {
		lang := strings.TrimSuffix(path.Base(file), ".json")
		if s := translateText(templateCatalogue(lang), source); s != "" {
			out[s] = true
		}
	}
	return out
}

// TemplateWords is the catalogue as a function, for the words this app says
// outside the template files themselves — the inbox board it makes at launch.
// A language nobody has translated the templates into gets the identity, which
// is the source language and is what those names already are.
func TemplateWords(lang string) func(string) string {
	cat := templateCatalogue(lang)
	return func(s string) string { return translateText(cat, s) }
}
