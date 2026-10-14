import {goApp, installGoBridge} from './goBridge'
import {fakeGoSide} from './test/goSide'
import type {IAppWindow} from './types'

function click(anchor: HTMLAnchorElement): MouseEvent {
    const event = new MouseEvent('click', {bubbles: true, cancelable: true, button: 0})
    anchor.dispatchEvent(event)
    return event
}

describe('goBridge', () => {
    // Every desktop-only screen hides itself on this answer, so a page with no
    // Go side behind it has to get "none" rather than bindings that fail on the
    // first call.
    test('hands out no bindings where nothing answers behind the page', () => {
        fakeGoSide()
        expect(goApp()).toBeUndefined()

        fakeGoSide({OpenInBrowser: vi.fn()})
        expect(goApp()).toBeDefined()
    })

    // The desktop webview cannot navigate to an outside origin, so every outward
    // link has to be handed to the system browser. A version that only caught
    // target=_blank anchors without an inline onclick left a markdown link in a
    // card comment dead on click.
    test('sends every outward link to the system browser, and only those', () => {
        const OpenInBrowser = vi.fn()
        fakeGoSide({OpenInBrowser})
        installGoBridge()

        const inlineHandler = vi.fn()
        const outward = document.createElement('a')
        outward.href = 'https://example.com/docs'
        outward.addEventListener('click', inlineHandler)
        const inApp = document.createElement('a')
        inApp.href = `${window.location.origin}/board/1`
        document.body.append(outward, inApp)

        // Followed, it would navigate the test's own page away.
        inApp.addEventListener('click', (e) => e.preventDefault())

        expect(click(outward).defaultPrevented).toBe(true)
        expect(OpenInBrowser).toHaveBeenCalledWith('https://example.com/docs')
        expect(inlineHandler).not.toHaveBeenCalled()

        OpenInBrowser.mockClear()
        click(inApp)
        expect(OpenInBrowser).not.toHaveBeenCalled()

        // Generated markup reaches the same place from an inline onclick.
        ;(window as unknown as IAppWindow).openInNewBrowser?.('https://example.com/csv')
        expect(OpenInBrowser).toHaveBeenCalledWith('https://example.com/csv')
    })
})
