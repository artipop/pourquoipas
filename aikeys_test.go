package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artipop/pourquoipas/internal/modelapi"
	"github.com/artipop/pourquoipas/internal/secrets"
)

// A key goes in, is checked against the provider, and never comes back out:
// the page hears that there is one, and the agent gets it when it runs.
func TestAProvidersKeyIsCheckedStoredAndNeverShown(t *testing.T) {
	var seenAuth string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if seenAuth != "Bearer sk-good-1234" {
			http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"gpt-test","object":"model","created":1,"owned_by":"x"}]}`)
	}))
	defer provider.Close()

	app := &App{aiKeys: newAIKeys(secrets.NewFileStore(filepath.Join(t.TempDir(), "secrets.json")))}
	ctx := context.Background()

	if _, err := app.SetAIProviderKey(ctx, modelapi.ProviderOpenAI, "sk-bad", provider.URL+"/v1"); err == nil {
		t.Fatal("a key the provider refused was stored")
	}
	if _, err := app.SetAIProviderKey(ctx, modelapi.ProviderOpenAI, "sk-good-1234", provider.URL+"/v1"); err != nil {
		t.Fatal(err)
	}

	listed, err := app.AIProviders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(listed)
	if strings.Contains(string(raw), "sk-good-1234") {
		t.Fatalf("the key reached the page: %s", raw)
	}
	var openai AIProvider
	for _, p := range listed {
		if p.ID == modelapi.ProviderOpenAI {
			openai = p
		}
	}
	if !openai.KeySet || openai.BaseURL != provider.URL+"/v1" || openai.KeyHint != "…1234" {
		t.Errorf("provider = %+v", openai)
	}

	conn, err := app.aiKeys.connection("", modelapi.ProviderOpenAI)
	if err != nil || conn.Key != "sk-good-1234" {
		t.Errorf("the agent's connection = %+v, %v", conn, err)
	}
	models, err := app.AIModels(ctx, modelapi.ProviderOpenAI)
	if err != nil || len(models) != 1 || models[0].ID != "gpt-test" {
		t.Errorf("models = %v, %v", models, err)
	}

	if err := app.ClearAIProviderKey(ctx, modelapi.ProviderOpenAI); err != nil {
		t.Fatal(err)
	}
	if _, err := app.aiKeys.connection("", modelapi.ProviderOpenAI); err != modelapi.ErrNoKey {
		t.Errorf("a cleared key still connects: %v", err)
	}
}

// On a server two people have two keys; the name says whose.
func TestAKeyIsStoredUnderItsOwner(t *testing.T) {
	if got := aiSecretName("", "anthropic", ""); got != "ai/anthropic" {
		t.Errorf("desktop name = %q", got)
	}
	if got := aiSecretName("u1", "openai", "url"); got != "ai/u1/openai.url" {
		t.Errorf("server name = %q", got)
	}
}
