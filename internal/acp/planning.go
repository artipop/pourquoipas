package acp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/artipop/pourquoipas/internal/apperr"
)

// Planning is talking a task over with an agent before a card exists: a
// conversation in the planning dialog, over the same protocol a stage runs on,
// that ends in cards the agent creates on the board (create_card) once the
// person agrees on them.
//
// It is a session with no card: it neither claims a folder nor records a row,
// and it is held read-only by its tool policy — a discussion, not the work.
// Unlike a stage it lives for many turns: each message a person sends is one
// turn, and the reply streams back as it is written (EventChunk).

// PlanningMessage is one line of the conversation as the dialog draws it.
type PlanningMessage struct {
	Role string `json:"role"` // "person" or "agent"
	Text string `json:"text"`
}

// PlanningInfo is a planning conversation as the dialog lists it.
type PlanningInfo struct {
	ID       string            `json:"id"`
	BoardID  string            `json:"boardId"`
	Agent    string            `json:"agent"`
	Folder   string            `json:"folder,omitempty"`
	Busy     bool              `json:"busy"`
	Error    string            `json:"error,omitempty"`
	Messages []PlanningMessage `json:"messages"`
	Started  time.Time         `json:"started"`
}

// planningChat is one live conversation: the session it talks through and the
// turns queued onto it.
type planningChat struct {
	s       *Session
	folder  string
	started time.Time
	turns   chan string
	stop    context.CancelFunc

	mu       sync.Mutex
	messages []PlanningMessage
	busy     bool
	err      string
	opened   bool // the planning instructions went out with the first turn
}

// planningTurnQueue is how many messages may wait behind a turn in progress. A
// person does not type ten messages ahead of a reply; the bound keeps a stuck
// agent from growing a queue without end.
const planningTurnQueue = 8

// StartPlanning opens a planning conversation on a board. folderName names a
// registered folder, or is empty for the board's drafts folder; agentName is
// the registry entry to talk to, or empty when there is only one.
func (m *Manager) StartPlanning(boardID, folderName, agentName string) (PlanningInfo, error) {
	if m.aiSwitchedOff() {
		return PlanningInfo{}, errSwitchedOff
	}
	if strings.TrimSpace(boardID) == "" {
		return PlanningInfo{}, fmt.Errorf("no board to plan on was given")
	}
	workdir, err := m.planningWorkdir(folderName)
	if err != nil {
		return PlanningInfo{}, err
	}
	agent, err := m.planningAgent(agentName)
	if err != nil {
		return PlanningInfo{}, err
	}
	net, err := m.resolveNetwork(agent)
	if err != nil {
		return PlanningInfo{}, err
	}
	cwd := workdir.Path
	folder := workdir.Name
	if m.hosted {
		// No folders on a hosted server, drafts included: the conversation
		// has the board and nothing else.
		cwd, folder = "", ""
	} else if cwd == "" {
		// No folder is a valid answer: the conversation stands in the board's
		// drafts, the same place every conversation without one has.
		cwd = m.boardFolder(boardID)
		if err := os.MkdirAll(cwd, 0o755); err != nil {
			return PlanningInfo{}, fmt.Errorf("could not create the board's drafts: %w", err)
		}
	}

	s := &Session{
		ID:          "plan-" + uuid.NewString(),
		BoardID:     boardID,
		WorkdirPath: cwd,
		Agent:       agent,
		Net:         net,
		Planning:    true,
		Policy:      planningPolicy(agent),
		status:      StatusQueued,
		allowTools:  map[string]bool{},
	}
	s.Worktree.Path = cwd

	ctx, cancel := context.WithCancel(m.rootOr())
	chat := &planningChat{
		s:       s,
		folder:  folder,
		started: time.Now(),
		turns:   make(chan string, planningTurnQueue),
		stop:    cancel,
	}
	m.mu.Lock()
	if m.active == nil {
		m.active = map[string]*Session{}
	}
	m.active[s.ID] = s
	if m.planning == nil {
		m.planning = map[string]*planningChat{}
	}
	m.planning[s.ID] = chat
	m.mu.Unlock()

	systemPrompt := m.BoardPrompt(boardID)
	opening := planningPrompt(systemPrompt, m.PlanningPrompt(), agent, workdir, m.boardAbout(boardID))

	m.wg.Add(1)
	go m.runPlanning(ctx, chat, opening)
	m.emitPlanning(chat)
	return chat.info(), nil
}

// runPlanning holds the connection for the conversation's whole life and runs
// the turns queued onto it, one at a time.
func (m *Manager) runPlanning(ctx context.Context, chat *planningChat, opening string) {
	defer m.wg.Done()
	defer m.endPlanning(chat)

	s := chat.s
	// A message sent while the agent is still starting waits in the queue.
	conv, err := m.openConversation(ctx, s)
	if err != nil {
		chat.fail(s.Net.redactProxySecret(err.Error()))
		m.emitPlanning(chat)
		return
	}
	defer conv.close()
	// Watched rather than asked each turn: an agent that dies between turns
	// takes the conversation with it, and the dialog should say so at once.
	go func() {
		select {
		case <-conv.done():
			chat.stop()
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case text := <-chat.turns:
			prompt := text
			chat.mu.Lock()
			if !chat.opened {
				// The instructions go out with the first message rather than as a
				// turn of their own: an agent told only "here are the rules"
				// answers the rules, and that is a reply nobody asked for.
				prompt = joinPrompts(opening, "The person says:\n"+text)
				chat.opened = true
			}
			chat.mu.Unlock()

			reply, err := m.runTurn(s, conv, prompt)
			stopped := s.wasCancelled()
			// A stopped reply is not a cancelled conversation: the next message
			// is a new turn (runTurn starts each one uncancelled).
			s.mu.Lock()
			s.status = StatusIdle
			s.mu.Unlock()
			chat.mu.Lock()
			if strings.TrimSpace(reply) != "" {
				chat.messages = append(chat.messages, PlanningMessage{Role: "agent", Text: reply})
			}
			if err != nil && ctx.Err() == nil && !stopped {
				chat.err = s.Net.redactProxySecret(err.Error())
			}
			chat.busy = len(chat.turns) > 0
			chat.mu.Unlock()
			m.emitPlanning(chat)
		}
	}
}

// SayToPlanning queues a person's message as the conversation's next turn. The
// reply streams back as EventChunk and lands in the conversation when the turn
// ends (EventPlanning).
func (m *Manager) SayToPlanning(id, text string) error {
	chat := m.planningChat(id)
	if chat == nil {
		return apperr.New("planning.over", "The conversation is over")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return apperr.New("planning.empty", "The message is empty")
	}
	chat.mu.Lock()
	if chat.err != "" && chat.s.Status().Terminal() {
		chat.mu.Unlock()
		return errors.New(chat.err)
	}
	chat.messages = append(chat.messages, PlanningMessage{Role: "person", Text: text})
	chat.busy = true
	chat.err = ""
	chat.mu.Unlock()
	select {
	case chat.turns <- text:
	default:
		chat.mu.Lock()
		chat.messages = chat.messages[:len(chat.messages)-1]
		chat.mu.Unlock()
		return apperr.New("planning.busy", "The agent has not answered the earlier messages yet")
	}
	m.emitPlanning(chat)
	return nil
}

// StopPlanningTurn interrupts the reply being written, keeping the
// conversation: the person saw where it was going and wants to say something
// else.
func (m *Manager) StopPlanningTurn(id string) {
	m.mu.Lock()
	s := m.active[id]
	m.mu.Unlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.turnCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// EndPlanning ends the conversation and the agent process with it — or, for one
// that failed to start, takes it off the list.
func (m *Manager) EndPlanning(id string) {
	m.mu.Lock()
	chat := m.planning[id]
	delete(m.planning, id)
	m.mu.Unlock()
	if chat != nil {
		// The reply in progress first: the loop only looks at the end of the
		// conversation between turns.
		m.StopPlanningTurn(id)
		chat.stop()
		m.emitPlanningEnded(chat)
	}
}

// Planning lists the conversations open on a board, oldest first: a dialog
// closed and opened again finds its conversation where it left it.
func (m *Manager) Planning(boardID string) []PlanningInfo {
	m.mu.Lock()
	chats := make([]*planningChat, 0, len(m.planning))
	for _, c := range m.planning {
		if boardID == "" || c.s.BoardID == boardID {
			chats = append(chats, c)
		}
	}
	m.mu.Unlock()
	out := make([]PlanningInfo, 0, len(chats))
	for _, c := range chats {
		out = append(out, c.info())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.Before(out[j].Started) })
	return out
}

func (m *Manager) planningChat(id string) *planningChat {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.planning[id]
}

// endPlanning forgets a conversation whose agent is gone. A failure to start
// stays listed with its reason until the person closes it, so the dialog can
// say what went wrong instead of the conversation vanishing.
func (m *Manager) endPlanning(chat *planningChat) {
	chat.s.mu.Lock()
	chat.s.status = StatusDone
	chat.s.mu.Unlock()
	m.mu.Lock()
	delete(m.active, chat.s.ID)
	chat.mu.Lock()
	failed := chat.err != ""
	chat.busy = false
	chat.mu.Unlock()
	if !failed {
		delete(m.planning, chat.s.ID)
	}
	m.mu.Unlock()
	if failed {
		m.emitPlanning(chat)
		return
	}
	m.emitPlanningEnded(chat)
}

func (c *planningChat) fail(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.err = reason
	c.busy = false
}

func (c *planningChat) info() PlanningInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return PlanningInfo{
		ID:       c.s.ID,
		BoardID:  c.s.BoardID,
		Agent:    c.s.Agent.Name,
		Folder:   c.folder,
		Busy:     c.busy,
		Error:    c.err,
		Messages: append([]PlanningMessage(nil), c.messages...),
		Started:  c.started,
	}
}

func (m *Manager) emitPlanning(chat *planningChat) {
	if m.ui == nil {
		return
	}
	info := chat.info()
	m.ui.Emit(EventPlanning, map[string]any{
		"id":      info.ID,
		"boardId": info.BoardID,
		"busy":    info.Busy,
		"error":   info.Error,
		"open":    true,
	})
}

func (m *Manager) emitPlanningEnded(chat *planningChat) {
	if m.ui == nil {
		return
	}
	info := chat.info()
	m.ui.Emit(EventPlanning, map[string]any{
		"id":      info.ID,
		"boardId": info.BoardID,
		"error":   info.Error,
		"open":    false,
	})
}

// planningPolicy is what a planning conversation may do unasked: read the
// folder, and nothing that writes. What the agent carries of its own still
// applies on top — a person who allowed it more allowed it everywhere.
func planningPolicy(agent AgentEntry) ToolPolicy {
	policy := ToolPolicy{"Read", "Glob", "Grep", "LS", "WebFetch", "WebSearch"}
	for _, cmd := range readOnlyShellCommands {
		policy = append(policy, "Bash("+cmd+"*)")
	}
	return append(policy, agent.AutoAllowTools...)
}

// boardAbout reads the board's name and description, or nothing: a
// conversation still opens on a board that cannot be read.
func (m *Manager) boardAbout(boardID string) BoardAbout {
	if boardID == "" || m.meta == nil {
		return BoardAbout{}
	}
	parent := m.rootCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	about, err := m.forBoard(boardID).Meta.BoardAbout(ctx, boardID)
	if err != nil {
		m.log.Warn("acp: cannot read the board's name and description", "board", boardID, "err", err)
		return BoardAbout{}
	}
	return about
}
