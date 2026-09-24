package boardadapter

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

// The templates are written once, in English, and translated on the way in.
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

// translateTemplate rewrites a template's JSONL through the catalogue, line by
// line as the importer reads it. Untouched — every id, every colour, every
// timestamp — except the text fields above.
func translateTemplate(data []byte, cat map[string]string) ([]byte, error) {
	if cat == nil {
		return data, nil
	}
	var out bytes.Buffer
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		// UseNumber, or a card's createAt — a millisecond timestamp — comes
		// back out as 1.7e+12 and the board arrives with a broken date.
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		var doc any
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("read a template line: %w", err)
		}
		encoded, err := json.Marshal(translateValue(doc, cat, ""))
		if err != nil {
			return nil, fmt.Errorf("write a template line: %w", err)
		}
		out.Write(encoded)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
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
	return templateStrings(files, readShippedTemplate)
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

// TemplateWords is the catalogue as a function, for the words this app says
// outside the template files themselves — the inbox board it makes at launch.
// A language nobody has translated the templates into gets the identity, which
// is the source language and is what those names already are.
func TemplateWords(lang string) func(string) string {
	cat := templateCatalogue(lang)
	return func(s string) string { return translateText(cat, s) }
}
