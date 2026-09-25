// Package apperr is how Go says something a person reads without saying it in
// a language: a code the page has a sentence for, the values that go into the
// sentence, and an English text for anybody who does not know the code — a
// page older than the code, a log, a script calling the API.
//
// The words are the page's (frontend/i18n, id "Go.<code>"); Go keeps no
// catalogue. docs/i18n.md, "What Go says".
package apperr

import (
	"encoding/json"
	"errors"
	"strings"
)

// Message is one thing said: a status, a reason, a decision. English is a
// template with the same {name} slots the page's message has.
type Message struct {
	Code string            `json:"code"`
	Args map[string]string `json:"args,omitempty"`
	// Text is the English, filled in. It is what a log reads and what a page
	// that does not know Code shows.
	Text string `json:"text"`
}

// Say makes a Message. args are name/value pairs, in the order the English
// names them.
func Say(code, english string, args ...string) Message {
	m := Message{Code: code, Text: english}
	if len(args) > 0 {
		m.Args = make(map[string]string, len(args)/2)
		pairs := make([]string, 0, len(args))
		for i := 0; i+1 < len(args); i += 2 {
			m.Args[args[i]] = args[i+1]
			pairs = append(pairs, "{"+args[i]+"}", args[i+1])
		}
		m.Text = strings.NewReplacer(pairs...).Replace(english)
	}
	return m
}

// String is the English.
func (m Message) String() string { return m.Text }

// IsZero says nothing was said.
func (m Message) IsZero() bool { return m.Code == "" && m.Text == "" }

// Plain is a Message with no code: English the page shows as it is. For what
// a person is not expected to act on — a process that failed, a service that
// answered badly — where the words come from somewhere else anyway.
func Plain(text string) Message { return Message{Text: text} }

// Error is a Message that is also an error. A Go method of App that returns
// one hands the page its code and values (MarshalError); wrapped in context by
// fmt.Errorf("...: %w") it still does, and the context is lost on the page,
// which is the point: the sentence is the page's.
type Error struct {
	Message
	cause error
	// quiet leaves the cause out of the text: it is a sentinel for errors.Is
	// rather than something that happened (Kind).
	quiet bool
}

// New is an error a person can act on.
func New(code, english string, args ...string) *Error {
	return &Error{Message: Say(code, english, args...)}
}

// Wrap is New with the error that caused it, which the English carries after a
// colon and the page does not.
func Wrap(err error, code, english string, args ...string) *Error {
	return &Error{Message: Say(code, english, args...), cause: err}
}

// Kind is New for one case of a sentinel: errors.Is(err, sentinel) holds, and
// the sentinel's own text is not repeated.
func Kind(sentinel error, code, english string, args ...string) *Error {
	return &Error{Message: Say(code, english, args...), cause: sentinel, quiet: true}
}

// MessageOf is what an error says to a person: the Message of the first Error
// in its chain, or its text as it is.
func MessageOf(err error) Message {
	if err == nil {
		return Message{}
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	return Plain(err.Error())
}

func (e *Error) Error() string {
	if e.cause != nil && !e.quiet {
		return e.Text + ": " + e.cause.Error()
	}
	return e.Text
}

func (e *Error) Unwrap() error { return e.cause }

// Is makes two errors with one code the same error, so a sentinel made with
// New can be matched with errors.Is after it has been re-made with values.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Code != "" && t.Code == e.Code
}

// wire is what the page receives as a binding error's cause.
type wire struct {
	Code    string            `json:"code,omitempty"`
	Args    map[string]string `json:"args,omitempty"`
	Message string            `json:"message"`
}

// MarshalError is application.Options.MarshalError: the first Error in the
// chain becomes the cause the page reads, with the whole chain's text as the
// message. Nil for any other error, which leaves Wails to its default.
func MarshalError(err error) []byte {
	var e *Error
	if !errors.As(err, &e) {
		return nil
	}
	data, jerr := json.Marshal(wire{Code: e.Code, Args: e.Args, Message: err.Error()})
	if jerr != nil {
		return nil
	}
	return data
}

// Columns is how a Message is kept in a table: the English, the code and the
// values as a JSON object, the last two NULL when there are none. Absence is
// NULL where a key looks (docs/db-schema-review.md).
func (m Message) Columns() (text string, code, args any) {
	if m.Code != "" {
		code = m.Code
	}
	if len(m.Args) > 0 {
		data, _ := json.Marshal(m.Args) // a map of strings always marshals
		args = string(data)
	}
	return m.Text, code, args
}

// FromColumns reads back what Columns wrote, with NULLs read as "".
func FromColumns(text, code, args string) (Message, error) {
	m := Message{Code: code, Text: text}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &m.Args); err != nil {
			return Message{}, err
		}
	}
	return m, nil
}
