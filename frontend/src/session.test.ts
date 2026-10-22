import {forgetSession, rememberSession, sessionToken} from './session'
import type {IAppWindow} from './types'

const cookie = (): string => document.cookie

describe('session', () => {
    beforeEach(() => {
        localStorage.clear()
        forgetSession()
    })

    // The token has to travel twice, and the second way is what the front door
    // reads: the Wails runtime makes its own fetches and a browser lets nobody
    // set a header on a WebSocket handshake (frontdoor.go). Losing this is losing
    // every bound method and both sockets on a team install.
    it('keeps a session where both the page and the front door can read it', () => {
        rememberSession('a-token')

        expect(sessionToken()).toBe('a-token')
        expect(cookie()).toContain('boardSession=a-token')
    })

    it('takes both away when the session ends', () => {
        rememberSession('a-token')
        forgetSession()

        expect(sessionToken()).toBe('')
        expect(cookie()).not.toContain('a-token')
    })

    // wails:// storage outlives the app: a token left there is from a launch
    // whose session is gone.
    it('a desktop window takes its session from the bootstrap, not the browser', () => {
        localStorage.setItem('boardSessionId', 'stale')
        const w = window as unknown as IAppWindow
        w.pageOnWails = true
        w.seededSession = 'fresh'
        try {
            expect(sessionToken()).toBe('fresh')
        } finally {
            delete w.pageOnWails
        }
    })
})
