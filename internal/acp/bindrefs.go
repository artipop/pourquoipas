package acp

import (
	"slices"
	"strings"
)

// Binding a board's automation to registry ids.
//
// This is for data the app does not insert: a board's columns live in its own
// JSON, written by a template, by another machine, or by an older version, so
// there is no write that could have handed out an id. Everything
// here runs once, where a board is read (boardseed.go), and what it resolves is
// written back to the board.
//
// An unresolvable name is left alone rather than cleared: a board from another
// machine names a target registered there, and erasing it would turn "register
// this target" into "set this column up again from memory". validateColumn
// then refuses it, which is what routes the column into rememberUnadopted.

// bindAgentRefs folds an agent entry's legacy proxy name into a proxy id.
func bindAgentRefs(a *AgentEntry, proxies []ProxyEntry) bool {
	if strings.TrimSpace(a.ProxyID) != "" || strings.TrimSpace(a.ProxyName) == "" {
		return false
	}
	p, ok := proxyByName(proxies, a.ProxyName)
	if !ok {
		return false
	}
	a.ProxyID, a.ProxyName = p.ID, ""
	return true
}

// bindCrew folds a list of agent names into ids, keeping the order somebody
// listed them in — the crew is tried in that order, so it is a setting and not
// a set. A name this machine cannot resolve is kept.
func bindCrew(ids, names []string, agents []AgentEntry) (bound, kept []string, changed bool) {
	bound = ids
	for _, name := range names {
		// A blank is not a name: the editor's own empty row and a hand-edited
		// list both produce one, and refusing it would make an empty field an
		// error message about an agent called " ".
		if strings.TrimSpace(name) == "" {
			changed = true
			continue
		}
		a, ok := agentByName(agents, name)
		if !ok || a.ID == "" {
			// An entry with no id cannot be pointed at, so it is not an answer
			// to "who works here" even though the name matches something.
			kept = append(kept, name)
			continue
		}
		if !slices.Contains(bound, a.ID) {
			bound = append(bound, a.ID)
		}
		changed = true
	}
	return bound, kept, changed
}

// crewNames turns a roster of ids into the names to show. A crew is stored by
// id and read by a person — or by a model, which is told the board in the
// vocabulary a person uses — so the two directions are separate on purpose. An
// id nothing answers to is dropped: it names an agent this machine has not got,
// and printing the id would say less than saying nothing.
func (m *Manager) crewNames(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	m.cfgMu.RLock()
	agents := append([]AgentEntry(nil), m.cfg.Agents...)
	m.cfgMu.RUnlock()
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if a, ok := agentByID(agents, id); ok {
			out = append(out, a.Name)
		}
	}
	return out
}

func workdirNamed(workdirs []WorkdirEntry, name string) (WorkdirEntry, bool) {
	name = strings.TrimSpace(name)
	for _, w := range workdirs {
		if strings.EqualFold(strings.TrimSpace(w.Name), name) {
			return w, true
		}
	}
	return WorkdirEntry{}, false
}

func workdirByID(workdirs []WorkdirEntry, id string) (WorkdirEntry, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return WorkdirEntry{}, false
	}
	for _, w := range workdirs {
		if w.ID == id {
			return w, true
		}
	}
	return WorkdirEntry{}, false
}

func proxyByName(proxies []ProxyEntry, name string) (ProxyEntry, bool) {
	name = strings.TrimSpace(name)
	for _, p := range proxies {
		if strings.EqualFold(strings.TrimSpace(p.Name), name) {
			return p, true
		}
	}
	return ProxyEntry{}, false
}

func proxyByID(proxies []ProxyEntry, id string) (ProxyEntry, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ProxyEntry{}, false
	}
	for _, p := range proxies {
		if p.ID == id {
			return p, true
		}
	}
	return ProxyEntry{}, false
}

func agentByID(agents []AgentEntry, id string) (AgentEntry, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AgentEntry{}, false
	}
	for _, a := range agents {
		if a.ID == id {
			return a, true
		}
	}
	return AgentEntry{}, false
}

func agentByName(agents []AgentEntry, name string) (AgentEntry, bool) {
	name = strings.TrimSpace(name)
	for _, a := range agents {
		if sameAgentName(a.Name, name) {
			return a, true
		}
	}
	return AgentEntry{}, false
}

// optionNamed is the last place a column is found by its name, and it exists
// only to stop finding one that way: what it resolves is written back as an id.
func optionNamed(options []Column, name string) (Column, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Column{}, false
	}
	for _, o := range options {
		if strings.EqualFold(strings.TrimSpace(o.Name), name) {
			return o, true
		}
	}
	return Column{}, false
}

// workdirForPath is the registry entry that lives at this path.
func workdirForPath(workdirs []WorkdirEntry, path string) (WorkdirEntry, bool) {
	if strings.TrimSpace(path) == "" {
		return WorkdirEntry{}, false
	}
	for _, w := range workdirs {
		if w.Path == path {
			return w, true
		}
	}
	return WorkdirEntry{}, false
}
