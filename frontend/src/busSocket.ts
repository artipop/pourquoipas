/* eslint-disable new-cap -- the Go side's methods and the runtime's are capitalised */
import {Events} from '@wailsio/runtime'

import {goApp} from './goBridge'
import type {IAppWindow} from './types'

// A WebSocket the page asks Go to hold, for a desktop window: the page there is
// on wails://, where a WebSocket has nowhere to go. Go dials the socket itself
// and the two ends meet on the Wails bus (sockettunnel.go) — frames arrive as
// the event "socket:<id>", frames leave as a binding call. It answers the part
// of WebSocket that wsclient.ts and agentEvents.ts use, so neither knows which
// one it holds.

type TunnelMessage = {data?: string, closed?: boolean, code?: number}

let counter = 0

export class BusSocket {
    onopen: (() => void) | null = null
    onmessage: ((e: {data: string}) => void) | null = null
    onclose: ((e: {code: number, reason: string}) => void) | null = null
    onerror: ((e: unknown) => void) | null = null
    readyState = 0

    private readonly id: string
    private off?: () => void

    // Frames leave in the order they were sent. Each is its own binding call,
    // and two calls in flight are two goroutines on the Go side: the board's
    // socket refused a subscription that overtook the login before it.
    private sending: Promise<unknown> = Promise.resolve()

    constructor(path: string) {
        counter++
        this.id = `${Date.now().toString(36)}-${counter}-${Math.random().toString(36).slice(2)}`

        // Subscribed before the socket is asked for, so its first frame finds
        // somebody listening.
        this.off = Events.On(`socket:${this.id}`, (event) => this.receive(event.data as TunnelMessage))

        // A page that goes away without closing leaves Go reading for nobody.
        window.addEventListener('pagehide', this.close)

        goApp()!.OpenSocket(this.id, path).then(() => {
            if (this.readyState !== 0) {
                return
            }
            this.readyState = 1
            this.onopen?.()
        }).catch((err: unknown) => {
            this.onerror?.(err)
            this.finish(1006)
        })
    }

    send(data: string): void {
        this.sending = this.sending.
            then(() => goApp()!.SendSocket(this.id, data)).
            catch((err: unknown) => this.onerror?.(err))
    }

    close = (): void => {
        if (this.readyState === 3) {
            return
        }
        goApp()?.CloseSocket(this.id)
        this.finish(1000)
    }

    private receive(message: TunnelMessage): void {
        if (message.closed) {
            this.finish(message.code ?? 1006)
            return
        }
        this.onmessage?.({data: message.data ?? ''})
    }

    private finish(code: number): void {
        if (this.readyState === 3) {
            return
        }
        this.readyState = 3
        this.off?.()
        this.off = undefined
        window.removeEventListener('pagehide', this.close)
        this.onclose?.({code, reason: ''})
    }
}

// pageOnWails says the page is in a desktop window, loaded from wails://; set
// by the bootstrap (proxy.go).
export function pageOnWails(): boolean {
    return Boolean((window as unknown as IAppWindow).pageOnWails) && Boolean(goApp())
}

// openSocket opens path on this page's own origin, through Go when the page
// cannot open it itself.
export function openSocket(path: string, url: () => string): WebSocket {
    return (pageOnWails() ? new BusSocket(path) : new WebSocket(url())) as unknown as WebSocket
}

// sendableForm is a form as a page on wails:// can send it. WebKit hands a
// custom scheme's handler no body at all when the body is a FormData or a Blob
// — a file arrived at the board as an empty multipart, "NextPart: EOF" — while
// bytes go through whole (3 MB measured). So the form is encoded here, with
// the boundary the encoding chose.
export async function sendableForm(form: FormData): Promise<{body: BodyInit, contentType?: string}> {
    if (!pageOnWails()) {
        return {body: form}
    }
    const encoded = new Response(form)
    return {body: await encoded.arrayBuffer(), contentType: encoded.headers.get('Content-Type') ?? undefined}
}
