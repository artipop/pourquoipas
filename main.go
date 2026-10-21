package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"

	"github.com/artipop/pourquoipas/server/services/notify"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/boardadapter"
	"github.com/artipop/pourquoipas/internal/inboxsort"
	"github.com/artipop/pourquoipas/internal/secrets"
	"github.com/artipop/pourquoipas/internal/sources"
	"github.com/artipop/pourquoipas/internal/userpath"
)

// appDataDir returns one of this install's own directories, made if it is not
// there. The install is named by appDirName, which is what keeps a development
// build's boards, agents and tokens out of the real app's — see
// datadir_dev.go.
//
// It is under os.UserConfigDir() rather than beside the binary because a
// packaged, signed app directory is read-only:
// ~/Library/Application Support on macOS, %AppData% on Windows,
// ~/.config (or $XDG_CONFIG_HOME) on Linux.
func appDataDir(name string, perm os.FileMode) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, appDirName, name)
	if err := os.MkdirAll(dir, perm); err != nil {
		return "", err
	}
	return dir, nil
}

// openAgentStore hands the agent integration the board's database. Both
// subsystems keep their tables there rather than in files of their own — see
// CLAUDE.md, "One database" — and neither creates anything: the schema is the
// migration's. On a database their queries are not written for they are off
// (appTablesSupported).
func openAgentStore(brd board, dir string) (*acp.Store, error) {
	if err := appTablesSupported(brd.dbType); err != nil {
		return nil, err
	}
	return acp.NewStore(brd.db), nil
}

func openSourceStore(brd board, dir string) (*sources.Store, error) {
	if err := appTablesSupported(brd.dbType); err != nil {
		return nil, err
	}
	return sources.NewStore(brd.db), nil
}

// acpDataDir returns the ACP integration's own state directory.
func acpDataDir() (string, error) { return appDataDir("acp", 0o750) }

// sourcesDataDir returns where the sources subsystem keeps its registry and
// what it has already seen.
func sourcesDataDir() (string, error) { return appDataDir("sources", 0o750) }

// tailnetDataDir returns where the tailnet door keeps its settings and the
// node's own state. Tighter than the rest: it holds an auth key.
func tailnetDataDir() (string, error) { return appDataDir("tailnet", 0o700) }

// attachDataDir returns where the address of the hosted board this desktop is
// attached to is kept. Tight like the tailnet's: it names a server somebody
// works on, and the session for it is in the credential store beside it.
func attachDataDir() (string, error) { return appDataDir("server", 0o700) }

func main() {
	// An update in progress re-execs this binary as its own helper: wait for
	// the old process to go, rename the new bundle into place, launch it, exit.
	// That is the whole of what the helper does, and it must not do anything
	// else — application.New calls this too, but by then a board server has
	// opened SQLite, taken a port, restored a PATH from the login shell and
	// started plugin processes, all of it in the one process whose job is to
	// wait for us to die. In helper mode the call never returns.
	updater.HandleHelperMode()

	// `pourquoipas mcp dokku` runs this same binary as an MCP server for an agent
	// session; it must come first, before the board server or a window exists.
	maybeRunMCP(os.Args[1:])

	// Before anything can be spawned: a packaged app is started by launchd and
	// gets its PATH, which has none of what this app runs for the user — npx,
	// node, the agent CLIs, a source plugin. See internal/userpath.
	if changed, err := userpath.Restore(); err != nil {
		log.Printf("path: %v", err)
	} else if changed {
		log.Printf("path: taken from the login shell (%s)", os.Getenv("SHELL"))
	}

	// Said out loud, because the two installs look identical from the inside
	// and the first sign of being in the wrong one is a board that should have
	// something on it and does not.
	if appIsDev {
		if base, err := os.UserConfigDir(); err == nil {
			log.Printf("development build: its own data, %s — the installed app keeps its own", filepath.Join(base, appDirName))
		}
	}

	// Who this install asks about is a property of the build, not a setting: a
	// desktop is one person and gets a single-user token, a hosted server is a
	// team and gets none, so /login and /register are open there and nowhere
	// else (identity.go).
	sessionToken := localSessionToken()
	if sessionToken == "" {
		log.Printf("hosted server: everybody logs in; /login and /register are open")
	}

	// Whether this desktop works its own board or a hosted one is read once
	// here, because the page's proxy and the board seam are both built with
	// the answer (attach.go).
	var attach *attachController
	if dir, err := attachDataDir(); err != nil {
		log.Printf("attach: unavailable, no data dir: %v", err)
	} else if attach, err = newAttachController(filepath.Join(dir, "settings.json"), openVault()); err != nil {
		log.Printf("attach: unavailable, settings error: %v", err)
		attach = nil
	}

	// The board on a server, once there is one. Declared here because the page's
	// router is built before it and the manager after.
	var attached *boardSeam

	port, err := getFreePort()
	if err != nil {
		log.Fatalf("failed to find a free port: %v", err)
	}

	// ACP integration: config + board-event backend, wired before the server
	// so the notify backend registers during server construction.
	var (
		acpCfg     acp.Config
		acpEnabled bool
		events     *boardadapter.EventsBackend
		backends   []notify.Backend
	)
	if dir, err := acpDataDir(); err != nil {
		log.Printf("acp: disabled, no data dir: %v", err)
	} else if acpCfg, err = acp.LoadConfig(filepath.Join(dir, "config.json"), dir); err != nil {
		log.Printf("acp: disabled, config error: %v", err)
	} else if acpCfg.Enabled {
		acpEnabled = true
	}

	logger := newServerLogger()
	// Only for a board this process serves: attached, card moves arrive over
	// the server's own websocket, and the local board has nobody moving cards
	// on it.
	if acpEnabled && attach.target() == "" {
		events = boardadapter.NewEventsBackend(logger)
		backends = append(backends, events)
	}

	brd, err := runServerWithLogger(logger, port, sessionToken, backends)
	if err != nil {
		log.Fatalf("failed to start the server: %v", err)
	}
	srv := brd.srv

	// The templates this app offers are its own (internal/boardadapter/
	// templates); the server module's are the upstream's examples and carry no
	// automation. Failing to install them costs the selector its contents, not
	// the app, so it is logged rather than fatal.
	// Not against a server: the templates on a hosted install are its own, and
	// this would delete the ones this build does not ship — every attached
	// desktop fighting over them at every launch.
	// In the language the app is read in: the boards themselves are one set of
	// files and the words are a catalogue beside them
	// (internal/boardadapter/templatetext.go).
	installTemplates := func(lang string) error {
		return boardadapter.ImportTemplates(srv.App(), logger, lang)
	}
	if attach.target() == "" {
		if err := installTemplates(uiLanguage()); err != nil {
			log.Printf("templates: %v", err)
		}
	}
	// The inbox is a board this app makes and keeps, there from the first
	// launch: it is where everything a source brings lands, and where a person
	// writes down what they have not decided about yet — so it cannot wait for
	// the first source. Made here, where the UI language is known, so its field
	// names are the person's. Failing costs nothing yet: the first arrival makes
	// it again.
	if _, err := boardadapter.EnsureInboxBoard(srv.App(), uiLanguage()); err != nil {
		log.Printf("inbox: %v", err)
	}
	// And in the language in force now, not the one it was made in: a board a
	// source made when the startup call failed is in the source language, and
	// a system locale may have changed since the last launch.
	if err := boardadapter.RelabelInboxBoard(srv.App(), uiLanguage()); err != nil {
		log.Printf("inbox: %v", err)
	}

	// Which board the page is served is the attachment's answer: this
	// process's own, or a hosted server this desktop logged in to (attach.go).
	// The token follows it — the single-user one for our own board, the
	// session for somebody else's.
	if !attachableBuild && attach != nil && attach.state().URL != "" {
		log.Printf("attach: ignoring %s — a hosted server works its own board", attach.state().URL)
	}
	// Which board the page is served is the selection's answer, not the
	// attachment's: a desktop can be attached to a server and looking at its
	// own board. The token follows the same answer.
	pageToken := func() string {
		if attach.selected() != "" {
			return attach.token()
		}
		return sessionToken
	}
	if attach.selected() != "" {
		log.Printf("showing %s: the page is served that board", attach.selected())
	}
	// Under `wails3 dev` the page itself comes from Vite (devserver.go).
	devServer := viteDevServer()
	if devServer != "" {
		log.Printf("development build: the page comes from Vite at %s", devServer)
	}
	router, err := newBoardRouter(port, attach.selected(), devServer, pageToken)
	if err != nil {
		log.Fatalf("failed to create the server proxy: %v", err)
	}
	handler := http.Handler(router)

	// The UI event socket lives on the front door beside the board: what the
	// agents are doing, sent to every page rather than only to the windows this
	// application owns. It is created before the manager exists — the manager
	// needs the board server, which needs the port the front door is already
	// listening on — and is given it below.
	uiEvents := newEventRoutes()
	// The board tools an agent calls back through, on the same subtree.
	boardTools := newBoardToolRoutes()
	// The column feed other applications read, given its board below.
	feed := &feedRoutes{}
	acpSockets := newACPSockets(uiEvents, boardTools.Handler(), feed.Handler())
	// The ingest endpoint, on the same terms: it is a route on the front door,
	// and it is handed its manager once the board exists.
	ingest := newSourceRoutes()

	emitter := newWailsEmitter(uiEvents)
	app := NewApp(emitter)
	// The templates and the inbox follow the language they are read in, and a
	// person changes that while the app runs — so the switch says nothing about
	// a restart and the picker is right the next time it is opened
	// (uisettings.go).
	app.relanguage = func(lang string) error {
		var errs []error
		if attach.target() == "" {
			errs = append(errs, installTemplates(lang))
		}
		if err := boardadapter.RelabelInboxBoard(srv.App(), lang); err != nil {
			errs = append(errs, fmt.Errorf("inbox: %w", err))
		}
		return errors.Join(errs...)
	}

	// The board as the page at /m reads it. Set here rather than beside the
	// sources registry because a phone lists boards and cards whether or not
	// this machine has a source or an agent.
	// The board this process serves is always worked: it is where this
	// machine's own cards are, whether or not anybody is looking at them.
	seam := localSeam(events, srv.App())
	lists := selectedLists{local: seam.lists, on: attach.selected}
	if target := attach.target(); target != "" {
		remote, _ := remoteSeam(target, attach.token(), logger)
		attached = &remote
		lists.remote = remote.lists
	}
	app.board = lists
	feed.SetLists(lists)

	// The tailnet door publishes the same front door to the user's own tailnet,
	// so a phone can reach the board. Off unless its settings file says
	// otherwise; a bad settings file disables it rather than stopping the app,
	// which is the same bargain the ACP config gets.
	var tailnet *tailnetController
	if dir, err := tailnetDataDir(); err != nil {
		log.Printf("tailnet: disabled, no data dir: %v", err)
	} else if tailnet, err = newTailnetController(filepath.Join(dir, "settings.json"), filepath.Join(dir, "state"), app.OpenInBrowser); err != nil {
		log.Printf("tailnet: disabled, settings error: %v", err)
		tailnet = nil
	}
	app.tailnet = tailnet
	app.attach = attach
	app.boards = router

	// The front door is the origin the page is served under — a loopback
	// listener of ours in a desktop build, the published address in a server
	// build. Its listener is bound here so the window can be pointed at it.
	// The guard on the routes that are about this machine rather than about a
	// board. Nil on a desktop, which is what "nothing authenticates a user"
	// means (frontdoor.go).
	sessionGuard := newSessionGuard(srv.App())
	front, err := newOrigin(handler, acpSockets, ingest, tailnet, sessionGuard)
	if err != nil {
		log.Fatalf("failed to open the front door: %v", err)
	}

	// Where a credential the app has to *present* is kept. The environment comes
	// first, so a token given from outside wins without anybody deleting
	// anything; then the platform keychain; then a 0600 file where there is no
	// keychain. The keychain service is this install's name, not the product's,
	// so a development build and the real app cannot reach each other's tokens.
	//
	// Built here rather than in the sources block below: the agent integration
	// reads it too, and neither half implies the other.
	vault := openVault()

	// Sources are wired independently of the agent integration, and on purpose:
	// cards from a phone are useful on a board that runs no agents at all.
	// Anything that goes wrong here costs the sources and nothing else.
	var (
		sourceStore *sources.Store
		// The same manager, kept for shutdown: its plugins are processes, and
		// they have to be stopped before the app goes.
		sourcePlugins *sources.Manager
		// The inbox sorter, its stop, and where its agent runs — handed the
		// agent once there is one, below.
		inboxSorter *inboxsort.Sorter
		stopSorter  = func() {}
		sortDir     string
	)
	if dir, err := sourcesDataDir(); err != nil {
		log.Printf("sources: disabled, no data dir: %v", err)
	} else if cfg, err := sources.LoadConfig(filepath.Join(dir, "sources.json")); err != nil {
		log.Printf("sources: disabled, config error: %v", err)
	} else if sourceStore, err = openSourceStore(brd, dir); err != nil {
		log.Printf("sources: disabled, store error: %v", err)
		sourceStore = nil
	} else {
		// Every source writes to the inbox, which is this machine's board; only
		// the share dialog may pick another, and that one may be the server's.
		sourceWriter := newSourcesByBoard(seam.sourceWriter, nil, nil)
		if attached != nil {
			sourceWriter = newSourcesByBoard(seam.sourceWriter, attached.sourceWriter, onServerBoard(attached.meta))
		}
		sourceMgr := sources.NewManager(cfg, filepath.Join(dir, "sources.json"),
			sourceStore, sourceWriter, nil)
		// Lets the board answer "have we brought this one already", so a board
		// that arrived from another machine does not have everything on it
		// brought a second time.
		sourceMgr.SetBoardItems(sourceWriter)
		ingest.SetManager(sourceMgr)
		app.sources = sourceMgr
		// Manifests are a directory rather than something compiled in: a source
		// is a plugin, and with MCP a manifest is the whole adapter, so adding a
		// service somebody else already wrote a server for is a JSON file. A bad
		// one is reported and skipped — the app has to come up to be able to say
		// what was wrong with it.
		manifests, errs := sources.LoadManifests(filepath.Join(dir, sources.ManifestsDir))
		for _, err := range errs {
			log.Printf("sources: manifest not read: %v", err)
		}
		sourceMgr.SetCatalog(manifests)
		if len(manifests) > 0 {
			log.Printf("sources: %d manifest(s) from %s", len(manifests), filepath.Join(dir, sources.ManifestsDir))
		}
		sourceMgr.SetSecrets(vault)
		// Plugins come up here and go down in shutdown below. A source fed over
		// ingest needs none of this; a source with a plugin is a process, and
		// this is where it starts.
		// Where an agent source files what it finds: this app's own front door,
		// on the ingest route, with a token minted for the length of one turn.
		sourceMgr.SetIngestURL(front.url())
		sourceMgr.Start(context.Background())
		sourcePlugins = sourceMgr
		log.Printf("sources: enabled (%d registered)", len(cfg.Sources))

		// The sorter reads this process's boards alone: the inbox is this
		// machine's, and so are the boards it is sorted onto.
		sorter, err := inboxsort.New(boardadapter.NewSortBoard(boardadapter.NewWriter(srv.App())),
			inboxsort.NewStore(brd.db), filepath.Join(dir, "sorting.json"), nil)
		if err != nil {
			log.Printf("inboxsort: disabled: %v", err)
		} else {
			sourceMgr.SetArrivals(sorter.Arrived)
			sorterCtx, stop := context.WithCancel(context.Background())
			sorter.Start(sorterCtx)
			stopSorter = stop
			inboxSorter = sorter
			app.sorter = sorter
			sortDir = dir
		}
	}

	// Manager lifecycle: created after the server (needs srv.App()), stopped
	// before it (agents may still post comments during the grace period).
	var mgr *acp.Manager
	if acpEnabled {
		if events != nil {
			events.SetApp(srv.App())
		}
		dir, _ := acpDataDir()
		store, err := openAgentStore(brd, dir)
		if err != nil {
			log.Printf("acp: disabled, store error: %v", err)
		} else {
			mgr = acp.NewManager(acpCfg, filepath.Join(dir, "config.json"), store, seam.writer, emitter, nil)
			// Lets the UI open a console on a card without moving it.
			mgr.SetBoardReader(seam.reader)
			// The app-wide AI switch, read on every start rather than once:
			// it is flipped from the settings while the app runs.
			mgr.SetSwitchedOff(func() bool { return !aiEnabled() })
			// Where the GitHub token for pull-request polling comes from. It
			// was `githubToken` in config.json, which is a credential in a
			// settings file people edit by hand and paste into issues.
			mgr.SetSecrets(vault)
			mgr.SetBoardMeta(seam.meta)
			// Lets the UI give agents board accounts, so cards can be
			// assigned to them in a person property.
			mgr.SetBoardUsers(seam.users)
			if err := mgr.Start(context.Background(), seam.events); err != nil {
				log.Printf("acp: disabled, start error: %v", err)
				mgr = nil
			} else {
				// Both boards are worked, whichever one is on screen: an agent
				// on the server's card must not stop because somebody went to
				// look at their own board.
				if attached != nil {
					mgr.AddBoardSource(acp.BoardSource{
						ID: attach.target(), Writer: attached.writer, Reader: attached.reader,
						Users: attached.users, Meta: attached.meta, Events: attached.events,
					})
				}
				app.mgr = mgr
				boardTools.SetManager(mgr)
				// A source may be read by an agent rather than by a mapping —
				// the service's MCP server plus one tool of ours to file
				// through. The two registries stay independent: sources works
				// with no agents at all, and this is the one thread between
				// them, handed over rather than imported.
				if sourcePlugins != nil {
					sourcePlugins.SetAgentRunner(inboxAgentRunner{mgr})
				}
				if inboxSorter != nil {
					inboxSorter.SetAgent(sortingAgent{mgr: mgr, dir: sortDir})
				}
			}
		}
	}

	// Assigned once the application exists; the shutdown closure is built here
	// because it is handed to application.New itself.
	var stopAlerts func()

	shutdown := func() {
		if stopAlerts != nil {
			stopAlerts()
		}
		if mgr != nil {
			mgr.Shutdown(5 * time.Second)
		}
		tailnet.close()
		app.updates.close()
		stopSorter()
		if sourcePlugins != nil {
			sourcePlugins.Stop(5 * time.Second)
		}
		// The stores are not closed here: their tables are in the board's
		// database, and closing that is the board's own shutdown below.
		_ = srv.Shutdown()
	}

	// The bound methods live on App, and the webapp calls them through the
	// bindings generated from it (frontend/bindings).
	wapp := application.New(application.Options{
		Name:        "Pourquoi Pas?",
		Description: "Boards with coding agents",
		Services: []application.Service{
			application.NewService(app),
		},
		// An error a person can act on reaches the page as a code and values
		// rather than as a sentence (internal/apperr).
		MarshalError: apperr.MarshalError,
		Assets: application.AssetOptions{
			Handler: handler,
		},
		// The window is not the app. Closing the board leaves the agents
		// working and the menu bar icon standing, and these are the two halves
		// of saying so: macOS asks about the last window closing, Windows and
		// Linux ask ShouldQuit. See mode_desktop.go.
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		ShouldQuit: appShouldQuit,
		// The share extension shares by launching us with a URL, so a second
		// launch has to reach the instance that already has the board rather
		// than start a second one beside it. Wails catches the URL that
		// launched the second process — macOS delivers it as an Apple Event and
		// not in argv — and passes it here in Args.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if req, ok := shareURLFrom(data.Args); ok {
					app.openShare(req)
				}
			},
		},
		// A desktop build hands Wails the front door as its transport, which is
		// what lets the front door serve the assets. A server build leaves both
		// empty and points Wails' own server at a private loopback port.
		Transport:  front.transport(),
		Server:     front.serverOptions(),
		OnShutdown: shutdown,
	})

	// Both App and the emitter (acp.UIEmitter) reach the runtime through the
	// application instance rather than through a context, so there is no
	// startup hook to wait for before an event can be delivered.
	app.setApplication(wapp)
	app.setOrigin(front.url())
	if mgr != nil {
		// Where an agent's board tools call back to. Known only now: the front
		// door picks its port when it binds.
		mgr.SetOrigin(front.url())
	}
	emitter.SetApplication(wapp)

	// Self-updating, after the emitter has its application: the controller
	// starts emitting the moment it subscribes.
	initUpdater(wapp, app, emitter)

	// The menu bar icon and the OS notifications, after the emitter for the same
	// reason: this listens on the very bus emitter.Emit publishes to.
	stopAlerts = initAlerts(wapp, app, mgr)

	front.start()

	// A share that started the app rather than reaching a running one: the URL
	// is on our own command line, and the dialog is the only window that should
	// open — somebody who shared a link did not ask to be shown their board.
	if req, ok := shareURLFrom(os.Args); ok {
		app.openShare(req)
	} else {
		newMainWindow(wapp, front.url())
	}

	// With the app outliving its windows, the Dock icon is a way back in as
	// much as the menu bar one is.
	watchDockReopen(wapp, front.url())

	if err := wapp.Run(); err != nil {
		shutdown()
		log.Fatalf("wails run error: %v", err)
	}
}

// inboxAgentRunner is the one thread between the two registries: sources asks
// for a turn, the ACP manager runs it. It is a type of its own rather than a
// method on the manager so that internal/sources keeps naming only its own
// interface — which is what lets it work with the agent integration switched
// off entirely.
type inboxAgentRunner struct{ mgr *acp.Manager }

func (r inboxAgentRunner) RunForSource(ctx context.Context, run sources.AgentRun) (string, error) {
	servers := make([]acp.InboxServer, 0, len(run.Servers))
	for _, server := range run.Servers {
		servers = append(servers, acp.InboxServer{
			Name:    server.Name,
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
		})
	}
	return r.mgr.RunInbox(ctx, acp.InboxRun{
		Agent:   run.Agent,
		Dir:     run.Dir,
		Prompt:  run.Prompt,
		Servers: servers,
	})
}

// sortingAgent runs the inbox sorter's turns: one prompt, no tools of ours, in
// the sources' own directory because a process needs one and nothing is
// written there.
type sortingAgent struct {
	mgr *acp.Manager
	dir string
}

func (r sortingAgent) Ask(ctx context.Context, agent, prompt string) (string, error) {
	return r.mgr.RunInbox(ctx, acp.InboxRun{Agent: agent, Dir: r.dir, Prompt: prompt})
}

// openVault is the credential store, built the same way whether or not this
// install has sources or agents. The file fallback lives beside the sources
// registry for history's sake: it was written there first, and moving it would
// take somebody's stored tokens with it for no gain.
func openVault() secrets.Store {
	store := secrets.Chain{secrets.Env{Prefix: "POURQUOIPAS_SECRET_"}}
	if keychain, ok := secrets.OpenKeychain(appDirName); ok {
		return append(store, keychain)
	}
	dir, err := sourcesDataDir()
	if err != nil {
		log.Printf("secrets: no data dir, only the environment is read: %v", err)
		return store
	}
	return append(store, secrets.NewFileStore(filepath.Join(dir, "secrets.json")))
}
