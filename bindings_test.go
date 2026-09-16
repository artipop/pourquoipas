package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"testing"
)

// generatedApp is the binding module `wails3 task generate:bindings` writes for
// App.
const generatedApp = "frontend/bindings/github.com/artipop/xciii/app.ts"

// The page calls App through bindings generated from it, and they are committed
// so that building or testing the page needs no Go. What that costs is that they
// can fall behind: a method added here and not generated is a method the page
// cannot call, and one removed here is a call that fails at runtime. Both are
// caught here rather than by somebody clicking.
//
// This compares the names only. A changed signature is what CI's regeneration
// catches (.github/workflows/test.yml), since only the generator knows how a Go
// type is spelled in TypeScript.
func TestTheGeneratedBindingsHaveEveryMethodOfApp(t *testing.T) {
	src, err := os.ReadFile(generatedApp)
	if err != nil {
		t.Fatal(err)
	}
	generated := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^export function (\w+)\(`).FindAllStringSubmatch(string(src), -1) {
		generated[m[1]] = true
	}

	bound := map[string]bool{}
	typ := reflect.TypeOf(&App{})
	for i := 0; i < typ.NumMethod(); i++ {
		bound[typ.Method(i).Name] = true
	}

	for name := range bound {
		if !generated[name] {
			t.Errorf("App.%s is not in %s — run `wails3 task generate:bindings`", name, generatedApp)
		}
	}
	for name := range generated {
		if !bound[name] {
			t.Errorf("%s still has %s, which App no longer does — run `wails3 task generate:bindings`", generatedApp, name)
		}
	}
}

// The bindings call through @wailsio/runtime, bundled into the page, and the Go
// side answers with the runtime of the Wails module in go.mod. They are two
// halves of one protocol shipped in two packages, so they move together.
func TestThePagesWailsRuntimeIsTheGoModulesVersion(t *testing.T) {
	gomod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s*github\.com/wailsapp/wails/v3 v(\S+)`).FindSubmatch(gomod)
	if m == nil {
		t.Fatal("go.mod requires no github.com/wailsapp/wails/v3")
	}
	goVersion := string(m[1])

	raw, err := os.ReadFile("frontend/package.json")
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatal(err)
	}
	if got := pkg.Dependencies["@wailsio/runtime"]; got != goVersion {
		t.Errorf("frontend/package.json has @wailsio/runtime %q, go.mod has Wails %q — pin the npm package to exactly the Go module's version", got, goVersion)
	}
}
