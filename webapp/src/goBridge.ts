import type {IAppWindow} from './types'

// installGoBridge puts the bindings generated from the Go side's App service
// (webapp/bindings, `wails3 task generate:bindings`) where the page reaches
// them: window.go.main.App, which agentBindings() and every desktop-only screen
// feature-detect.
//
// Only when the bootstrap the front door injects says there is a Go side
// (window.hasGoSide, proxy.go). The same bundle runs in a plain browser, and
// there the bindings stay unloaded: importing them loads @wailsio/runtime,
// which goes looking for a webview and a server that are not there. That is
// also why the import is dynamic rather than part of the main chunk.
//
// A test puts a fake in the same place, which is why the seam is a global
// rather than an import every caller makes.
export async function installGoBridge(): Promise<void> {
    const w = window as unknown as IAppWindow
    if (!w.hasGoSide) {
        return
    }
    const App = await import('../bindings/github.com/artipop/xciii/app')
    w.go = {main: {App}}
}
