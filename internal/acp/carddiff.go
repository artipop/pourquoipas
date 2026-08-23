package acp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// A card's work read by eye. A route can put a card on a stage where nothing
// runs and a person decides — «На ревью» is exactly that — and until this
// existed there was nothing on screen to decide *from*: the branch was a label
// on the card, the commits were a line in a comment, and the changes themselves
// lived only in a directory the person had to find for themselves.
//
// It is git and nothing else. What a card's work is, is a branch in a folder on
// this machine, and every question here — what changed, against what — is one
// git already answers. Nothing is cached: the copy on disk is the truth, and a
// stored answer would be wrong the moment the agent commits again.

// How much of a diff travels to the page. A viewer is for reading, and a
// hundred-thousand-line patch is not read — it is scrolled past once and then
// costs the socket, the parser and the browser every time the panel opens.
const (
	cardDiffFileLimit  = 400
	cardDiffPatchLimit = 200 * 1024
	cardDiffTotalLimit = 2 * 1024 * 1024
)

// cardDiffTimeout bounds the git calls. A repository with a cold object cache
// can take a moment; a person waiting on a panel cannot take for ever.
const cardDiffTimeout = 30 * time.Second

// CardDiff is a card's work as a person reads it: where it is, what it is cut
// from, and the files it changed — those on the branch, and those nobody has
// committed yet.
type CardDiff struct {
	CardID string `json:"cardId"`
	Branch string `json:"branch,omitempty"`
	Base   string `json:"base,omitempty"`
	Dir    string `json:"dir,omitempty"`
	Mode   string `json:"mode,omitempty"`
	// Committed is what the branch adds to its base — the question a review
	// asks. Empty for a folder worked in place, which has no base of its own.
	Committed []DiffFile `json:"committed"`
	// Uncommitted is what is in the working copy and not in any commit. A stage
	// told to work in the person's own folder is told to leave its changes
	// exactly like this (composePrompt), so for that card it is the whole of
	// the work.
	Uncommitted []DiffFile `json:"uncommitted"`
	// Reason says why there is nothing here, in words a person can act on: no
	// work started, work on another machine, the folder is not a repository.
	// Not an error — "nothing to show" is an ordinary state of a card.
	Reason string `json:"reason,omitempty"`
}

// DiffFile is one file's change, patch and all. The counts are read off the
// patch rather than off `git diff --numstat`, which would be a second call to
// match up against this one by path — and a path is exactly what a rename
// changes.
type DiffFile struct {
	Path      string `json:"path"`
	OldPath   string `json:"oldPath,omitempty"`
	Added     int    `json:"added"`
	Removed   int    `json:"removed"`
	Binary    bool   `json:"binary,omitempty"`
	Untracked bool   `json:"untracked,omitempty"`
	Patch     string `json:"patch,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// CardDiff reads the card's work off the disk it lives on.
func (m *Manager) CardDiff(cardID string) (CardDiff, error) {
	if cardID == "" {
		return CardDiff{}, fmt.Errorf("не указана карточка")
	}
	out := CardDiff{CardID: cardID, Committed: []DiffFile{}, Uncommitted: []DiffFile{}}
	place, err := m.cardWorkPlace(cardID)
	if err != nil {
		return out, err
	}
	out.Branch, out.Base, out.Dir, out.Mode = place.Branch, place.Base, place.Path, place.Mode
	if out.Dir == "" {
		// The two cases a person can tell apart from the card itself: work that
		// has not started, and work that started somewhere else. CardWork knows
		// which (Started without Here is a card that travelled).
		if m.CardWork(cardID).Started {
			out.Reason = "работа над этой карточкой начата на другой машине — здесь её рабочей копии нет"
		} else {
			out.Reason = "по этой карточке ещё никто не работал: ни ветки, ни рабочей копии"
		}
		return out, nil
	}
	if info, err := os.Stat(out.Dir); err != nil || !info.IsDir() {
		out.Reason = fmt.Sprintf("папки %s больше нет на диске", out.Dir)
		return out, nil
	}

	ctx, cancel := context.WithTimeout(m.rootOr(), cardDiffTimeout)
	defer cancel()
	if !IsGitWorkdir(ctx, out.Dir) {
		out.Reason = fmt.Sprintf("папка %s — не git-репозиторий, смотреть изменения не по чему", out.Dir)
		return out, nil
	}

	budget := cardDiffTotalLimit
	// Three dots: what this branch added, and not what the base moved on to
	// while it was being worked — the same question a reviewer asks.
	if out.Base != "" && out.Base != "HEAD" {
		out.Committed = readDiff(ctx, out.Dir, &budget, out.Base+"...HEAD")
	}
	out.Uncommitted = readDiff(ctx, out.Dir, &budget, "HEAD")
	out.Uncommitted = append(out.Uncommitted, untrackedFiles(ctx, out.Dir)...)
	if len(out.Committed) == 0 && len(out.Uncommitted) == 0 {
		out.Reason = "в рабочей копии ничего не изменилось"
	}
	return out, nil
}

// cardWorkPlace is where this card's work is on this machine. The claim knows
// the whole of it; a card working in an ordinary folder records no claim at all
// (ClaimWorkspace writes nothing for WorkModePlain), and for that one the
// folder the card names is the answer — which is the case where the changes are
// deliberately left uncommitted for a person to read.
func (m *Manager) cardWorkPlace(cardID string) (Checkout, error) {
	if m.store != nil {
		c, ok, err := m.store.CheckoutForCard(cardID)
		if err != nil {
			return Checkout{}, err
		}
		if ok && c.Path != "" {
			return c, nil
		}
	}
	if m.reader == nil {
		return Checkout{}, nil
	}
	ctx, cancel := context.WithTimeout(m.rootOr(), 10*time.Second)
	defer cancel()
	ev, err := m.forCard(cardID).Reader.CardByID(ctx, cardID)
	if err != nil {
		return Checkout{}, nil
	}
	path, err := m.resolveWorkdir(ev)
	if err != nil {
		return Checkout{}, nil
	}
	return Checkout{Path: path, Mode: WorkModePlain}, nil
}

// readDiff runs one `git diff` and cuts the answer up per file. One call rather
// than one per file: a patch already carries its own file boundaries, and
// asking git once per path turns a folder of forty changed files into forty
// process spawns.
func readDiff(ctx context.Context, dir string, budget *int, span string) []DiffFile {
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-color", "--find-renames", span)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	files := []DiffFile{}
	for _, chunk := range splitPatch(string(out)) {
		if len(files) == cardDiffFileLimit {
			break
		}
		f := parsePatch(chunk)
		if f.Path == "" {
			continue
		}
		if *budget <= 0 {
			f.Patch, f.Truncated = "", true
		} else if len(f.Patch) > *budget {
			f.Patch, f.Truncated = f.Patch[:*budget], true
		}
		*budget -= len(f.Patch)
		files = append(files, f)
	}
	return files
}

// splitPatch cuts git's output at each file header. Split on the header line
// itself rather than on a blank line: a patch of a file whose content contains
// "diff --git" at the start of a line is a patch of a patch, which this
// repository has plenty of.
func splitPatch(out string) []string {
	var chunks []string
	var cur strings.Builder
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if cur.Len() > 0 {
				chunks = append(chunks, cur.String())
				cur.Reset()
			}
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if strings.TrimSpace(cur.String()) != "" {
		chunks = append(chunks, cur.String())
	}
	return chunks
}

// parsePatch reads one file's chunk: which file it is about, how much moved,
// and whether git refused to describe it at all.
func parsePatch(chunk string) DiffFile {
	f := DiffFile{Patch: chunk}
	inHunk := false
	for _, line := range strings.Split(chunk, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			f.Path, f.OldPath = patchPaths(line)
		case strings.HasPrefix(line, "rename from "):
			f.OldPath = strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			f.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
			f.Binary = true
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk:
			// Still in the header: +++ and --- are not changed lines.
		case strings.HasPrefix(line, "+"):
			f.Added++
		case strings.HasPrefix(line, "-"):
			f.Removed++
		}
	}
	if f.OldPath == f.Path {
		f.OldPath = ""
	}
	if len(f.Patch) > cardDiffPatchLimit {
		f.Patch, f.Truncated = f.Patch[:cardDiffPatchLimit], true
	}
	return f
}

// patchPaths reads the two paths off a `diff --git a/x b/y` line. Git quotes a
// path with spaces in it and this does not unquote: what it is used for is a
// label and a key, and a wrong label on a strange path is better than dropping
// the file out of the list.
func patchPaths(line string) (newPath, oldPath string) {
	rest := strings.TrimPrefix(line, "diff --git ")
	// Split on " b/" rather than on space: half the paths in any repository
	// contain no spaces and the other half is what this guards.
	if i := strings.Index(rest, " b/"); i > 0 {
		oldPath = strings.TrimPrefix(rest[:i], "a/")
		newPath = strings.TrimPrefix(rest[i+1:], "b/")
		return newPath, oldPath
	}
	fields := strings.Fields(rest)
	if len(fields) == 2 {
		return strings.TrimPrefix(fields[1], "b/"), strings.TrimPrefix(fields[0], "a/")
	}
	return "", ""
}

// untrackedFiles are the files git is not watching yet. They carry no patch:
// a new file's whole content is not a diff, and an agent that has just written
// one has usually written several — reading them is what the editor is for.
// Naming them is the part that matters, because otherwise a whole stage's work
// can be invisible in the viewer.
func untrackedFiles(ctx context.Context, dir string) []DiffFile {
	var out []DiffFile
	for _, path := range gitLines(ctx, dir, "ls-files", "--others", "--exclude-standard") {
		if len(out) == cardDiffFileLimit {
			break
		}
		out = append(out, DiffFile{Path: path, Untracked: true})
	}
	return out
}

// CardTitle is what the card is called, for a window that has to be named after
// it. Empty when the board cannot be reached — a window with a fallback title
// is better than no window.
func (m *Manager) CardTitle(cardID string) string {
	if m.reader == nil || cardID == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(m.rootOr(), 10*time.Second)
	defer cancel()
	ev, err := m.forCard(cardID).Reader.CardByID(ctx, cardID)
	if err != nil {
		return ""
	}
	return ev.Title
}
