package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/artipop/pourquoipas/internal/apperr"
	"github.com/artipop/pourquoipas/internal/modelapi"
	"github.com/artipop/pourquoipas/internal/secrets"
)

// A model agent runs on its owner's key (docs/web-ai-plan.md). The key is a
// credential this app presents to somebody else, so it lives in the secrets
// store — the keychain on a desktop, the store's file on a server — under the
// person's name, and the page only ever hears whether there is one.
//
// On a desktop the person is the one at it, and their key is the install's:
// `ai/anthropic`. On a server it is `ai/<userID>/anthropic`. The environment is
// read first, as for every secret (PP_AI_ANTHROPIC), which is
// how a desktop in development is given a key without typing it.

var errNobodySignedIn = apperr.New("ai.nobody", "Sign in first")

type aiKeys struct {
	vault secrets.Store
	env   secrets.Env
}

func newAIKeys(vault secrets.Store) *aiKeys {
	return &aiKeys{vault: vault, env: secrets.Env{Prefix: secretEnvPrefix}}
}

// secretEnvPrefix is openVault's: the two must agree on where an environment
// secret is looked for.
const secretEnvPrefix = "PP_"

func aiSecretName(person, provider, part string) string {
	name := "ai/"
	if person != "" {
		name += person + "/"
	}
	name += provider
	if part != "" {
		name += "." + part
	}
	return name
}

func (k *aiKeys) get(name string) string {
	if k == nil || k.vault == nil {
		return ""
	}
	value, err := k.vault.Get(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// connection is acp.ModelConnections: where an agent's requests go and with
// whose key.
func (k *aiKeys) connection(owner, provider string) (modelapi.Connection, error) {
	conn := modelapi.Connection{
		Provider: provider,
		Key:      k.get(aiSecretName(owner, provider, "")),
		BaseURL:  k.get(aiSecretName(owner, provider, "url")),
	}
	if conn.Key == "" {
		return conn, modelapi.ErrNoKey
	}
	return conn, nil
}

// AIProvider is one provider as the settings panel shows it: never the key.
type AIProvider struct {
	ID string `json:"id"`
	// KeySet says there is a key; FromEnv that it came from the environment,
	// where the page can neither change nor remove it.
	KeySet       bool   `json:"keySet"`
	FromEnv      bool   `json:"fromEnv,omitempty"`
	BaseURL      string `json:"baseUrl,omitempty"`
	DefaultModel string `json:"defaultModel,omitempty"`
}

// AIProviders lists the providers and whether the signed-in person has a key
// for each.
func (a *App) AIProviders(ctx context.Context) ([]AIProvider, error) {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return nil, err
	}
	out := make([]AIProvider, 0, len(modelapi.Providers))
	for _, id := range modelapi.Providers {
		name := aiSecretName(person, id, "")
		_, envErr := a.aiKeys.env.Get(name)
		out = append(out, AIProvider{
			ID:           id,
			KeySet:       a.aiKeys.get(name) != "",
			FromEnv:      envErr == nil,
			BaseURL:      a.aiKeys.get(aiSecretName(person, id, "url")),
			DefaultModel: modelapi.DefaultModel(id),
		})
	}
	return out, nil
}

// SetAIProviderKey stores the person's key for a provider, and the address
// for one that is not the vendor's. The key is checked first by asking the
// provider for its models: a key it refuses is not stored. An empty key keeps
// the one stored and changes only the address.
func (a *App) SetAIProviderKey(ctx context.Context, provider, key, baseURL string) error {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return err
	}
	if !knownAIProvider(provider) {
		return apperr.New("ai.unknown-provider", "Unknown provider {provider}", "provider", provider)
	}
	key, baseURL = strings.TrimSpace(key), strings.TrimSpace(baseURL)
	if key == "" {
		key = a.aiKeys.get(aiSecretName(person, provider, ""))
	}
	if key == "" {
		return apperr.New("ai.key-empty", "The key is empty")
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if _, err := modelapi.Models(checkCtx, modelapi.Connection{Provider: provider, Key: key, BaseURL: baseURL}); err != nil {
		return apperr.Wrap(err, "ai.key-refused", "The provider did not accept the key: {reason}", "reason", err.Error())
	}
	if err := a.aiKeys.vault.Set(aiSecretName(person, provider, ""), key); err != nil {
		return err
	}
	if baseURL == "" {
		return deleteSecret(a.aiKeys.vault, aiSecretName(person, provider, "url"))
	}
	return a.aiKeys.vault.Set(aiSecretName(person, provider, "url"), baseURL)
}

// ClearAIProviderKey forgets the person's key and address for a provider.
func (a *App) ClearAIProviderKey(ctx context.Context, provider string) error {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return err
	}
	if err := deleteSecret(a.aiKeys.vault, aiSecretName(person, provider, "")); err != nil {
		return err
	}
	return deleteSecret(a.aiKeys.vault, aiSecretName(person, provider, "url"))
}

// AIModels is what the provider offers the person's key, for the agent form.
func (a *App) AIModels(ctx context.Context, provider string) ([]modelapi.Model, error) {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return nil, err
	}
	conn, err := a.aiKeys.connection(person, provider)
	if err != nil {
		return nil, apperr.New("ai.no-key", "There is no API key for {provider}: add one in Settings → AI", "provider", provider)
	}
	listCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return modelapi.Models(listCtx, conn)
}

func knownAIProvider(p string) bool {
	for _, known := range modelapi.Providers {
		if p == known {
			return true
		}
	}
	return false
}

// deleteSecret removes what the store has, whichever store of the chain has
// it; a secret that was never there is already deleted.
func deleteSecret(store secrets.Store, name string) error {
	err := store.Delete(name)
	if err == nil || errors.Is(err, secrets.ErrNotFound) {
		return nil
	}
	return err
}
