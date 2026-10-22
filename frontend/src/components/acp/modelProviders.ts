// The providers a model agent is reached through (internal/modelapi), as the
// page names them. Its own module because both the agent forms and the AI
// settings use it, and the settings show the agent form.
export const MODEL_PROVIDERS = ['anthropic', 'openai']

export const providerName = (id: string): string => {
    switch (id) {
    case 'anthropic':
        return 'Anthropic (Claude)'
    case 'openai':
        return 'OpenAI'
    default:
        return id
    }
}
