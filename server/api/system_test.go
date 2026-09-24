package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"testing"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/mlog"
	"github.com/artipop/xciii/server/model"
)

func TestHello(t *testing.T) {
	testAPI := API{logger: mlog.CreateConsoleTestLogger(t)}

	t.Run("Returns 'Hello' on success", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/hello", nil)
		response := httptest.NewRecorder()

		testAPI.handleHello(response, request)

		got := response.Body.String()
		want := "Hello"

		if got != want {
			t.Errorf("got %q want %q", got, want)
		}

		if response.Code != http.StatusOK {
			t.Errorf("got HTTP %d want %d", response.Code, http.StatusOK)
		}
	})
}

func TestPing(t *testing.T) {
	testAPI := API{logger: mlog.CreateConsoleTestLogger(t)}

	t.Run("Returns metadata on success", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		response := httptest.NewRecorder()

		testAPI.handlePing(response, request)

		var got app.ServerMetadata
		err := json.NewDecoder(response.Body).Decode(&got)
		if err != nil {
			t.Fatalf("Unable to JSON decode response body %q", response.Body)
		}

		want := app.ServerMetadata{
			Version:      model.CurrentVersion,
			Capabilities: model.Capabilities,
			DBType:       "",
			DBVersion:    "",
			OSType:       runtime.GOOS,
			OSArch:       runtime.GOARCH,
		}

		// What a desktop reads before it joins: every capability, by name.
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %q want %q", got, want)
		}

		if response.Code != http.StatusOK {
			t.Errorf("got HTTP %d want %d", response.Code, http.StatusOK)
		}
	})

}
