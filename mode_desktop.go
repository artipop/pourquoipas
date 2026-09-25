//go:build !server

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/artipop/xciii/internal/feedmcp"
)

// origin is the front door of a desktop build. A Wails desktop app has no HTTP
// server — the webview asks a custom scheme handler for its pages — so we bring
// one: a transport implementing AssetServerTransport is handed Wails' own asset
// handler and is then free to serve it wherever it likes.
//
// The window is pointed at that address instead of the wails:// origin, which
// is the whole point: a page served over real HTTP can open a real WebSocket,
// so /ws reaches the board directly and window.webSocketBaseURL is not needed.
// The listener is loopback-only and its port is random per launch.
type origin struct {
	*application.HTTPTransport

	listener net.Listener
	board    http.Handler
	acp      http.Handler
	ingest   http.Handler
	tailnet  *tailnetController
	server   *http.Server
	// session says whether a request carries a live board session; nil in
	// single-user mode, where nothing does. See identity.go.
	session func(string) bool
}

// newOrigin binds the front door's listener up front, so its address is known
// before the application — and therefore the window — is created. Serving
// starts later, when Wails hands over the asset handler.
func newOrigin(board, acp, ingest http.Handler, tailnet *tailnetController, session func(string) bool) (*origin, error) {
	listener, err := listenLoopback(0)
	if err != nil {
		return nil, fmt.Errorf("front door: %w", err)
	}
	return &origin{
		HTTPTransport: application.NewHTTPTransport(),
		listener:      listener,
		board:         board,
		acp:           acp,
		ingest:        ingest,
		tailnet:       tailnet,
		session:       session,
	}, nil
}

// ServeAssets is called by Wails once the asset server exists. Everything but
// /wails/ goes to the board.
func (o *origin) ServeAssets(assetHandler http.Handler) error {
	assetHandler = quietOptionalCustomJS(assetHandler)
	o.server = &http.Server{Handler: newFrontDoor(assetHandler, o.acp, o.ingest, o.board, o.host(), o.session)}
	go func() {
		if err := o.server.Serve(o.listener); err != nil && err != http.ErrServerClosed {
			log.Printf("front door stopped: %v", err)
		}
	}()
	// The tailnet gets a front door of its own rather than this one: the guards
	// are keyed to the authority the page is served under, and there it is a
	// tailnet name, not this loopback address.
	o.tailnet.publish(func(allowedHost string) http.Handler {
		return newFrontDoor(assetHandler, o.acp, o.ingest, o.board, allowedHost, o.session)
	})
	// The port is new every launch, so the column feed's bridge learns it
	// from a file (internal/feedmcp). A server build has an address that does
	// not move and is given it instead.
	if dir, err := appDataDir("", 0o755); err == nil {
		if err := feedmcp.WriteHandoff(dir, "http://"+o.host()); err != nil {
			log.Printf("feed: the address was not written: %v", err)
		}
	}
	return nil
}

// customJSPath is the optional script the Wails runtime probes for on every
// page load.
const customJSPath = "/wails/custom.js"

// quietOptionalCustomJS answers that probe with "nothing here" instead of an
// error. Wails serves custom.js in a server build (it carries the runtime's
// WebSocket connection) and deliberately 404s it in a desktop one, because the
// runtime reads the refusal as "there is no custom script". The browser reads
// it as a failed resource and writes it into the console on every launch, where
// it looks like a broken app.
//
// 204 says the same thing to the runtime — it checks response.ok and then the
// content type, and takes anything that is not JavaScript as nothing to load —
// without the console entry, and without the second request an empty script
// would cost. Desktop only, since a server build has a real one to serve.
func quietOptionalCustomJS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == customJSPath {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// frontDoorDrain bounds the wait for requests in flight, because Shutdown runs
// on the main thread inside the framework's cleanup: one request that never
// returns hangs the quit for ever, process alive and app gone from the screen.
// Quitting *through* the front door is exactly that case — Shutdown waits for
// the very request asking it to quit.
const frontDoorDrain = 2 * time.Second

func (o *origin) Stop() error {
	if dir, err := appDataDir("", 0o755); err == nil {
		feedmcp.RemoveHandoff(dir)
	}
	if o.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), frontDoorDrain)
		defer cancel()
		_ = o.server.Shutdown(ctx)
	}
	return o.HTTPTransport.Stop()
}

func (o *origin) host() string { return o.listener.Addr().String() }

// url is where the window is pointed.
func (o *origin) url() string { return "http://" + o.host() + "/" }

// transport hands the front door to Wails as the IPC transport, which is how
// it gets to serve the assets at all.
func (o *origin) transport() application.Transport { return o }

// serverOptions are unused in a desktop build: nothing here binds a public port.
func (o *origin) serverOptions() application.ServerOptions { return application.ServerOptions{} }

// start is a no-op: the front door began serving from ServeAssets.
func (o *origin) start() {}

// mainWindowName is what the board's own window is called. It has a name so
// that anything bringing a person back to the board — the menu bar icon, a
// notification they clicked — finds the window they left rather than opening a
// second board beside it.
const mainWindowName = "main"

// The app outlives its windows.
//
// Closing the board is not quitting: the agents go on working, a running stage
// goes on running, and the icon in the menu bar is what says so and what leads
// back.
//
// Two mechanisms, because the platforms disagree about whose question this is.
// macOS asks whether to terminate once the last window closed; Windows and
// Linux call Options.ShouldQuit from a teardown that App.Quit() also goes
// through — so a constant "never" would make the quit item do nothing. Hence a flag.
var quitting atomic.Bool

// requestQuit is a person asking to leave, from the one place left to ask once
// there is no window to press ⌘Q in.
func requestQuit(wapp *application.App) {
	quitting.Store(true)
	wapp.Quit()
}

// appShouldQuit answers Wails' «may we go now?», and which question that is
// depends on the platform — getting it wrong costs ⌘Q.
//
// Windows and Linux ask from a teardown both the last window closing and
// App.Quit() go through, so the flag is the honest answer there. macOS asks from
// applicationShouldTerminate alone (the last window is settled by
// ApplicationShouldTerminateAfterLastWindowClosed), so the flag there refuses
// ⌘Q and the Dock's Quit as well.
func appShouldQuit() bool { return runtime.GOOS == "darwin" || quitting.Load() }

// watchDockReopen brings the board back when the app is activated with nothing
// on screen — the Dock icon, which is the other way in now that closing the
// window leaves the app running. macOS only; the other two have the menu bar
// icon and their taskbar.
func watchDockReopen(wapp *application.App, url string) {
	if runtime.GOOS != "darwin" {
		return
	}
	wapp.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) {
		go showMainWindow(wapp, url)
	})
}

// showMainWindow brings the board back: un-minimised, in front, and made if it
// is genuinely not there. A share that started the app opens only its own
// dialog, so "no board window" is an ordinary state rather than an error.
func showMainWindow(wapp *application.App, url string) {
	if win, ok := wapp.Window.GetByName(mainWindowName); ok {
		if win.IsMinimised() {
			win.UnMinimise()
		}
		win.Show()
		win.Focus()
		return
	}
	newMainWindow(wapp, url)
}

// newMainWindow opens the one window the desktop app has, on the front door's
// address rather than the wails:// origin.
func newMainWindow(wapp *application.App, url string) {
	win := wapp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:  mainWindowName,
		Title: "XCIII",
		// The stated size is what Restore returns to; the window itself opens
		// maximised (below).
		Width:  1024,
		Height: 768,
		URL:    url,
		// What shows between the window appearing and the page painting. Wails
		// cannot know which theme the page will choose, and of the two seams a
		// dark one is the quieter mistake.
		BackgroundColour: application.NewRGB(12, 12, 14),
	})
	// Maximised, not fullscreen: the board is the kind of thing a person gives
	// the whole screen to, and a 1024×768 default opened every session with
	// the columns squeezed and a resize as the first chore. Done when the
	// runtime reports the window ready rather than as StartState, which this
	// Wails runs before the window can zoom, so it opened at the stated size
	// anyway.
	var once sync.Once
	win.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		once.Do(func() { win.Maximise() })
	})

	// Closing puts the board away rather than destroying it, so coming back is
	// instant and lands on the board that was open rather than on a page loading
	// from scratch. A hook and not a listener: hooks run first and a cancelled
	// event never reaches the default listener, which is the one that destroys
	// the window. Off the main thread, so Hide's own dispatch is safe.
	//
	// Only this window. A terminal's window is one conversation, and closing it
	// means closing it.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if quitting.Load() {
			return
		}
		e.Cancel()
		win.Hide()
	})
}

// openDiffWindow gives a card's changes a window, named after the card so a
// second ask focuses the one already open. Deliberately not the terminal's
// black: a diff is read like a document, and the page paints it on the theme's
// own ground.
func openDiffWindow(wapp *application.App, cardID, title, url string) {
	name := "acp-diff-" + cardID
	if existing, ok := wapp.Window.GetByName(name); ok {
		existing.Show()
		existing.Focus()
		return
	}
	wapp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:   name,
		Title:  fill(currentWords().DiffWindow, "{title}", title),
		Width:  1040,
		Height: 760,
		URL:    url,
	})
}

// openShareWindow opens the share dialog: a small window, on top of whatever
// the person was reading when they pressed the share dialog, and named so that a
// second share focuses the one already open rather than stacking dialogs.
//
// It is deliberately not the main window: the app may not be running visibly at
// all, and showing somebody their whole board because they shared a link would
// be answering a question they did not ask.
func openShareWindow(wapp *application.App, url string) {
	if existing, ok := wapp.Window.GetByName(shareWindowName); ok {
		existing.SetURL(url)
		existing.Show()
		existing.Focus()
		return
	}
	wapp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             shareWindowName,
		Title:            currentWords().ShareWindow,
		Width:            420,
		Height:           520,
		DisableResize:    true,
		AlwaysOnTop:      true,
		BackgroundColour: application.NewRGB(12, 12, 14),
		URL:              url,
	})
}

// closeShareWindow shuts the dialog once it has said what happened — and goes
// with it, when the dialog is all there was.
//
// Sharing a link may be the only reason the app was launched, and the window it
// opens is deliberately not the board (openShareWindow) — so with the app
// outliving its windows, nothing but the dialog open means nothing to stay for.
// A board window somebody closed is hidden rather than gone, so it still
// counts.
func closeShareWindow(wapp *application.App) {
	if existing, ok := wapp.Window.GetByName(shareWindowName); ok {
		existing.Close()
	}
	for _, win := range wapp.Window.GetAll() {
		if win.Name() != shareWindowName {
			return
		}
	}
	requestQuit(wapp)
}

// pickDirectory opens the native folder picker.
func pickDirectory(wapp *application.App, title string) (string, error) {
	dialog := wapp.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                title,
		CanChooseDirectories: true,
		CanChooseFiles:       false,
	})
	return dialog.PromptForSingleSelection()
}
