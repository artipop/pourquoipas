package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/artipop/xciii/server/mlog"
)

func Test_NewServer(t *testing.T) {
	tests := []struct {
		name               string
		rootPath           string
		serverRoot         string
		ssl                bool
		port               int
		localOnly          bool
		logger             mlog.LoggerIFace
		expectedBaseURL    string
		expectedServerAddr string
	}{
		{
			name:               "should return server with given properties",
			rootPath:           "./test/path/to/root",
			serverRoot:         "https://some-fake-server.com/fake-url",
			ssl:                false,
			port:               9999, // fake port number
			localOnly:          false,
			logger:             &mlog.Logger{},
			expectedBaseURL:    "/fake-url",
			expectedServerAddr: ":9999",
		},
		{
			name:               "should return local server with given properties",
			rootPath:           "./test/path/to/root",
			serverRoot:         "https://some-fake-server.com/fake-url",
			ssl:                false,
			port:               3000, // fake port number
			localOnly:          true,
			logger:             &mlog.Logger{},
			expectedBaseURL:    "/fake-url",
			expectedServerAddr: "localhost:3000",
		},
		{
			name:               "should match Server properties when ssl true",
			rootPath:           "./test/path/to/root",
			serverRoot:         "https://some-fake-server.com/fake-url",
			ssl:                true,
			port:               8000, // fake port number
			localOnly:          false,
			logger:             &mlog.Logger{},
			expectedBaseURL:    "/fake-url",
			expectedServerAddr: ":8000",
		},
		{
			name:               "should return local server when ssl true",
			rootPath:           "./test/path/to/root",
			serverRoot:         "https://localhost:8080/fake-url",
			ssl:                true,
			port:               9999, // fake port number
			localOnly:          true,
			logger:             &mlog.Logger{},
			expectedBaseURL:    "/fake-url",
			expectedServerAddr: "localhost:9999",
		},
		{
			name:               "should return '/' as base url is not good!",
			rootPath:           "",
			serverRoot:         "https://localhost:8080/#!@$@#@",
			ssl:                true,
			port:               9999, // fake port number
			localOnly:          true,
			logger:             &mlog.Logger{},
			expectedBaseURL:    "/",
			expectedServerAddr: "localhost:9999",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := NewServer(test.rootPath, test.serverRoot, test.port, test.ssl, test.localOnly, test.logger)

			require.NotNil(t, ws, "The webserver object is nil!")

			require.Equal(t, test.expectedBaseURL, ws.baseURL, "baseURL does not match")
			require.Equal(t, test.rootPath, ws.rootPath, "rootPath does not match")
			require.Equal(t, test.port, ws.port, "rootPath does not match")
			require.Equal(t, test.ssl, ws.ssl, "logger pointer does not match")
			require.Equal(t, test.logger, ws.logger, "logger pointer does not match")

			if test.localOnly == true {
				require.Equal(t, test.expectedServerAddr, ws.Server.Addr, "localhost address not as matching!")
			} else {
				require.Equal(t, test.expectedServerAddr, ws.Server.Addr, "server address not matching!")
			}
		})
	}
}

// The page is served the way a Wails asset server serves one: a file of the
// bundle by its own path — Vite's hashed output under /assets, what public/
// put at the root — and every other path as the page, because a route the page
// owns has to survive a reload. index.html is a template (BaseURL) and must
// never go out as it lies on disk, and nothing outside the bundle is reachable.
func TestTheBundleIsServedFromItsRootAndEveryOtherPathIsThePage(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}
	write("index.html", `<html>{{.BaseURL}}page</html>`)
	write("assets/index-abc.js", "console.log(1)")
	write("favicon.svg", "<svg/>")
	secret := filepath.Join(filepath.Dir(root), "outside.txt")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	t.Cleanup(func() { _ = os.Remove(secret) })

	ws := NewServer(root, "http://localhost/base", -1, false, true, &mlog.Logger{})
	ws.registerRoutes()
	get := func(target string) (int, string) {
		rec := httptest.NewRecorder()
		ws.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code, rec.Body.String()
	}

	code, body := get("/assets/index-abc.js")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "console.log(1)", body)

	_, body = get("/favicon.svg?v=1")
	require.Equal(t, "<svg/>", body)

	for _, page := range []string{"/", "/index.html", "/team/board/view", "/assets", "/assets/missing.js"} {
		code, body = get(page)
		require.Equal(t, http.StatusOK, code, page)
		require.Equal(t, "<html>/basepage</html>", body, page)
	}

	for _, escape := range []string{"/../outside.txt", "/assets/../../outside.txt", "/%2e%2e/outside.txt"} {
		_, body = get(escape)
		require.NotContains(t, body, "secret", escape)
	}
}
