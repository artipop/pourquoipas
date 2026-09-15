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
    // side behind this origin. Absent in a plain browser.
    hasGoSide?: boolean

    // The bindings generated from the Go side's App service (webapp/bindings),
    // put here by goBridge.ts when hasGoSide is set, so their types are Go's
    // own. Absent in a plain browser; feature-detect before use. A test puts a
    // fake here.
    go?: {
        main?: {
            App?: typeof import('../../bindings/github.com/artipop/xciii/app')
        }
    }
    webkit?: {messageHandlers: {nativeApp?: {postMessage: <T>(message: T) => void}}}
}
