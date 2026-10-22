import type {IAppWindow} from './types'

// The board session token, kept twice: a header for the board, a cookie for a
// server build's guard, which can be handed neither a header on a WebSocket
// handshake nor one on the Wails runtime's own fetches (mode_server.go).

const TOKEN_KEY = 'boardSessionId'

// Kept in step with sessionCookie in frontdoor.go.
const COOKIE = 'boardSession'

// A desktop window holds its session in memory, handed over by the bootstrap
// on each load: wails:// storage outlives the app's database, and a token
// left there belongs to a launch that is gone.
let seeded: Map<string, string> | undefined

function sessionStorageOf(): Pick<Storage, 'getItem' | 'setItem' | 'removeItem'> {
    const w = window as unknown as IAppWindow
    if (!w.pageOnWails) {
        return localStorage
    }
    if (!seeded) {
        seeded = new Map(w.seededSession ? [[TOKEN_KEY, w.seededSession]] : [])
    }
    const memory = seeded
    return {
        getItem: (key) => memory.get(key) ?? null,
        setItem: (key, value) => {
            memory.set(key, value)
        },
        removeItem: (key) => {
            memory.delete(key)
        },
    }
}

export function sessionToken(): string {
    return sessionStorageOf().getItem(TOKEN_KEY) || ''
}

// rememberSession stores a token both ways.
export function rememberSession(token: string): void {
    sessionStorageOf().setItem(TOKEN_KEY, token)
    writeCookie(token)
}

export function forgetSession(): void {
    sessionStorageOf().removeItem(TOKEN_KEY)
    writeCookie('')
}

function writeCookie(token: string): void {
    if (typeof document === 'undefined') {
        return
    }
    const secure = typeof location !== 'undefined' && location.protocol === 'https:' ? '; Secure' : ''
    if (token) {
        document.cookie = `${COOKIE}=${token}; path=/; SameSite=Strict${secure}`
        return
    }
    document.cookie = `${COOKIE}=; path=/; SameSite=Strict${secure}; Max-Age=0`
}
