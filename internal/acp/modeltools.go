package acp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/modelapi"
	"github.com/artipop/pourquoipas/internal/procgroup"
)

// What a CLI brings of its own and a model has to be given: the files of the
// folder it stands in and a shell. Only on a machine, only in that folder, and
// a command goes through the same policy and the same question as a CLI's.

// folderTools are the folder's tools; writing ones only when the session may
// change things — a planning conversation reads.
func folderTools(c *modelConversation, write bool) []modelTool {
	root := c.s.Worktree.Path
	tools := []modelTool{
		{
			spec: modelapi.Tool{
				Name:        "read_file",
				Description: "Read a text file of the folder. Lines are numbered. Give offset and limit (in lines) to read part of a long file.",
				Schema: object(map[string]any{
					"path":   str("the file, relative to the folder"),
					"offset": integer("the first line to read, from 1"),
					"limit":  integer("how many lines to read"),
				}, "path"),
			},
			run: func(_ context.Context, input json.RawMessage) (string, error) {
				var in struct {
					Path          string
					Offset, Limit int
				}
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				path, err := inFolder(root, in.Path)
				if err != nil {
					return "", err
				}
				return readLines(path, in.Offset, in.Limit)
			},
		},
		{
			spec: modelapi.Tool{
				Name:        "list_files",
				Description: "List the files and directories under a directory of the folder (directories end in /). Skips .git and node_modules.",
				Schema: object(map[string]any{
					"path":  str("the directory, relative to the folder; empty for the folder itself"),
					"depth": integer("how many levels down to go, 1 to 5; 2 if not given"),
				}),
			},
			run: func(_ context.Context, input json.RawMessage) (string, error) {
				var in struct {
					Path  string
					Depth int
				}
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				dir, err := inFolder(root, in.Path)
				if err != nil {
					return "", err
				}
				return listFiles(root, dir, in.Depth)
			},
		},
		{
			spec: modelapi.Tool{
				Name:        "search",
				Description: "Search the folder's text files for a regular expression (Go syntax). Answers file:line: text, at most 200 matches.",
				Schema: object(map[string]any{
					"pattern": str("the regular expression"),
					"path":    str("a directory or file to search in, relative to the folder; empty for all of it"),
				}, "pattern"),
			},
			run: func(ctx context.Context, input json.RawMessage) (string, error) {
				var in struct{ Pattern, Path string }
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				re, err := regexp.Compile(in.Pattern)
				if err != nil {
					return "", fmt.Errorf("the pattern does not compile: %w", err)
				}
				start, err := inFolder(root, in.Path)
				if err != nil {
					return "", err
				}
				return searchFiles(ctx, root, start, re)
			},
		},
	}
	if !write {
		return tools
	}
	return append(tools,
		modelTool{
			spec: modelapi.Tool{
				Name:        "write_file",
				Description: "Write a whole file of the folder, creating it and its directories if needed. For a change to part of an existing file use edit_file.",
				Schema: object(map[string]any{
					"path":    str("the file, relative to the folder"),
					"content": str("the file's whole new content"),
				}, "path", "content"),
			},
			run: func(_ context.Context, input json.RawMessage) (string, error) {
				var in struct{ Path, Content string }
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				path, err := inFolder(root, in.Path)
				if err != nil {
					return "", err
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return "", err
				}
				if err := os.WriteFile(path, []byte(in.Content), 0o644); err != nil {
					return "", err
				}
				return fmt.Sprintf("Wrote %s (%d bytes).", in.Path, len(in.Content)), nil
			},
		},
		modelTool{
			spec: modelapi.Tool{
				Name:        "edit_file",
				Description: "Replace text in a file of the folder. old_text must appear exactly once, unless replace_all is set; copy it from read_file without the line numbers.",
				Schema: object(map[string]any{
					"path":        str("the file, relative to the folder"),
					"old_text":    str("the exact text to replace"),
					"new_text":    str("what to put in its place"),
					"replace_all": map[string]any{"type": "boolean", "description": "replace every occurrence"},
				}, "path", "old_text", "new_text"),
			},
			run: func(_ context.Context, input json.RawMessage) (string, error) {
				var in struct {
					Path       string
					OldText    string `json:"old_text"`
					NewText    string `json:"new_text"`
					ReplaceAll bool   `json:"replace_all"`
				}
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				path, err := inFolder(root, in.Path)
				if err != nil {
					return "", err
				}
				return editFile(path, in.OldText, in.NewText, in.ReplaceAll)
			},
		},
		modelTool{
			spec: modelapi.Tool{
				Name: "run_command",
				Description: "Run a shell command in the folder and get its output (stdout and stderr together). " +
					"A command the person has not allowed waits for their answer first. Do not push.",
				Schema: object(map[string]any{
					"command":         str("the command line"),
					"timeout_seconds": integer("how long it may run; 600 if not given"),
				}, "command"),
			},
			run: func(ctx context.Context, input json.RawMessage) (string, error) {
				var in struct {
					Command        string
					TimeoutSeconds int `json:"timeout_seconds"`
				}
				if err := json.Unmarshal(input, &in); err != nil {
					return "", err
				}
				if strings.TrimSpace(in.Command) == "" {
					return "", errors.New("the command is empty")
				}
				if !c.mayRun(ctx, in.Command) {
					return "", errors.New("the person did not allow this command")
				}
				return runCommand(ctx, root, in.Command, in.TimeoutSeconds)
			},
		},
	)
}

// askPersonTool is a question to the person whose card it is, drawn where
// every agent's question is drawn and answered there.
func askPersonTool(c *modelConversation) modelTool {
	return modelTool{
		spec: modelapi.Tool{
			Name: "ask_person",
			Description: "Ask the person a question and wait for the answer. Use it for a decision only they can make. " +
				"Offer options when the answer is one of a few; let them type when it is not.",
			Schema: object(map[string]any{
				"question":   str("the question, in the person's language"),
				"options":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "answers to choose from, if any"},
				"allow_text": map[string]any{"type": "boolean", "description": "whether they may type an answer of their own; true if not given"},
			}, "question"),
		},
		run: func(ctx context.Context, input json.RawMessage) (string, error) {
			var in struct {
				Question  string
				Options   []string
				AllowText *bool `json:"allow_text"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return "", err
			}
			if strings.TrimSpace(in.Question) == "" {
				return "", errors.New("the question is empty")
			}
			q := Question{Kind: QuestionForm, Text: in.Question, FreeText: in.AllowText == nil || *in.AllowText || len(in.Options) == 0}
			for i, label := range in.Options {
				q.Options = append(q.Options, QuestionOption{ID: fmt.Sprint(i + 1), Label: label})
			}
			answer := c.m.ask(ctx, c.s, q)
			switch {
			case answer.Declined || answer.empty():
				return "The person did not answer. Decide without them, or stop and say what you need.", nil
			case strings.TrimSpace(answer.Text) != "":
				return "The person answered: " + strings.TrimSpace(answer.Text), nil
			}
			for _, opt := range q.Options {
				if opt.ID == answer.OptionID {
					return "The person chose: " + opt.Label, nil
				}
			}
			return "The person answered: " + answer.OptionID, nil
		},
	}
}

// mayRun is the policy a CLI's Bash goes through, then the question a CLI's
// permission request becomes.
func (c *modelConversation) mayRun(ctx context.Context, command string) bool {
	input := map[string]any{"command": command}
	if c.s.autoAllowed("Bash", input, c.m.cfg) || c.s.toolAllowed("Bash") {
		c.recordDecision(command, "allow", true)
		return true
	}
	asked := apperr.Say("question.allow-command", "Run the command: {command}?", "command", command)
	answer := c.m.ask(ctx, c.s, Question{
		Kind: QuestionPermission,
		Text: asked.Text,
		Say:  &asked,
		Tool: "Bash",
		Options: []QuestionOption{
			{ID: "allow_once", Label: "Allow", Kind: "allow_once"},
			{ID: "allow_always", Label: "Allow always", Kind: "allow_always"},
			{ID: "reject_once", Label: "Refuse", Kind: "reject_once"},
		},
	})
	switch answer.OptionID {
	case "allow_always":
		c.s.allowToolAlways("Bash")
		fallthrough
	case "allow_once":
		c.recordDecision(command, "allow", false)
		return true
	}
	c.recordDecision(command, "reject", answer.Declined)
	return false
}

func (c *modelConversation) recordDecision(command, decision string, byPolicy bool) {
	c.s.appendEvent(c.m, "permission", map[string]any{
		"tool": "Bash", "title": command, "decision": decision, "byPolicy": byPolicy,
	})
}

func object(props map[string]any, required ...string) map[string]any {
	out := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func integer(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

// inFolder resolves a path the model gave against the folder and refuses one
// outside it, whichever way it is spelled (sessionClient.jail's rule).
func inFolder(root, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path == "." {
		return root, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	clean := filepath.Clean(path)
	for _, candidate := range []string{clean, resolvedPath(clean)} {
		for _, r := range []string{root, resolvedPath(root)} {
			if underRoot(candidate, r) {
				return clean, nil
			}
		}
	}
	return "", fmt.Errorf("%s is outside the folder", path)
}

const (
	maxReadLines   = 2000
	maxFileBytes   = 4 << 20
	maxListEntries = 1000
	maxMatches     = 200
)

func readLines(path string, offset, limit int) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory: use list_files", filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if offset < 1 {
		offset = 1
	}
	if limit <= 0 || limit > maxReadLines {
		limit = maxReadLines
	}
	var b strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxFileBytes)
	line, shown := 0, 0
	for scanner.Scan() {
		line++
		if line < offset {
			continue
		}
		if shown == limit {
			fmt.Fprintf(&b, "… (more lines follow; read on from line %d)\n", line)
			break
		}
		fmt.Fprintf(&b, "%6d\t%s\n", line, scanner.Text())
		shown++
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if line == 0 {
		return "(the file is empty)", nil
	}
	return b.String(), nil
}

func skippedDir(name string) bool {
	return name == ".git" || name == "node_modules"
}

func listFiles(root, dir string, depth int) (string, error) {
	if depth <= 0 {
		depth = 2
	}
	if depth > 5 {
		depth = 5
	}
	base := strings.Count(dir, string(filepath.Separator))
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == dir {
			return nil
		}
		if d.IsDir() && skippedDir(d.Name()) {
			return fs.SkipDir
		}
		if len(out) >= maxListEntries {
			return fs.SkipAll
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			out = append(out, rel+"/")
			if strings.Count(path, string(filepath.Separator))-base >= depth {
				return fs.SkipDir
			}
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "(empty)", nil
	}
	sort.Strings(out)
	text := strings.Join(out, "\n")
	if len(out) >= maxListEntries {
		text += "\n… (cut off; list a narrower directory)"
	}
	return text, nil
}

func searchFiles(ctx context.Context, root, start string, re *regexp.Regexp) (string, error) {
	var out []string
	err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if d.IsDir() {
			if path != start && skippedDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if len(out) >= maxMatches {
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxFileBytes {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			return nil // unreadable, or not text
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				out = append(out, fmt.Sprintf("%s:%d: %s", rel, i+1, truncateRunes(strings.TrimRight(line, "\r"), 300)))
				if len(out) >= maxMatches {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "No matches.", nil
	}
	text := strings.Join(out, "\n")
	if len(out) >= maxMatches {
		text += "\n… (more matches; narrow the search)"
	}
	return text, nil
}

func editFile(path, oldText, newText string, all bool) (string, error) {
	if oldText == "" {
		return "", errors.New("old_text is empty: to write a new file use write_file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	count := strings.Count(content, oldText)
	switch {
	case count == 0:
		return "", errors.New("old_text does not appear in the file: read it again and copy the text exactly")
	case count > 1 && !all:
		return "", fmt.Errorf("old_text appears %d times: give more of the surrounding text, or set replace_all", count)
	}
	if all {
		content = strings.ReplaceAll(content, oldText, newText)
	} else {
		content = strings.Replace(content, oldText, newText, 1)
	}
	info, _ := os.Stat(path)
	mode := fs.FileMode(0o644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return "", err
	}
	return fmt.Sprintf("Replaced %d occurrence(s).", map[bool]int{true: count, false: 1}[all]), nil
}

const maxCommandOutput = 30000

// runCommand runs one command line in its own process group, so a cancelled
// turn takes everything it started with it.
func runCommand(ctx context.Context, dir, command string, timeoutSeconds int) (string, error) {
	if timeoutSeconds <= 0 || timeoutSeconds > 3600 {
		timeoutSeconds = 600
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	argv := []string{"/bin/sh", "-c", "{ " + command + "\n} 2>&1"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/C", command + " 2>&1"}
	}
	proc, err := procgroup.Spawn(ctx, argv, dir, nil)
	if err != nil {
		return "", err
	}
	_ = proc.Stdin.Close()
	stop := context.AfterFunc(ctx, func() { proc.KillGroup(2 * time.Second) })
	defer stop()

	var out bytes.Buffer
	_, _ = io.Copy(&limitedWriter{w: &out, left: maxCommandOutput}, proc.Stdout)
	waitErr := proc.Wait()
	text := out.String()
	if out.Len() >= maxCommandOutput {
		text += "\n… (output cut off)"
	}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		return text, fmt.Errorf("the command ran out of time (%ds)\n%s", timeoutSeconds, text)
	case ctx.Err() != nil:
		return text, errors.New("the command was stopped")
	case waitErr != nil:
		return "", fmt.Errorf("%v\n%s", waitErr, text)
	}
	return text, nil
}

// limitedWriter keeps the first bytes and drops the rest without failing the
// copy, so a chatty command still runs to its end.
type limitedWriter struct {
	w    io.Writer
	left int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if l.left > 0 {
		keep := p
		if len(keep) > l.left {
			keep = keep[:l.left]
		}
		l.left -= len(keep)
		_, _ = l.w.Write(keep)
	}
	return n, nil
}
