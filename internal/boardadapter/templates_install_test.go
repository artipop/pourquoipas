package boardadapter

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/server"
	"github.com/artipop/pourquoipas/server/services/config"
	"github.com/artipop/pourquoipas/server/services/notify"
	"github.com/artipop/pourquoipas/server/services/permissions/localpermissions"

	"github.com/artipop/pourquoipas/server/mlog"
)

// Installing the templates is the half nothing else covers: the files are
// embedded, the ids are regenerated on import, and whether the four boards end
// up in the global team as templates — once, not once per launch — is decided
// by a store nobody sees until the app is opened. So this runs the real board
// server against a temporary database and looks.

func newTestApp(t *testing.T) *app.App {
	t.Helper()
	return newTestAppWith(t)
}

// newTestAppWith is the same board server with notify backends registered, which
// is what a test needs when it cares whether a write is heard — a card moving is
// only a card moving if the trigger sees it.
func newTestAppWith(t *testing.T, backends ...notify.Backend) *app.App {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "board.db")

	logger, err := mlog.NewLogger()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logger.Shutdown() })

	cfg := &config.Configuration{
		DBType:            "sqlite3",
		DBConfigString:    dsn,
		FilesDriver:       "local",
		FilesPath:         filepath.Join(dir, "files"),
		WebPath:           dir,
		AuthMode:          "native",
		SessionExpireTime: 259200000000,
	}
	store, _, err := server.NewStore(cfg, true, logger)
	if err != nil {
		t.Skipf("cannot open a board store here: %v", err)
	}
	t.Cleanup(func() { _ = store.Shutdown() })

	srv, err := server.New(server.Params{
		Cfg:                cfg,
		SingleUserToken:    "test-token",
		DBStore:            store,
		Logger:             logger,
		NotifyBackends:     backends,
		PermissionsService: localpermissions.New(store, logger),
	})
	if err != nil {
		t.Fatalf("cannot build the board server: %v", err)
	}
	t.Cleanup(srv.App().Shutdown)
	return srv.App()
}

func installedTemplates(t *testing.T, a *app.App) map[string]*model.Board {
	t.Helper()
	boards, err := a.GetTemplateBoards(model.GlobalTeamID, "")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*model.Board{}
	for _, board := range boards {
		if slug := templateSlug(board); slug != "" {
			if _, twice := out[slug]; twice {
				t.Errorf("%q is installed more than once", slug)
			}
			out[slug] = board
		}
	}
	return out
}

func TestTemplatesAreInstalledOnceAndStayThere(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()

	if err := ImportTemplates(a, logger); err != nil {
		t.Fatalf("first launch: %v", err)
	}
	first := installedTemplates(t, a)

	var slugs []string
	for slug, board := range first {
		slugs = append(slugs, slug)
		if !board.IsTemplate || board.Type != model.BoardTypeOpen {
			t.Errorf("%q is installed as %+v", slug, board)
		}
		if board.TemplateVersion != TemplateVersion {
			t.Errorf("%q is at version %d", slug, board.TemplateVersion)
		}
		if strings.TrimSpace(board.Title) == "" {
			t.Errorf("%q has no title", slug)
		}
	}
	sort.Strings(slugs)
	// What this build ships, rather than a list written here: the paid edition
	// carries two more boards (internal/edition), and a test naming the three
	// core ones would fail on it for being right.
	want := shippedSlugs(t)
	if strings.Join(slugs, ",") != strings.Join(want, ",") {
		t.Fatalf("installed %v, expected %v", slugs, want)
	}

	// A second launch must find them and leave them alone — including their
	// ids, since a board somebody made from one keeps no link to it but the
	// registry's routes are filed under the board they were taken from.
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatalf("second launch: %v", err)
	}
	for slug, board := range installedTemplates(t, a) {
		if first[slug] == nil || first[slug].ID != board.ID {
			t.Errorf("%q was reinstalled on the second launch", slug)
		}
	}
}

// The templates are stored once, in English, and a board made from one is in
// the language of whoever made it. The language was the install's when they
// were installed in it, and on a server two people reading in two languages
// reinstalled them for each other at every page load.
func TestABoardMadeFromATemplateIsInTheLanguageOfWhoMadeIt(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}
	a.SetTemplateText(TranslateTemplateCopy)
	dev := installedTemplates(t, a)["developer-tasks"]

	ru := copyTemplate(t, a, dev.ID, "ru")
	if ru.Boards[0].Title != "Разработка" {
		t.Errorf("the Russian copy is called %q", ru.Boards[0].Title)
	}
	en := copyTemplate(t, a, dev.ID, "en")
	if en.Boards[0].Title != "Development" {
		t.Errorf("the English copy is called %q", en.Boards[0].Title)
	}

	// And the template itself is what it was: the same board, still English.
	again := installedTemplates(t, a)["developer-tasks"]
	if again.ID != dev.ID || again.Title != "Development" {
		t.Errorf("the template became %q (%s), was %q (%s)", again.Title, again.ID, dev.Title, dev.ID)
	}
}

func copyTemplate(t *testing.T, a *app.App, templateID, lang string) *model.BoardsAndBlocks {
	t.Helper()
	bab, _, err := a.DuplicateBoard(templateID, model.SingleUser, model.GlobalTeamID, false, lang)
	if err != nil {
		t.Fatalf("copy the template into %s: %v", lang, err)
	}
	if len(bab.Boards) != 1 || bab.Boards[0].IsTemplate {
		t.Fatalf("the copy is %+v", bab.Boards)
	}
	return bab
}

// The upgrade path: an edited template reaches an install that already has the
// old one, and reaches it as a replacement rather than as a second board with
// the same name.
func TestABumpedVersionReplacesTheInstalledCopy(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := importTemplates(a, logger, 1); err != nil {
		t.Fatal(err)
	}
	before := installedTemplates(t, a)

	if err := importTemplates(a, logger, 2); err != nil {
		t.Fatal(err)
	}
	after := installedTemplates(t, a) // fails the test if any slug is there twice
	if len(after) != len(before) {
		t.Fatalf("%d templates installed, expected %d", len(after), len(before))
	}
	for slug, board := range after {
		if board.TemplateVersion != 2 {
			t.Errorf("%q is still at version %d", slug, board.TemplateVersion)
		}
		if board.ID == before[slug].ID {
			t.Errorf("%q was left as it was", slug)
		}
	}
}

// The cards and the board's own records have to survive the import, or the
// board arrives as a title with nothing behind it.
func TestAnInstalledTemplateKeepsItsCardsAndRecords(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}

	board := installedTemplates(t, a)["home-chores"]
	if board == nil {
		t.Fatal("«Личное» is not installed")
	}
	if _, ok := board.Properties["projectProperty"]; !ok {
		t.Error("the board arrived without the field its cards name a folder in")
	}
	if len(board.CardProperties) == 0 {
		t.Error("the board arrived with no card properties")
	}

	cards, err := a.GetBlocks(board.ID, "", string(model.TypeCard))
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) == 0 {
		t.Error("the board arrived with no cards")
	}
}

// the save-as-template item copies the board, properties and all — and a board
// made from the developer template carries the marker that says it *is* the developer template. So
// a saved copy claimed to be one of ours: three of them stood in the picker
// under one name, and the importer kept replacing whichever came last while the
// rest stayed for ever.
//
// The marker is the app's to give. A template it did not install is disowned —
// listed among the person's own, where it can be deleted — rather than deleted
// out from under somebody who made it on purpose.
func TestATemplateSomebodySavedStopsClaimingToBeOurs(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}
	ours := installedTemplates(t, a)["developer-tasks"]
	if ours == nil {
		t.Fatal("«Разработка» is not installed")
	}

	// The copy: same marker, same version, made by a person.
	copied := &model.Board{
		TeamID:          model.GlobalTeamID,
		Title:           ours.Title,
		Type:            model.BoardTypeOpen,
		IsTemplate:      true,
		TemplateVersion: ours.TemplateVersion,
		CreatedBy:       model.SingleUser,
		Properties:      map[string]any{TemplateMarkerProperty: "developer-tasks"},
	}
	saved, err := a.CreateBoard(copied, model.SingleUser, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}

	// installedTemplates fails the test if a slug is there twice, so getting an
	// answer at all is half the assertion.
	if kept := installedTemplates(t, a)["developer-tasks"]; kept == nil || kept.ID != ours.ID {
		t.Errorf("the app's own copy is %v, expected %s", kept, ours.ID)
	}
	saved, err = a.GetBoard(saved.ID)
	if err != nil {
		t.Fatalf("the saved template was removed rather than disowned: %v", err)
	}
	if slug := templateSlug(saved); slug != "" {
		t.Errorf("the saved template still claims to be %q", slug)
	}
	if !saved.IsTemplate || saved.Title != ours.Title {
		t.Errorf("disowning changed more than the marker: %+v", saved)
	}
}

// A template this build no longer ships goes with the build that dropped it.
// It was installed by the app and is the app's: nobody else maintains it, and
// the picker gives the app's own templates no delete button, so one left behind
// would stand there for ever. Boards made from it are untouched — those are the
// person's.
func TestATemplateThisBuildNoLongerShipsIsRemoved(t *testing.T) {
	a := newTestApp(t)
	logger, _ := mlog.NewLogger()
	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}

	// A template from an older build: same shape as ours, a slug no file has.
	retired, err := a.CreateBoard(&model.Board{
		TeamID: model.GlobalTeamID, Title: "Покупки и меню", Type: model.BoardTypeOpen,
		IsTemplate: true, TemplateVersion: TemplateVersion,
		Properties: map[string]any{TemplateMarkerProperty: "shopping-and-meals"},
	}, model.SystemUserID, false)
	if err != nil {
		t.Fatal(err)
	}
	// …and a board somebody made from it, which is not ours to touch.
	made, err := a.CreateBoard(&model.Board{
		TeamID: model.GlobalTeamID, Title: "Мои покупки", Type: model.BoardTypeOpen,
		CreatedBy:  model.SingleUser,
		Properties: map[string]any{TemplateMarkerProperty: "shopping-and-meals"},
	}, model.SingleUser, false)
	if err != nil {
		t.Fatal(err)
	}

	if err := ImportTemplates(a, logger); err != nil {
		t.Fatal(err)
	}

	if _, err := a.GetBoard(retired.ID); err == nil {
		t.Error("a template the build no longer ships is still installed")
	}
	if _, err := a.GetBoard(made.ID); err != nil {
		t.Errorf("a board made from it was taken away with it: %v", err)
	}
	// And the ones this build does ship are still there, at this version.
	if got, want := installedTemplates(t, a), shippedSlugs(t); len(got) != len(want) {
		t.Errorf("installed %d templates, want the %d this build ships", len(got), len(want))
	}
}
