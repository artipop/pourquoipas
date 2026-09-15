package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
)

// bootstrapScript is injected into the served index.html <head>, before any app
// JS runs. It mirrors the init scripts of linux/main.go:
//   - seeds the single-user session token into localStorage,
//   - builds the bridge the webapp expects — window.go.main.App and
//     window.runtime — on top of the v3 runtime, and
//   - wires window.openInNewBrowser (a hook the webapp already calls for
//     external links, CSV export, etc.) to the bound App.OpenInBrowser method,
//     plus a catch-all that sends every outward link there.
//
// The bridge is what replaces v2's generated bindings. v3 injects nothing into
// the page: it serves /wails/runtime.js and the page loads it. So the shim
// imports that module once and turns every property read on
// window.go.main.App into Call.ByName('main.App.<name>', …) — the fully
// qualified name of a method on the bound App service — and window.runtime
// .EventsOn into Events.On, unwrapping the event object the v3 runtime passes
// (v2 handed the payload itself). The webapp therefore sees the same surface it
// saw under v2, feature detection included, and stays inert in browser and
// plugin builds where nothing injects this script at all.
//
// The catch-all runs in the capture phase and covers *every* absolute http(s)
// anchor, not only target=_blank ones without an inline handler: deferring to
// an inline handler leaves the link dead wherever that handler does not run,
// and the webview cannot navigate to an outside origin either, so the click
// does nothing at all. Capturing first makes this the one path out.
//
// There is deliberately no window.webSocketBaseURL here, which under v2 told
// the webapp's socket client the board server's own address. The page, the API
// and /ws now share one origin — the front door (frontdoor.go) — so the socket
// connects to where the page came from, as it does in a browser.
func bootstrapScript(sessionToken string) string {
	return fmt.Sprintf(`<script>
(function () {
  // In single-user mode this is the session the API synthesizes, seeded before
  // any app script runs. On a hosted server it is empty and the page logs in.
  try {
    var seeded = %q;
    if (seeded) { localStorage.setItem('boardSessionId', seeded); }
    // The same token a second way, for the front door (frontdoor.go): the Wails
    // runtime makes its own fetches and a WebSocket carries no header a page
    // can set, so what guards those routes is a cookie. Written here and by
    // octoClient at login, because a login navigates without reloading.
    var token = localStorage.getItem('boardSessionId') || '';
    if (token) {
      document.cookie = 'boardSession=' + token + '; path=/; SameSite=Strict' +
        (location.protocol === 'https:' ? '; Secure' : '');
    }
  } catch (e) {}

  // One import of the v3 runtime, kept for the life of the page — but a failed
  // one is not kept. A dynamic import that rejects would otherwise be the
  // page's answer for ever: the module map caches the failure, and logging in
  // navigates without reloading, so every bound call after it would fail with
  // an error about the *module* rather than about the session.
  var runtimePromise = null;
  function wailsRuntime() {
    if (!runtimePromise) {
      runtimePromise = import('/wails/runtime.js').catch(function (err) {
        runtimePromise = null;
        throw err;
      });
    }
    return runtimePromise;
  }

  // Anything read off the App object is a bound method; 'then' is excluded so
  // the object is never mistaken for a promise by an accidental await.
  var app = new Proxy({}, {
    get: function (_target, name) {
      if (typeof name !== 'string' || name === 'then') { return undefined; }
      return function () {
        var args = Array.prototype.slice.call(arguments);
        return wailsRuntime().then(function (rt) {
          return rt.Call.ByName('main.App.' + name, ...args);
        });
      };
    },
    has: function (_target, name) { return typeof name === 'string' && name !== 'then'; },
  });
  window.go = {main: {App: app}};
  window.runtime = {
    EventsOn: function (event, callback) {
      var off = null;
      var cancelled = false;
      wailsRuntime().then(function (rt) {
        if (cancelled) { return; }
        off = rt.Events.On(event, function (e) { callback(e && e.data); });
      });
      return function () {
        cancelled = true;
        if (off) { off(); off = null; }
      };
    },
  };

  window.openInNewBrowser = function (href) {
    if (href && window.go && window.go.main && window.go.main.App) {
      window.go.main.App.OpenInBrowser(href);
    }
  };
  document.addEventListener('click', function (e) {
    if (e.button !== 0 || e.defaultPrevented) { return; }
    var a = e.target && e.target.closest && e.target.closest('a[href]');
    if (!a) { return; }
    var href = a.href || a.getAttribute('href') || '';
    if (!/^https?:\/\//i.test(href)) { return; }
    // In-app navigation is same-origin and stays in the webview; anything
    // pointing outside is for the system browser.
    try {
      if (new URL(href, window.location.href).origin === window.location.origin) { return; }
    } catch (err) {}
    e.preventDefault();
    e.stopPropagation();
    window.openInNewBrowser(href);
  }, true);
})();
</script>`, sessionToken)
}

// newServerProxy builds what the front door puts behind everything that is not
// /wails/ or /acp/: the page, and the board the page talks to. Unattached those
// are the same thing.
//
// Attached they come apart, and the split is the point: **the page is not
// fetched from the server.** webapp/pack is embedded in this binary, so
// proxying somebody's HTML would ask the network for a file already on this
// disk — and it would be *their* page, while the bootstrap injected into it
// assumes one this desktop's Go side matches.
//
// What lives on the server is the data, and the page asks for it under two
// prefixes: /api/v0 (with /files inside it) and /ws. Everything else stays
// local. The page never knows which is which — every path it builds is relative
// to window.location.origin, and that is the front door either way.
//
// token is asked per response rather than baked in: the session for a hosted
// board is not known until somebody logs in. Which board is behind it can
// change while the app runs, which is what boardRouter below is for.
func newServerProxy(port int, attached string, token func() string) (http.Handler, error) {
	local, err := url.Parse(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		return nil, err
	}
	page := httputil.NewSingleHostReverseProxy(local)
	pageDirector := page.Director
	page.Director = func(req *http.Request) {
		pageDirector(req)
		req.Host = local.Host
		// Ask upstream for uncompressed HTML so we can inject the bootstrap script.
		req.Header.Del("Accept-Encoding")
	}
	page.ModifyResponse = injectBootstrap(token)

	if attached == "" {
		return page, nil
	}

	remote, err := url.Parse(attached)
	if err != nil {
		return nil, fmt.Errorf("attached server %q: %w", attached, err)
	}
	data := httputil.NewSingleHostReverseProxy(remote)
	dataDirector := data.Director
	data.Director = func(req *http.Request) {
		dataDirector(req)
		// Load-bearing, unlike the local one above: the far side's own
		// hostGuard checks the Host it was published under and would refuse a
		// loopback name. Do not "simplify" this away.
		req.Host = remote.Host
		// Our loopback origin is not who is calling upstream, and saying so is
		// a lie a guard added there later would believe. Sec-Fetch-* is the
		// browser's own account of the fetch and means nothing after a hop.
		req.Header.Set("Origin", remote.Scheme+"://"+remote.Host)
		req.Header.Del("Sec-Fetch-Site")
		req.Header.Del("Sec-Fetch-Mode")
		req.Header.Del("Sec-Fetch-Dest")
		req.Header.Del("Sec-Fetch-User")
		// The front door's own cookie is the front door's business. The board
		// authenticates by the Authorization header the page sends.
		stripCookie(req, sessionCookie)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/v0/", data)
	mux.Handle("/ws", data)
	mux.Handle("/", page)
	return mux, nil
}

// injectBootstrap puts the shim into the served HTML, right after <head>, so
// the token is set before app scripts load.
func injectBootstrap(token func() string) func(*http.Response) error {
	return func(resp *http.Response) error {
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
			return nil
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if idx := bytes.Index(body, []byte("<head>")); idx != -1 {
			at := idx + len("<head>")
			inject := []byte(bootstrapScript(token()))
			body = append(body[:at], append(inject, body[at:]...)...)
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
		return nil
	}
}

// stripCookie removes one cookie from a request, leaving the others.
func stripCookie(req *http.Request, name string) {
	cookies := req.Cookies()
	req.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name == name {
			continue
		}
		req.AddCookie(cookie)
	}
}

// boardRouter is the handler the front door holds, so which board the page
// talks to can change without rebuilding the door. A switch is a rebuild and a
// reload: everything the page holds came from the board it loaded against.
type boardRouter struct {
	port  int
	token func() string

	current atomic.Pointer[http.Handler]
}

func newBoardRouter(port int, attached string, token func() string) (*boardRouter, error) {
	r := &boardRouter{port: port, token: token}
	if err := r.point(attached); err != nil {
		return nil, err
	}
	return r, nil
}

// point aims the router at a board: a server's origin, or empty for the one
// this process serves.
func (r *boardRouter) point(attached string) error {
	next, err := newServerProxy(r.port, attached, r.token)
	if err != nil {
		return err
	}
	r.current.Store(&next)
	return nil
}

func (r *boardRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := r.current.Load()
	if handler == nil {
		http.Error(w, "no board", http.StatusServiceUnavailable)
		return
	}
	(*handler).ServeHTTP(w, req)
}
