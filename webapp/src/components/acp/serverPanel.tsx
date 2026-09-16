// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Show, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'
import Button from '../../widgets/buttons/button'

import {goApp} from '../../goBridge'

import './serverPanel.scss'

// Working somebody else's board from this desktop.
//
// The board moves and the agents do not: terminals, folders and the CLIs stay
// on this machine, and the cards live on the server. Which is why the login is
// here rather than on a page — the Go side needs the session for its own board
// client, and the page cannot be pointed anywhere until there is an address.
//
// It is not the tailnet, and the panel says so: that one answers where the
// board can be reached *from*, this one answers *whose* board it is.

type AttachState = {
    url: string
    username: string
    running: string
    error?: string
}

export function isServerAttachAvailable(): boolean {
    return Boolean(goApp()?.GetServerAttachment)
}

const ServerPanel = () => {
    const intl = useIntl()
    const bindings = goApp()

    const [state, setState] = createSignal<AttachState | null>(null)
    const [url, setUrl] = createSignal('')
    const [username, setUsername] = createSignal('')
    const [password, setPassword] = createSignal('')
    const [busy, setBusy] = createSignal(false)
    const [error, setError] = createSignal('')

    const load = async () => {
        if (!bindings?.GetServerAttachment) {
            return
        }
        try {
            setState(JSON.parse(await bindings.GetServerAttachment()) as AttachState)
        } catch (e) {
            setError(String(e))
        }
    }

    // Not awaited: onSettled reads a returned promise as a cleanup value.
    onSettled(() => {
        void load()
    })

    const attached = () => Boolean(state()?.url)

    // The address is decided when the app is built up — both the board the page
    // is served and the board the agents work against — so a switch that
    // pretended to take effect now would be a switch that lies.
    const restartOwed = () => Boolean(state()) && state()!.url !== state()!.running

    const attach = async () => {
        if (!bindings?.AttachServer) {
            return
        }
        setBusy(true)
        setError('')
        try {
            const next = JSON.parse(await bindings.AttachServer(JSON.stringify({
                url: url(),
                username: username(),
                password: password(),
            }))) as AttachState
            setState(next)
            setPassword('')
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const detach = async () => {
        if (!bindings?.DetachServer) {
            return
        }
        setBusy(true)
        setError('')
        try {
            setState(JSON.parse(await bindings.DetachServer()) as AttachState)
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    return (
        <div class='ServerPanel'>
            <div class='ServerPanel__subtitle'>
                {intl.formatMessage({
                    id: 'Server.subtitle',
                    defaultMessage: 'Work a board that lives on a server. The cards are there; the agents, the terminals and the folders stay on this machine.',
                })}
            </div>

            <Show
                when={attached()}
                fallback={
                    <div class='ServerPanel__form'>
                        <label class='ServerPanel__field'>
                            <span>{intl.formatMessage({id: 'Server.address', defaultMessage: 'Server address'})}</span>
                            <input
                                type='text'
                                placeholder='board.example.com'
                                value={url()}
                                onInput={(e) => setUrl(e.currentTarget.value)}
                            />
                        </label>
                        <label class='ServerPanel__field'>
                            <span>{intl.formatMessage({id: 'Server.username', defaultMessage: 'Username'})}</span>
                            <input
                                type='text'
                                value={username()}
                                onInput={(e) => setUsername(e.currentTarget.value)}
                            />
                        </label>
                        <label class='ServerPanel__field'>
                            <span>{intl.formatMessage({id: 'Server.password', defaultMessage: 'Password'})}</span>
                            <input
                                type='password'
                                value={password()}
                                onInput={(e) => setPassword(e.currentTarget.value)}
                            />
                        </label>
                        <Button
                            filled={true}
                            submit={true}
                            onClick={attach}
                        >
                            {busy() ? intl.formatMessage({id: 'Server.connecting', defaultMessage: 'Connecting…'}) : intl.formatMessage({id: 'Server.connect', defaultMessage: 'Connect'})}
                        </Button>
                        <div class='ServerPanel__hint'>
                            {intl.formatMessage({
                                id: 'Server.password-hint',
                                defaultMessage: 'The password is not kept. What is stored is the session it buys.',
                            })}
                        </div>
                    </div>
                }
            >
                <div class='ServerPanel__attached'>
                    <div class='ServerPanel__where'>{state()!.url}</div>
                    <div class='ServerPanel__hint'>
                        {intl.formatMessage(
                            {id: 'Server.as', defaultMessage: 'Signed in as {name}'},
                            {name: state()!.username},
                        )}
                    </div>
                    <Button onClick={detach}>
                        {intl.formatMessage({id: 'Server.disconnect', defaultMessage: 'Work this machine\'s own board'})}
                    </Button>
                </div>
            </Show>

            <Show when={restartOwed()}>
                <div class='ServerPanel__restart'>
                    {intl.formatMessage({
                        id: 'Server.restart',
                        defaultMessage: 'Restart the app: which board it works is decided when it starts.',
                    })}
                </div>
            </Show>

            <Show when={error()}>
                <div class='ServerPanel__error'>{error()}</div>
            </Show>
        </div>
    )
}

export default ServerPanel
