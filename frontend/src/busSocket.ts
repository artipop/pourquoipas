/* eslint-disable new-cap -- the Go side's methods and the runtime's are capitalised */
import {Events} from '@wailsio/runtime'

import {goApp} from './goBridge'
import type {IAppWindow} from './types'

// A WebSocket held by Go for a page on wails://, where the page cannot open
// one (sockettunnel.go).

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

    // Each send is a binding call, and two in flight can arrive out of order.
    private sending: Promise<unknown> = Promise.resolve()

    constructor(path: string) {
        counter++
        this.id = `${Date.now().toString(36)}-${counter}-${Math.random().toString(36).slice(2)}`

        this.off = Events.On(`socket:${this.id}`, (event) => this.receive(event.data as TunnelMessage))

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

export function pageOnWails(): boolean {
    return Boolean((window as unknown as IAppWindow).pageOnWails) && Boolean(goApp())
}

export function openSocket(path: string, url: () => string): WebSocket {
    return (pageOnWails() ? new BusSocket(path) : new WebSocket(url())) as unknown as WebSocket
}

// WebKit gives a custom scheme's handler no body for a FormData or a Blob, so a
// page on wails:// sends the form's encoded bytes.
export async function sendableForm(form: FormData): Promise<{body: BodyInit, contentType?: string}> {
    if (!pageOnWails()) {
        return {body: form}
    }
    const encoded = new Response(form)
    return {body: await encoded.arrayBuffer(), contentType: encoded.headers.get('Content-Type') ?? undefined}
}
