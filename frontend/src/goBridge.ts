import * as App from '../bindings/github.com/artipop/xciii/app'

import type {IAppWindow} from './types'

// The App service as `wails3 task generate:bindings` writes it into
// frontend/bindings, imported the way the Wails template imports a service.
export type GoApp = typeof App

// goApp is the Go side's App service, or undefined when nothing answers behind
// this origin. Every desktop-only screen asks here and draws nothing without
// it.
//
// What says there is a Go side is window.hasGoSide, set by the bootstrap the
// front door injects (proxy.go): every page a build serves has it, so in
// practice its absence is a test that did not ask for one. A test that does
// asks with fakeGoSide (src/test/goSide.ts), which fills in the vi.mock the
// setup file puts over the bindings module — no test reaches the real
// runtime.
export function goApp(): GoApp | undefined {
    return (window as unknown as IAppWindow).hasGoSide ? App : undefined
}

// installGoBridge sends every outward link to the system browser through
// App.OpenInBrowser. It used to live in the bootstrap, which could only reach
// Go through a global the page no longer has.
//
// The catch-all runs in the capture phase and covers *every* absolute http(s)
// anchor, not only target=_blank ones without an inline handler: deferring to
// an inline handler leaves the link dead wherever that handler does not run,
// and the webview cannot navigate to an outside origin either, so the click
// does nothing at all. Capturing first makes this the one path out.
// window.openInNewBrowser stays a global because generated markup calls it
// from an inline onclick (utils.ts).
export function installGoBridge(): void {
    const app = goApp()
    if (!app) {
        return
    }
    const w = window as unknown as IAppWindow
    w.openInNewBrowser = (href: string) => {
        if (href) {
            // eslint-disable-next-line new-cap
            app.OpenInBrowser(href)
        }
    }
    document.addEventListener('click', (e) => {
        if (e.button !== 0 || e.defaultPrevented) {
            return
        }
        const a = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null
        if (!a) {
            return
        }
        const href = a.href || a.getAttribute('href') || ''
        if (!(/^https?:\/\//i).test(href)) {
            return
        }

        // In-app navigation is same-origin and stays in the webview; anything
        // pointing outside is for the system browser.
        try {
            if (new URL(href, window.location.href).origin === window.location.origin) {
                return
            }
        } catch {
            // An unparsable href is not in-app navigation either.
        }
        e.preventDefault()
        e.stopPropagation()
        w.openInNewBrowser?.(href)
    }, true)
}
