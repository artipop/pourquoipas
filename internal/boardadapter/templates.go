package boardadapter

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/artipop/pourquoipas/server/app"
	"github.com/artipop/pourquoipas/server/mlog"
	"github.com/artipop/pourquoipas/server/model"
)

// The board templates this app ships are ours, and they live here. The server
// module ships its own — a sales pipeline, a retrospective — and those are the
// upstream's examples: they know nothing about columns that run an agent, so a
// board made from one arrives with no automation at all. Ours carry it, which
// is why the selector offers these and hides the upstream ones
// (VISIBLE_TEMPLATE_SLUGS in the webapp). A template the user made is offered
// whatever it carries — it is theirs, and they can see what is in it.
//
// The format is what the board server imports, one board per file: the first
// line is the board, the rest its cards, views and content. Authoring one by
// hand is possible but not the intended way — build the board in the app and
// export it (board menu → *Export board archive*), then unzip the archive and
// keep the board.jsonl.

//go:embed templates/*.jsonl
var templateFiles embed.FS

// shippedTemplates is every template this build carries: the ones above, which
// every edition has, and whatever the edition adds (templates_base.go,
// templates_lifetime.go). Sorted, so the order a fresh install imports them in
// is the order they are named in rather than the order two embedded
// filesystems happened to be walked in.
//
// The two sets are kept apart on disk and joined here, which is what makes the
// edition a build tag rather than a runtime check: the base binary has no path
// to a file it does not contain.
func shippedTemplates() ([]string, error) {
	ours, err := fs.Glob(templateFiles, "templates/*.jsonl")
	if err != nil {
		return nil, err
	}
	extra, err := fs.Glob(editionTemplateFiles, "templates/*/*.jsonl")
	if err != nil {
		return nil, err
	}
	files := append(ours, extra...)
	sort.Strings(files)
	return files, nil
}

// readShippedTemplate reads one of them back. Which filesystem a path lives in
// is the path itself — the edition's are a directory deeper — so nothing else
// has to carry the answer around.
func readShippedTemplate(file string) ([]byte, error) {
	if strings.Count(file, "/") > 1 {
		return editionTemplateFiles.ReadFile(file)
	}
	return templateFiles.ReadFile(file)
}

// TemplateVersion is bumped to push edited templates into installs that already
// have them. Nothing else re-imports: a template somebody has since changed is
// theirs until this number moves, and then it is replaced.
//
// What each past bump changed is in the log, not here.
const TemplateVersion = 21

// TemplateMarkerProperty is the board property each template carries its slug
// in. Ids are regenerated on import and titles are the user's to change, so the
// marker is the only thing that still says "this board came from that file".
const TemplateMarkerProperty = "template"

// TemplateLanguageProperty is which language the installed copy was translated
// into (templatetext.go). It is beside the version and does the same job: a
// person who switches the app to English has templates in Russian, and nothing
// about the files has changed, so the version alone would leave them there for
// ever.
const TemplateLanguageProperty = "templateLanguage"

// ImportTemplates makes the app's own templates exist in the global team, and
// keeps them at the version this build ships and in the language the app is
// being read in. It is safe to call on every launch, and on a language change:
// a template that is already there at the right version and language is left
// alone.
func ImportTemplates(a *app.App, log mlog.LoggerIFace, lang string) error {
	if err := importTemplates(a, log, TemplateVersion, lang); err != nil {
		return err
	}
	return nil
}

// importTemplates takes the version as an argument so a test can install an
// older set and watch the newer one replace it — the path that only ever runs
// on an upgrade, and the one that would quietly leave two copies if it broke.
func importTemplates(a *app.App, log mlog.LoggerIFace, version int, lang string) error {
	lang = templateLanguage(lang)
	cat := templateCatalogue(lang)
	files, err := shippedTemplates()
	if err != nil {
		return fmt.Errorf("read the shipped templates: %w", err)
	}

	// As the person, not as nobody: with no user id the store returns the open
	// templates alone, and one somebody saved is private to them. Ours are open
	// and come back either way — what asking as nobody hid was exactly the
	// copies this sweep is here for. The same single user everything else in
	// this file reads the boards as; the app has one.
	existing, err := a.GetTemplateBoards(model.GlobalTeamID, model.SingleUser)
	if err != nil {
		return fmt.Errorf("read the templates already installed: %w", err)
	}
	installed := make(map[string]*model.Board, len(existing))
	for _, board := range existing {
		slug := templateSlug(board)
		if slug == "" {
			continue
		}

		// The marker is ours to give. A template somebody saved from a board
		// (the save-as-template item) is a copy of that board, properties and
		// all — and a board made from the developer template carries the marker, so the
		// copy claimed to be the developer template too. Three of them stood in the
		// picker under one name, and the sweep below would have kept replacing
		// whichever came last while the others stayed for ever.
		//
		// Disowned rather than deleted: it is somebody's own template, made on
		// purpose, and all that is wrong with it is a word it inherited. With
		// the marker gone it is listed as theirs, with a way to delete it.
		if board.CreatedBy != model.SystemUserID {
			disownTemplate(a, log, board)
			continue
		}
		installed[slug] = board
	}

	for _, file := range files {
		slug := strings.TrimSuffix(path.Base(file), ".jsonl")
		board, ok := installed[slug]

		// Taken off the list of what is installed as it is dealt with, so what
		// is left at the end is what this build no longer ships.
		delete(installed, slug)
		if ok {
			if board.TemplateVersion >= version && templateBoardLanguage(board) == lang {
				continue
			}
			// Replaced rather than patched: a template is a whole board, and
			// merging one into another has no meaning a person could predict.
			if err := a.DeleteBoard(board.ID, model.SystemUserID); err != nil {
				log.Warn("templates: cannot remove the old copy", mlog.String("template", slug), mlog.Err(err))
				continue
			}
		}
		data, err := readShippedTemplate(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}
		data, err = translateTemplate(data, cat)
		if err != nil {
			return fmt.Errorf("translate %s into %s: %w", file, lang, err)
		}
		if _, err := a.ImportBoardJSONL(bytes.NewReader(data), templateImportOptions(version, lang)); err != nil {
			// One unreadable template must not cost the others: the app is
			// usable without it, and the log says which one to look at.
			log.Error("templates: cannot install", mlog.String("template", slug), mlog.Err(err))
			continue
		}
		log.Info("templates: installed", mlog.String("template", slug))
	}

	// A template we used to ship and no longer do. It was installed by this app
	// and belongs to it, so it goes with the build that dropped it — otherwise a
	// retired template stands in the picker for ever, maintained by nobody and
	// deletable by nobody, since the app's own templates carry no delete
	// button. Boards made from it are untouched: they are the person's.
	for slug, board := range installed {
		if err := a.DeleteBoard(board.ID, model.SystemUserID); err != nil {
			log.Warn("templates: cannot remove a template this build no longer ships",
				mlog.String("template", slug), mlog.Err(err))
			continue
		}
		log.Info("templates: removed, no longer shipped", mlog.String("template", slug))
	}
	return nil
}

func templateImportOptions(version int, lang string) model.ImportArchiveOptions {
	return model.ImportArchiveOptions{
		TeamID:     model.GlobalTeamID,
		ModifiedBy: model.SystemUserID,
		BoardModifier: func(board *model.Board, cache map[string]interface{}) bool {
			return asTemplate(board, version, lang)
		},
	}
}

// asTemplate is where a file becomes a template, rather than each file having
// to say so: open (so it is offered without anybody being a member of it) and
// stamped with the version that decides when it is replaced.
func asTemplate(board *model.Board, version int, lang string) bool {
	board.IsTemplate = true
	board.Type = model.BoardTypeOpen
	board.TemplateVersion = version
	board.CreatedBy = model.SystemUserID
	if board.Properties == nil {
		board.Properties = map[string]any{}
	}
	board.Properties[TemplateLanguageProperty] = lang
	return true
}

// disownTemplate takes our marker off a template the app did not install, so it
// stops being read as one of ours: the picker lists it among the person's own,
// and the importer stops confusing it with the copy it maintains.
func disownTemplate(a *app.App, log mlog.LoggerIFace, board *model.Board) {
	patch := &model.BoardPatch{DeletedProperties: []string{TemplateMarkerProperty, TemplateLanguageProperty}}
	if _, err := a.PatchBoard(patch, board.ID, model.SystemUserID); err != nil {
		log.Warn("templates: cannot take the marker off a template we did not install",
			mlog.String("board", board.ID), mlog.Err(err))
		return
	}
	log.Info("templates: a saved template no longer claims to be one of ours",
		mlog.String("board", board.ID), mlog.String("title", board.Title))
}

// templateBoardLanguage is the language the installed copy was translated into.
// A copy installed before this existed says nothing, and nothing means the
// language the files are written in — which is what it was.
func templateBoardLanguage(board *model.Board) string {
	if board == nil || board.Properties == nil {
		return SourceLanguage
	}
	lang, _ := board.Properties[TemplateLanguageProperty].(string)
	if lang == "" {
		return SourceLanguage
	}
	return lang
}

func templateSlug(board *model.Board) string {
	if board == nil || board.Properties == nil {
		return ""
	}
	slug, _ := board.Properties[TemplateMarkerProperty].(string)
	return slug
}
