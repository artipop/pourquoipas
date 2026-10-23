import {IUser} from './user'
import {addBoardUsersSource, boardUsersChanged, extraBoardUsers, onBoardUsersChanged, resetExtensions} from './extensions'

describe('extensions', () => {
    afterEach(() => resetExtensions())

    // The board runs with nothing plugged in: it then knows its members only.
    test('with nothing registered a board knows nobody extra', async () => {
        expect(await extraBoardUsers('board-1')).toEqual([])
    })

    // A source that fails costs the board nothing but that source's people.
    test('a failing source is skipped, the rest are asked', async () => {
        addBoardUsersSource(() => Promise.reject(new Error('no registry')))
        addBoardUsersSource(() => Promise.resolve([{id: 'u-1', username: 'claude'} as IUser]))
        expect((await extraBoardUsers('board-1')).map((u) => u.id)).toEqual(['u-1'])
    })

    test('a change is heard until the listener is dropped', () => {
        const heard = vi.fn()
        const off = onBoardUsersChanged(heard)
        boardUsersChanged()
        off()
        boardUsersChanged()
        expect(heard).toHaveBeenCalledTimes(1)
    })
})
