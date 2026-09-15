package acp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// branchState is what the card's workspace already holds when a stage starts:
// the branch, what it was cut from, the commits standing on it and the files
// they touched.
//
// Nothing said this before, and a route's second stage paid for it. A review
// stage was handed exactly the brief the stage that did the work got — the
// board's prompt, the column's, then «Task: <the card's title>» — in a copy
// full of somebody else's commits, with nothing anywhere saying those commits
// were the thing to read. The agent did the task again.
//
// The diff itself is deliberately not here. The agent is standing in the
// directory it describes, one command away from all of it, and a large diff
// pasted into a brief spends the context the reading needs.
func branchState(ctx context.Context, w WorktreeInfo) string {
	if w.Path == "" || w.BaseRef == "" {
		return ""
	}
	// Three dots: what this branch added, and not what the base moved on to
	// meanwhile — the same question a review asks.
	span := w.BaseRef + "...HEAD"
	commits := gitLines(ctx, w.Path, "log", "--oneline", "--no-decorate", span)
	files, added, removed := changedFiles(ctx, w.Path, span)
	dirty := gitLines(ctx, w.Path, "status", "--porcelain")
	if len(commits) == 0 && len(dirty) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("What is already in this working copy:")
	if w.Branch != "" {
		fmt.Fprintf(&b, "\nBranch: %s (cut from %s)", w.Branch, w.BaseRef)
	}
	if len(commits) > 0 {
		fmt.Fprintf(&b, "\nCommits (%d):", len(commits))
		writeList(&b, commits, branchStateCommits)
	}
	if len(files) > 0 {
		fmt.Fprintf(&b, "\nChanged files (%d, +%d -%d):", len(files), added, removed)
		writeList(&b, files, branchStateFiles)
		fmt.Fprintf(&b, "\nRead the whole diff with: git diff %s", span)
	}
	if len(dirty) > 0 {
		// Uncommitted work is the ordinary state in a folder worked in place:
		// composePrompt tells that stage to leave its changes for review, so a
		// stage after it that only read the commits would see nothing at all.
		fmt.Fprintf(&b, "\nUncommitted changes (%d files), read them with: git diff HEAD", len(dirty))
	}
	return b.String()
}

// How much of each list travels. A brief names what is there; a hundred file
// names are the diff by another route, and the command above reaches all of it.
const (
	branchStateCommits = 30
	branchStateFiles   = 40
)

func writeList(b *strings.Builder, lines []string, limit int) {
	for i, line := range lines {
		if i == limit {
			fmt.Fprintf(b, "\n- … and %d more", len(lines)-limit)
			return
		}
		b.WriteString("\n- " + line)
	}
}

// changedFiles reads git's own numstat: one line per file, added and removed
// counts first, a dash for each on a binary one.
func changedFiles(ctx context.Context, dir, span string) (files []string, added, removed int) {
	for _, line := range gitLines(ctx, dir, "diff", "--numstat", span) {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		plus, minus, path := parts[0], parts[1], parts[2]
		if plus == "-" || minus == "-" {
			files = append(files, path+" (binary)")
			continue
		}
		a, _ := strconv.Atoi(plus)
		r, _ := strconv.Atoi(minus)
		added += a
		removed += r
		files = append(files, fmt.Sprintf("%s (+%d -%d)", path, a, r))
	}
	return files, added, removed
}
