import {fakeGoSide} from './test/goSide'
import type {IAppWindow} from './types'

// The runtime's bus, standing in for Go's half of the tunnel: a test emits on
// it what Go would.
const listeners = vi.hoisted(() => new Map<string, (event: {data: unknown}) => void>())
vi.mock('@wailsio/runtime', async (importOriginal) => ({
    ...await importOriginal<Record<string, unknown>>(),
    Events: {
        On: (name: string, callback: (event: {data: unknown}) => void) => {
            listeners.set(name, callback)
            return () => listeners.delete(name)
        },
    },
}))

const {BusSocket, openSocket, sendableForm} = await import('./busSocket')

const tick = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('busSocket', () => {
    afterEach(() => {
        fakeGoSide()
        delete (window as unknown as IAppWindow).pageOnWails
        listeners.clear()
    })

    // The board's socket refused a subscription that overtook the login: two
    // binding calls in flight are two goroutines, and nothing orders them.
    test('sends frames one after another, in the order they were written', async () => {
        const sent: string[] = []
        let release!: () => void
        const first = new Promise<void>((resolve) => {
            release = resolve
        })
        fakeGoSide({
            OpenSocket: vi.fn().mockResolvedValue(undefined),
            SendSocket: vi.fn(async (_id: string, data: string) => {
                if (data === 'login') {
                    await first
                }
                sent.push(data)
            }),
        })
        const socket = new BusSocket('/ws')
        await tick()

        socket.send('login')
        socket.send('subscribe')
        await tick()
        expect(sent).toEqual([])

        release()
        await tick()
        await tick()
        expect(sent).toEqual(['login', 'subscribe'])
    })

    // What Go hears from the socket reaches the page as a message, and the end
    // of it as a close, so wsclient.ts reconnects as it would from a real one.
    test('delivers frames and the close Go reports', async () => {
        fakeGoSide({OpenSocket: vi.fn().mockResolvedValue(undefined)})
        const socket = new BusSocket('/ws')
        const opened = vi.fn()
        const messages: string[] = []
        const closed = vi.fn()
        socket.onopen = opened
        socket.onmessage = (e) => messages.push(e.data)
        socket.onclose = closed
        await tick()

        const emit = [...listeners.values()][0]
        emit({data: {data: '{"action":"UPDATE_BLOCK"}'}})
        emit({data: {closed: true, code: 1001}})

        expect(opened).toHaveBeenCalled()
        expect(messages).toEqual(['{"action":"UPDATE_BLOCK"}'])
        expect(closed).toHaveBeenCalledWith({code: 1001, reason: ''})
        expect(listeners.size).toBe(0)
    })

    // A page in a browser opens its sockets itself; only a desktop window,
    // whose page is on wails://, goes through Go.
    test('goes through Go only in a desktop window', () => {
        const opened: string[] = []
        const real = vi.spyOn(window, 'WebSocket').mockImplementation(function(this: unknown, url: string | URL) {
            opened.push(String(url))
        } as unknown as typeof WebSocket)
        fakeGoSide({OpenSocket: vi.fn().mockResolvedValue(undefined)})

        openSocket('/ws', () => 'ws://example.test/ws')
        expect(opened).toEqual(['ws://example.test/ws'])

        ;(window as unknown as IAppWindow).pageOnWails = true
        expect(openSocket('/ws', () => 'ws://example.test/ws')).toBeInstanceOf(BusSocket)
        real.mockRestore()
    })

    // WebKit gives a custom scheme's handler no body for a FormData: the file
    // reached the board as an empty multipart. The same form as bytes arrives.
    test('sends a form as bytes from a desktop window', async () => {
        const form = new FormData()
        form.append('file', new Blob([new Uint8Array([1, 2, 3])]), 'x.bin')

        expect((await sendableForm(form)).body).toBe(form)

        fakeGoSide({})
        ;(window as unknown as IAppWindow).pageOnWails = true
        const {body, contentType} = await sendableForm(form)
        expect(body).toBeInstanceOf(ArrayBuffer)
        expect(contentType).toMatch(/^multipart\/form-data; boundary=/)
        expect(new TextDecoder().decode(body as ArrayBuffer)).toContain('filename="x.bin"')
    })
})
