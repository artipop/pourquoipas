export interface IAppWindow extends Window {
    baseURL?: string

    // Absolute base URL the WebSocket client should connect to, overriding the
    // page origin. Set by native desktop wrappers (e.g. the Wails macOS app)
    // whose webview origin differs from the server, so the socket reaches the
    // real server directly. Unused in browser/plugin deployments.
    webSocketBaseURL?: string
    getCurrentTeamId?: () => string
    msCrypto: Crypto
    openInNewBrowser?: ((href: string) => void) | null

    // Set by the bootstrap the front door injects (proxy.go): there is a Go
    // side behind this origin, and goApp (goBridge.ts) hands out its bindings.
    // Absent under jsdom unless a test asks for one (src/test/goSide.ts).
    hasGoSide?: boolean
    webkit?: {messageHandlers: {nativeApp?: {postMessage: <T>(message: T) => void}}}
}
