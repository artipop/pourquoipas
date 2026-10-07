import {render, screen} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {mockAppStore, wrapIntl, wrapStore} from './testUtils'
import FocalboardRouter from './router'

// The board's own routes end in a catch-all — /:boardId?/:viewId?/:cardId? —
// which is exactly the shape a path like /m falls into. If the ranking ever
// changed, a phone would silently get an empty board instead of the page
// written for it, and nothing else in the suite would notice.

vi.mock('./pages/boardPage/boardPage', () => ({default: (props: {readonly?: boolean}) => <div>{props.readonly ? 'the shared board' : 'the board'}</div>}))
vi.mock('./pages/mobile/mobilePage', () => ({default: () => <div>{'the phone page'}</div>}))
vi.mock('./components/acp/diffPage', () => ({default: () => <div>{'the diff'}</div>}))
vi.mock('./pages/share/sharePage', () => ({default: () => <div>{'the share dialog'}</div>}))

const renderAt = (path: string) => {
    window.history.pushState({}, '', path)
    const store = mockAppStore({users: {me: {id: 'user-1', username: 'u'} as any}})
    return render(() => wrapStore(store, () => wrapIntl(() => <FocalboardRouter/>)))
}

describe('router', () => {
    afterEach(() => window.history.pushState({}, '', '/'))

    it('gives /m the page written for a phone, not the board', async () => {
        renderAt('/m')

        expect(await screen.findByText('the phone page')).toBeInTheDocument()
    })

    // A diff window is opened by the app itself, on an address nobody types.
    // The board's catch-all would swallow it exactly as it would swallow /m.
    it('gives /acp/diff/:cardId the changes, not the board', async () => {
        renderAt('/acp/diff/card-1')

        expect(await screen.findByText('the diff')).toBeInTheDocument()
    })

    // /share falls into the same catch-all, and it is opened by the system's
    // share sheet — where an empty board instead of the dialog would be a
    // feature that silently stopped existing.
    it('gives /share the share dialog, not the board', async () => {
        renderAt('/share')

        expect(await screen.findByText('the share dialog')).toBeInTheDocument()
    })

    it('still gives a board id the board', async () => {
        renderAt('/79dfb64e-9a41-4d69-9e0e-27dfd4f5f9d3')

        expect(await screen.findByText('the board')).toBeInTheDocument()
    })

    it.each(['/board/board-1', '/team/team-1/board-1'])('loads the board at %s', async (path) => {
        renderAt(path)

        expect(await screen.findByText('the board')).toBeInTheDocument()
    })

    it.each(['/shared/board-1', '/team/team-1/shared/board-1'])('loads the shared board read-only at %s', async (path) => {
        renderAt(path)

        expect(await screen.findByText('the shared board')).toBeInTheDocument()
    })
})
