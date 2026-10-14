// A card rendered in a test subscribes to the agent event socket
// (components/acp/agentEvents), and the browser would dial the test server,
// which has no such route, then schedule a reconnect into the next test. A
// socket that connects to nothing is what every suite but agentEvents' own
// wants; that one installs a fake of its own over this. Installed before any
// module is evaluated, because a module may open its socket while loading.
class SilentWebSocket {
    static readonly CONNECTING = 0
    static readonly OPEN = 1
    static readonly CLOSING = 2
    static readonly CLOSED = 3

    readyState = SilentWebSocket.CONNECTING
    onopen: ((event: unknown) => void) | null = null
    onclose: ((event: unknown) => void) | null = null
    onerror: ((event: unknown) => void) | null = null
    onmessage: ((event: unknown) => void) | null = null

    constructor(public url: string) {}

    send() {}
    close() {
        this.readyState = SilentWebSocket.CLOSED
    }
    addEventListener() {}
    removeEventListener() {}
}

(globalThis as unknown as {WebSocket: unknown}).WebSocket = SilentWebSocket

// A side-effect-only file is a global script under --isolatedModules; this makes
// it a module without changing what it does.
export {}
