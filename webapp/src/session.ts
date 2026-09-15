// The board session token, kept twice: a header for the board, a cookie for the
// front door — which can be handed neither a header on a WebSocket handshake
// nor one on the Wails runtime's own fetches (frontdoor.go).

const TOKEN_KEY = 'boardSessionId'

// Kept in step with sessionCookie in frontdoor.go.
const COOKIE = 'boardSession'

export function sessionToken(): string {
    return localStorage.getItem(TOKEN_KEY) || ''
}

// rememberSession stores a token both ways.
export function rememberSession(token: string): void {
    localStorage.setItem(TOKEN_KEY, token)
    writeCookie(token)
}

export function forgetSession(): void {
    localStorage.removeItem(TOKEN_KEY)
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
