// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'
import Switch from '../../widgets/switch'
import Button from '../../widgets/buttons/button'
import {aiEnabled, setAIEnabled} from '../../aiSwitch'
import {goApp} from '../../goBridge'
import {goError} from '../../goMessages'
import {createLoader} from '../../hooks/loader'

import './appPanel.scss'
import './aiPanel.scss'

// Everything about the AI that is the person's rather than a board's: whether
// agents work at all, and the keys a model agent runs on. A key is stored by
// the Go side and never comes back; the page hears whether there is one.
//
// The agents themselves stay in the board's ⋯ menu, where they mean something
// (appSettingsDialog.tsx says why).

type Provider = {
    id: string
    keySet: boolean
    fromEnv?: boolean
    baseUrl?: string
    defaultModel?: string
}

export function isAIPanelAvailable(): boolean {
    return Boolean(goApp()?.ListAgents)
}

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

const ProviderRow = (props: {provider: Provider, onChanged: () => void}) => {
    const intl = useIntl()
    const bindings = goApp()
    const [key, setKey] = createSignal('')
    const [baseUrl, setBaseUrl] = createSignal(props.provider.baseUrl || '')
    const [busy, setBusy] = createSignal(false)
    const [error, setError] = createSignal('')
    const [saved, setSaved] = createSignal(false)

    const save = async () => {
        if (!bindings?.SetAIProviderKey) {
            return
        }
        setBusy(true)
        setError('')
        setSaved(false)
        try {
            await bindings.SetAIProviderKey(props.provider.id, key().trim(), baseUrl().trim())
            setKey('')
            setSaved(true)
            props.onChanged()
        } catch (e) {
            setError(goError(intl, e))
        } finally {
            setBusy(false)
        }
    }

    const clear = async () => {
        if (!bindings?.ClearAIProviderKey) {
            return
        }
        setBusy(true)
        setError('')
        setSaved(false)
        try {
            await bindings.ClearAIProviderKey(props.provider.id)
            setBaseUrl('')
            props.onChanged()
        } catch (e) {
            setError(goError(intl, e))
        } finally {
            setBusy(false)
        }
    }

    return (
        <div class='AIPanel__provider'>
            <div class='AIPanel__providerHead'>
                <span class='AppPanel__actionName'>{providerName(props.provider.id)}</span>
                <span class={`AIPanel__state${props.provider.keySet ? ' AIPanel__state--set' : ''}`}>
                    {props.provider.keySet ? intl.formatMessage({id: 'AI.key-set', defaultMessage: 'Key is set'}) : intl.formatMessage({id: 'AI.key-unset', defaultMessage: 'No key'})}
                </span>
            </div>
            <Show
                when={!props.provider.fromEnv}
                fallback={
                    <span class='AppPanel__actionHint'>
                        {intl.formatMessage({id: 'AI.key-from-env', defaultMessage: 'The key comes from the environment the app was started in, and is changed there.'})}
                    </span>
                }
            >
                <label class='AppPanel__field'>
                    <span class='AppPanel__fieldName'>{intl.formatMessage({id: 'AI.key', defaultMessage: 'API key'})}</span>
                    <input
                        class='AppPanel__name'
                        type='password'
                        autocomplete='off'
                        value={key()}
                        placeholder={props.provider.keySet ? intl.formatMessage({id: 'AI.key-keep', defaultMessage: 'Leave empty to keep the key that is set'}) : ''}
                        onInput={(e) => setKey(e.currentTarget.value)}
                    />
                </label>
                <label class='AppPanel__field'>
                    <span class='AppPanel__fieldName'>
                        {props.provider.id === 'openai' ? intl.formatMessage({id: 'AI.base-url-openai', defaultMessage: 'Address (optional) — for any service that speaks the OpenAI API: OpenRouter, a local model'}) : intl.formatMessage({id: 'AI.base-url', defaultMessage: 'Address (optional)'})}
                    </span>
                    <input
                        class='AppPanel__name'
                        value={baseUrl()}
                        placeholder={props.provider.id === 'openai' ? 'https://api.openai.com/v1' : 'https://api.anthropic.com'}
                        onInput={(e) => setBaseUrl(e.currentTarget.value)}
                    />
                </label>
                <div class='AppPanel__customActions'>
                    <Button
                        emphasis='primary'
                        onClick={save}
                        disabled={busy() || (!key().trim() && !props.provider.keySet)}
                    >
                        {busy() ? intl.formatMessage({id: 'AI.checking', defaultMessage: 'Checking the key…'}) : intl.formatMessage({id: 'AI.save', defaultMessage: 'Check and save'})}
                    </Button>
                    <Show when={props.provider.keySet}>
                        <Button
                            onClick={clear}
                            disabled={busy()}
                        >
                            {intl.formatMessage({id: 'AI.clear', defaultMessage: 'Remove the key'})}
                        </Button>
                    </Show>
                    <Show when={saved()}>
                        <span class='AppPanel__actionHint'>{intl.formatMessage({id: 'AI.saved', defaultMessage: 'The provider accepted the key.'})}</span>
                    </Show>
                </div>
            </Show>
            <Show when={error()}>
                <div class='AppPanel__customError'>{error()}</div>
            </Show>
        </div>
    )
}

const AIPanel = () => {
    const intl = useIntl()
    const bindings = goApp()
    const [providers, reload] = createLoader(async (): Promise<Provider[]> => {
        if (!bindings?.AIProviders) {
            return []
        }
        try {
            return (await bindings.AIProviders()) || []
        } catch {
            return []
        }
    }, [] as Provider[])

    return (
        <div class='AppPanel AIPanel'>
            {/* The one switch for everything the app does with agents
                (aiSwitch.ts), over all of them: a CLI, a key, a subscription. */}
            <section class='AppPanel__group'>
                <div class='AppPanel__action'>
                    <div class='AppPanel__actionText'>
                        <span class='AppPanel__actionName'>
                            {intl.formatMessage({id: 'Settings.ai-enabled', defaultMessage: 'Agents work on the cards'})}
                        </span>
                        <span class='AppPanel__actionHint'>
                            {intl.formatMessage({
                                id: 'Settings.ai-enabled-hint',
                                defaultMessage: 'When this is off, the app is a plain kanban board: no agents and no agent settings, and cards moved between columns start nothing. The routes and agents already set up are kept and come back when this is turned on again.',
                            })}
                        </span>
                    </div>
                    <Switch
                        isOn={aiEnabled()}
                        onChanged={setAIEnabled}
                    />
                </div>
            </section>

            <Show when={aiEnabled()}>
                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'AI.keys', defaultMessage: 'Model keys'})}
                    </h4>
                    <span class='AppPanel__actionHint'>
                        {intl.formatMessage({
                            id: 'AI.keys-hint',
                            defaultMessage: 'An agent of the kind «Model by API key» calls the provider with your key. The key is kept by the app and is not shown again. A key is checked with the provider before it is saved.',
                        })}
                    </span>
                    <For each={providers()}>
                        {(provider) => (
                            <ProviderRow
                                provider={provider}
                                onChanged={reload}
                            />
                        )}
                    </For>
                </section>
            </Show>
        </div>
    )
}

export default AIPanel
