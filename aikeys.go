package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/artipop/pourquoipas/internal/acp"
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
	KeySet  bool `json:"keySet"`
	FromEnv bool `json:"fromEnv,omitempty"`
	// KeyHint is the key's last characters, so a person can tell which key is
	// set without the key coming back.
	KeyHint      string `json:"keyHint,omitempty"`
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
		key := a.aiKeys.get(name)
		out = append(out, AIProvider{
			ID:           id,
			KeySet:       key != "",
			KeyHint:      keyHint(key),
			FromEnv:      envErr == nil,
			BaseURL:      a.aiKeys.get(aiSecretName(person, id, "url")),
			DefaultModel: modelapi.DefaultModel(id),
		})
	}
	return out, nil
}

// AIKeySaved is what saving a key did: the agent it made, when it made one.
type AIKeySaved struct {
	Agent string `json:"agent,omitempty"`
}

// SetAIProviderKey stores the person's key for a provider, and the address
// for one that is not the vendor's. The key is checked first by asking the
// provider for its models: a key it refuses is not stored. An empty key keeps
// the one stored and changes only the address.
//
// A key nobody can use is a key that looks broken: everything that runs a
// model picks an agent, not a key. So the first key of a provider also makes
// the person an agent on it, on a model the provider just listed, which the
// inbox sorting and a card's assignee field offer at once.
func (a *App) SetAIProviderKey(ctx context.Context, provider, key, baseURL string) (AIKeySaved, error) {
	person, err := personOf(ctx, a.owners)
	if err != nil {
		return AIKeySaved{}, err
	}
	if !knownAIProvider(provider) {
		return AIKeySaved{}, apperr.New("ai.unknown-provider", "Unknown provider {provider}", "provider", provider)
	}
	key, baseURL = strings.TrimSpace(key), strings.TrimSpace(baseURL)
	if key == "" {
		key = a.aiKeys.get(aiSecretName(person, provider, ""))
	}
	if key == "" {
		return AIKeySaved{}, apperr.New("ai.key-empty", "The key is empty")
	}
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	models, err := modelapi.Models(checkCtx, modelapi.Connection{Provider: provider, Key: key, BaseURL: baseURL})
	if err != nil {
		return AIKeySaved{}, apperr.Wrap(err, "ai.key-refused", "The provider did not accept the key: {reason}", "reason", err.Error())
	}
	if err := a.aiKeys.vault.Set(aiSecretName(person, provider, ""), key); err != nil {
		return AIKeySaved{}, err
	}
	if baseURL == "" {
		err = deleteSecret(a.aiKeys.vault, aiSecretName(person, provider, "url"))
	} else {
		err = a.aiKeys.vault.Set(aiSecretName(person, provider, "url"), baseURL)
	}
	if err != nil {
		return AIKeySaved{}, err
	}
	return AIKeySaved{Agent: a.ensureModelAgent(person, provider, models)}, nil
}

// ensureModelAgent makes the person an agent on the provider unless they have
// one, and returns its name; "" when nothing was made. A failure is logged and
// not returned: the key is saved either way, and an agent can be added by hand.
func (a *App) ensureModelAgent(person, provider string, models []modelapi.Model) string {
	if a.mgr == nil {
		return ""
	}
	taken := map[string]bool{}
	for _, entry := range a.mgr.Agents() {
		taken[strings.ToLower(entry.Name)] = true
		if entry.IsModel() && entry.Provider == provider && (entry.OwnerID == "" || entry.OwnerID == person) {
			return ""
		}
	}
	name := providerAgentName(provider)
	for i := 2; taken[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s %d", providerAgentName(provider), i)
	}
	saved, err := a.mgr.AddAgent(acp.AgentEntry{
		Name:     name,
		Kind:     acp.AgentKindModel,
		Provider: provider,
		Model:    modelapi.SuggestModel(provider, models),
		OwnerID:  person,
	})
	if err != nil {
		log.Printf("ai: no agent was made for the %s key: %v", provider, err)
		return ""
	}
	return saved.Name
}

// providerAgentName is what the agent a key makes is called: the name people
// know the models by.
func providerAgentName(provider string) string {
	switch provider {
	case modelapi.ProviderAnthropic:
		return "Claude"
	case modelapi.ProviderOpenAI:
		return "OpenAI"
	}
	return provider
}

// keyHint is a key's last four characters behind an ellipsis.
func keyHint(key string) string {
	r := []rune(strings.TrimSpace(key))
	if len(r) < 8 {
		return ""
	}
	return "…" + string(r[len(r)-4:])
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
