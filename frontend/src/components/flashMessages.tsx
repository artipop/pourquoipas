import {Show, createEffect, createSignal} from 'solid-js'
import type {Component} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {useIntl} from '../intl'

import CompassIcon from '../widgets/icons/compassIcon'

import './flashMessages.scss'

export type FlashMessage = {
    content: JSX.Element
    severity: 'low' | 'normal' | 'high'

    // A notice is a sentence, not a confirmation: it takes the corner-card
    // shape of the agent notifications rather than the centered pill, gets a
    // close button of its own, and usually asks for more time than the
    // mount-wide default.
    notice?: boolean
    milliseconds?: number
}

// A fresh request also restarts the timer when the same message is sent again.
// Release JSX content when the last renderer goes away.
const [request, setRequest] = createSignal<{message: FlashMessage} | null>(null, {
    unobserved: () => setRequest(null),
})

export function sendFlashMessage(message: FlashMessage): void {
    setRequest({message})
}

type Props = {
    milliseconds: number
}

export const FlashMessages: Component<Props> = (props) => {
    const intl = useIntl()
    const [message, setMessage] = createSignal<FlashMessage|null>(null)
    const [fadeOut, setFadeOut] = createSignal(false)
    let handleClick: () => void = () => undefined

    // Only new requests are events: remounting must not replay the previous
    // message. Solid owns the subscription and cancels the current timer both
    // on the next request and on disposal.
    createEffect(request, (sent) => {
        if (!sent) {
            return undefined
        }
        setMessage(sent.message)
        setFadeOut(false)
        let timeoutId: ReturnType<typeof setTimeout>
        const fade = () => {
            clearTimeout(timeoutId)
            setFadeOut(true)
            timeoutId = setTimeout(() => {
                setMessage(null)
                setFadeOut(false)
            }, 200)
        }
        handleClick = fade
        timeoutId = setTimeout(fade, Math.max(0, (sent.message.milliseconds ?? props.milliseconds) - 200))
        return () => clearTimeout(timeoutId)
    }, {defer: true})

    // The message may already be gone. rc.8 could re-run these bindings during
    // an unrelated async revalidation; keep the nullable reads even though
    // rc.18 fixes that scheduling issue (docs/solid2.md).
    return (
        <Show when={message()}>
            <div
                class={'FlashMessages ' + (message()?.severity ?? '') + (message()?.notice ? ' FlashMessages--notice' : '') + (fadeOut() ? ' flashOut' : ' flashIn')}
                onClick={handleClick}
            >
                {message()?.content}
                <Show when={message()?.notice}>
                    <button
                        type='button'
                        class='FlashMessages__close'
                        title={intl.formatMessage({id: 'Modal.close', defaultMessage: 'Close'})}
                        aria-label={intl.formatMessage({id: 'Modal.close', defaultMessage: 'Close'})}
                        onClick={handleClick}
                    >
                        <CompassIcon icon='close'/>
                    </button>
                </Show>
            </div>
        </Show>
    )
}
