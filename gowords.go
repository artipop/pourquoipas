package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/artipop/xciii/internal/acp"
	"github.com/artipop/xciii/internal/apperr"
)

// The words Go draws itself: the menu bar, the OS notification and the titles
// of the windows it opens. Everything else a person reads is drawn by the page,
// and so are these words — they live in frontend/i18n with every other string
// and the page hands them over (UseWords), because the moment they are drawn
// may be one with no page to ask: the board's window closed, the app started by
// a share. Go keeps no catalogue of its own (docs/i18n.md).
//
// Until the page has spoken, and in a server build where it never needs to,
// these are the English defaults the page's own messages carry.

// uiWords is what the page sends. Every field is a finished sentence in the
// page's language, with {agent}, {title} or {count} left for Go to fill.
type uiWords struct {
	AgentAsking   string `json:"agentAsking"`
	AnAgentAsking string `json:"anAgentAsking"`
	OpenToAnswer  string `json:"openToAnswer"`
	Open          string `json:"open"`
	Quit          string `json:"quit"`
	DiffWindow    string `json:"diffWindow"`
	ShareWindow   string `json:"shareWindow"`

	// MoreWaits says how many waits the menu stopped naming. The plural is the
	// page's to decide: it sends one form per CLDR category and which category
	// each count takes, because the rule is its Intl.PluralRules and Go has no
	// copy of it. A count past the list takes "other", which for the numbers a
	// menu of waiting agents can reach is a moot point.
	MoreWaits pluralWords `json:"moreWaits"`

	// Messages are the page's sentences for the codes Go has to word by
	// itself (apperr), keyed by code: a permission question reaches the OS
	// notification without passing the page.
	Messages map[string]string `json:"messages,omitempty"`
}

type pluralWords struct {
	Forms  map[string]string `json:"forms"`
	Counts []string          `json:"counts"`
}

var englishWords = uiWords{
	AgentAsking:   "{agent} is asking",
	AnAgentAsking: "The agent is asking",
	OpenToAnswer:  "Open the board to answer.",
	Open:          "Open",
	Quit:          "Quit",
	DiffWindow:    "Changes · {title}",
	ShareWindow:   "Save to a board",
	MoreWaits:     pluralWords{Forms: map[string]string{"other": "…and {count} more"}},
}

var (
	wordsMu sync.Mutex
	words   = englishWords
)

// currentWords is what to draw with now.
func currentWords() uiWords {
	wordsMu.Lock()
	defer wordsMu.Unlock()
	return words
}

// UseWords is the page handing over the words Go draws on its behalf, in the
// language it is showing. Sent at boot beside UseLanguage and again when the
// language changes; a field left empty keeps its English default, so a page
// older than a new word does not blank it.
func (a *App) UseWords(raw string) error {
	var got uiWords
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		return fmt.Errorf("words: %w", err)
	}
	next := mergeWords(englishWords, got)
	wordsMu.Lock()
	words = next
	wordsMu.Unlock()

	// The menu bar redraws on an attention event (alerts_desktop.go), and it
	// is showing the old language until one comes.
	if a != nil && a.emitter != nil {
		go a.emitter.Emit(acp.EventAttention, nil)
	}
	return nil
}

func mergeWords(base, got uiWords) uiWords {
	pick := func(dst *string, v string) {
		if strings.TrimSpace(v) != "" {
			*dst = v
		}
	}
	pick(&base.AgentAsking, got.AgentAsking)
	pick(&base.AnAgentAsking, got.AnAgentAsking)
	pick(&base.OpenToAnswer, got.OpenToAnswer)
	pick(&base.Open, got.Open)
	pick(&base.Quit, got.Quit)
	pick(&base.DiffWindow, got.DiffWindow)
	pick(&base.ShareWindow, got.ShareWindow)
	if got.MoreWaits.Forms["other"] != "" {
		base.MoreWaits = got.MoreWaits
	}
	if len(got.Messages) > 0 {
		base.Messages = got.Messages
	}
	return base
}

// say words a Message in the page's language, or in the English it carries
// when the page sent no sentence for its code.
func (w uiWords) say(m apperr.Message) string {
	sentence, ok := w.Messages[m.Code]
	if !ok || m.Code == "" {
		return m.Text
	}
	pairs := make([]string, 0, 2*len(m.Args))
	for name, value := range m.Args {
		pairs = append(pairs, "{"+name+"}", value)
	}
	return fill(sentence, pairs...)
}

// fill puts the values into a sentence's {name} slots.
func fill(sentence string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(sentence)
}

// plural picks the form a count takes and fills it in.
func (p pluralWords) plural(n int) string {
	category := "other"
	if n >= 0 && n < len(p.Counts) {
		category = p.Counts[n]
	}
	form, ok := p.Forms[category]
	if !ok {
		form = p.Forms["other"]
	}
	return fill(form, "{count}", strconv.Itoa(n))
}
