package apperr

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// said matches a code and its English where the Go source says one.
var said = regexp.MustCompile(`apperr\.(?:New|Say|Wrap|Kind)\(\s*(?:[\w.]+,\s*)?"([a-z0-9.-]+)",\s*("(?:[^"\\]|\\.)*")`)

// A code the page has no sentence for reaches a Russian screen in English, and
// a sentence whose English drifted from Go's is two texts for one thing. The
// page's catalogue is generated from goMessages.ts, so this holds both.
func TestEveryCodeHasASentence(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "frontend", "i18n", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalogue map[string]string
	if err := json.Unmarshal(data, &catalogue); err != nil {
		t.Fatal(err)
	}

	used := map[string]string{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range said.FindAllStringSubmatch(string(src), -1) {
			english, err := strconv.Unquote(m[2])
			if err != nil {
				t.Errorf("%s: %s: %v", path, m[1], err)
				continue
			}
			if was, ok := used[m[1]]; ok && was != english {
				t.Errorf("%s says two things: %q and %q", m[1], was, english)
			}
			used[m[1]] = english
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(used) == 0 {
		t.Fatal("no codes found; the pattern no longer matches the source")
	}
	for code, english := range used {
		sentence, ok := catalogue["Go."+code]
		switch {
		case !ok:
			t.Errorf("Go.%s is not in en.json: add it to frontend/src/goMessages.ts", code)
		case sentence != english:
			t.Errorf("Go.%s: the page says %q, Go says %q", code, sentence, english)
		}
	}
	for id := range catalogue {
		if code, ok := strings.CutPrefix(id, "Go."); ok {
			if _, used := used[code]; !used {
				t.Errorf("%s is in the catalogue and Go no longer says it", id)
			}
		}
	}
}
