package inboxsort

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// What the agent is told. English, like everything said to an agent; what a
// person wrote — titles, descriptions, card text — passes through as data, and
// the reasons are asked for in the card's own language because a person reads
// them.

// promptCard is a card as the sorting prompt shows it.
type promptCard struct {
	Key, Title, Author, Link, Text string
}

func sortPrompt(boards []*boardView, cards []promptCard) string {
	var b strings.Builder
	b.WriteString(`You sort a person's inbox. Each card below arrived in the inbox. Each board is where the person keeps cards about one area of their work or life.

For every card, choose the one board it clearly belongs on. When no board fits, or you are not sure, leave the card in the inbox: a card left there is read by the person, a card put on the wrong board is lost.

Do not use any tools and do not change anything. Decide from what is written below.

Boards:
`)
	for i, v := range boards {
		fmt.Fprintf(&b, "\n[b%d] %s\n", i+1, oneLine(v.Title))
		if d := strings.TrimSpace(v.Description); d != "" {
			fmt.Fprintf(&b, "Description: %s\n", oneLine(d))
		}
		if v.summary != "" {
			fmt.Fprintf(&b, "What its cards are about: %s\n", oneLine(v.summary))
		}
		if titles := cardTitles(v.cards, recentTitles); len(titles) > 0 {
			fmt.Fprintf(&b, "Recent cards: %s\n", strings.Join(titles, "; "))
		}
		if strings.TrimSpace(v.Description) == "" && v.summary == "" && len(v.cards) == 0 {
			b.WriteString("(nothing is known about this board but its title)\n")
		}
	}
	b.WriteString("\nCards:\n")
	for _, c := range cards {
		fmt.Fprintf(&b, "\n[%s] %s\n", c.Key, oneLine(c.Title))
		if c.Author != "" {
			fmt.Fprintf(&b, "Brought by: %s\n", c.Author)
		}
		if c.Link != "" {
			fmt.Fprintf(&b, "Link: %s\n", c.Link)
		}
		if text := clip(strings.TrimSpace(c.Text), cardTextLimit); text != "" {
			fmt.Fprintf(&b, "Text:\n%s\n", text)
		}
	}
	b.WriteString(`
Answer with a JSON array and nothing else, one object per card:
[{"card":"c1","board":"b2","reason":"..."},{"card":"c2","board":"","reason":"..."}]

"board" is the key of a board above, or "" to leave the card in the inbox. "reason" is one short sentence for the person saying why, written in the language the card is written in.
`)
	return b.String()
}

func summaryPrompt(boards []*boardView) string {
	var b strings.Builder
	b.WriteString(`Below are boards a person keeps cards on, each with the titles of its cards, newest first.

For each board, write what its cards are about in one to three sentences: the topics, the kinds of tasks, the places, people or projects that recur. The text is used later to decide which board a new card belongs on, so be concrete and say what sets this board apart from the others. Do not describe how the board is laid out. Write in the language the cards are written in.

Do not use any tools and do not change anything.
`)
	for i, v := range boards {
		fmt.Fprintf(&b, "\n[b%d] %s\n", i+1, oneLine(v.Title))
		if d := strings.TrimSpace(v.Description); d != "" {
			fmt.Fprintf(&b, "Description: %s\n", oneLine(d))
		}
		b.WriteString("Cards:\n")
		for _, t := range cardTitles(v.cards, summaryTitles) {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}
	b.WriteString(`
Answer with a JSON object and nothing else, mapping each board key to its summary:
{"b1":"...","b2":"..."}
`)
	return b.String()
}

func cardTitles(cards []CardInfo, limit int) []string {
	out := make([]string, 0, min(len(cards), limit))
	for _, c := range cards {
		if len(out) == limit {
			break
		}
		if t := oneLine(c.Title); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func clip(s string, limit int) string {
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit]) + "…"
}

// decodeJSON reads the first JSON value that opens with `open` out of an
// agent's answer. An agent asked for JSON and nothing else still wraps it in a
// fence or a sentence often enough that reading from the first bracket is the
// difference between a sorter that works and one that fails every other turn.
func decodeJSON(answer string, open byte, v any) error {
	for i := 0; i < len(answer); i++ {
		if answer[i] != open {
			continue
		}
		// Into a raw value first, so an attempt that fails half-way leaves
		// nothing behind in v for the next one to be mixed with.
		var raw json.RawMessage
		if err := json.NewDecoder(strings.NewReader(answer[i:])).Decode(&raw); err != nil {
			continue
		}
		if err := json.Unmarshal(raw, v); err == nil {
			return nil
		}
	}
	if strings.TrimSpace(answer) == "" {
		return errors.New("the agent answered nothing")
	}
	return fmt.Errorf("the answer has no JSON: %s", clip(oneLine(answer), 200))
}
