package boardmodel

import (
	"strings"
	"testing"

	"github.com/artipop/xciii/server/model"
)

func personSchema() model.PropSchema {
	return model.PropSchema{
		"prop-assignee":  {ID: "prop-assignee", Name: "Assignee", Type: "person"},
		"prop-reviewers": {ID: "prop-reviewers", Name: "Reviewers", Type: "multiPerson"},
		"prop-project":   {ID: "prop-project", Name: "repo_path", Type: "text"},
	}
}

func TestPersonPropertiesResolveToUsernames(t *testing.T) {
	props := map[string]any{
		"prop-assignee":  "uid-claude",
		"prop-reviewers": []any{"uid-codex", "uid-ghost"},
		"prop-project":   "/tmp/project",
	}
	lookups := 0
	resolver := NewResolver(func(userID string) string {
		lookups++
		return map[string]string{"uid-claude": "claude", "uid-codex": "codex"}[userID]
	})

	names, ids := PersonNames(props, personSchema(), resolver)
	// The ids go out even for a user the resolver cannot read: they are what the
	// card stores and what an agent is matched by.
	if len(ids) != 3 {
		t.Errorf("person ids = %v, want one per value including the unknown one", ids)
	}
	if len(names) != 2 {
		t.Fatalf("person names = %v, want claude and codex", names)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "claude") || !strings.Contains(joined, "codex") {
		t.Errorf("person names = %v", names)
	}
	// An unknown id contributes no name — it must not be mistaken for one.
	if strings.Contains(joined, "uid-ghost") {
		t.Errorf("unknown user leaked into person names: %v", names)
	}

	// The same values are resolved once, whoever asks: BlockChanged runs on the
	// notify worker and this is a DB read per person value.
	block := &model.Block{ID: "card1", Fields: map[string]any{"properties": props}}
	parsed := NamedProperties(block, personSchema(), resolver)
	if parsed["assignee"] != "claude" {
		t.Errorf("assignee prop = %q, want claude", parsed["assignee"])
	}
	if parsed["repo_path"] != "/tmp/project" {
		t.Errorf("unrelated props broken: %v", parsed)
	}
	if lookups != 3 {
		t.Errorf("lookups = %d, want 3 (one per distinct user id)", lookups)
	}
}

func TestNamedPropertiesSurvivesUnresolvableUsers(t *testing.T) {
	// No app (nil lookup) is the case in a browser/plugin build and in tests:
	// person values stay raw ids, and the rest of the map must still arrive.
	props := map[string]any{"prop-assignee": "uid-claude", "prop-project": "/tmp/project"}
	block := &model.Block{ID: "card1", Fields: map[string]any{"properties": props}}
	resolver := NewResolver(nil)

	parsed := NamedProperties(block, personSchema(), resolver)
	if parsed["repo_path"] != "/tmp/project" {
		t.Fatalf("props lost when a user cannot be resolved: %v", parsed)
	}
	if parsed["assignee"] != "uid-claude" {
		t.Errorf("assignee prop = %q, want the raw id", parsed["assignee"])
	}
	if names, _ := PersonNames(props, personSchema(), resolver); len(names) != 0 {
		t.Errorf("person names = %v, want none", names)
	}
}
