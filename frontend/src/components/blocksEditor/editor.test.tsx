import {render, screen, fireEvent} from '@solidjs/testing-library'

import {mockAppStore, wrapDNDIntl, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {TestBlockFactory} from '../../test/testBlockFactory'

import Editor from './editor'

describe('components/blocksEditor/editor', () => {
    const board1 = TestBlockFactory.createBoard()
    board1.id = 'board-id-1'

    const state = {
        users: {
            boardUsers: {
                1: {username: 'abc'},
                2: {username: 'd'},
                3: {username: 'e'},
                4: {username: 'f'},
                5: {username: 'g'},
            },
        },
        boards: {
            current: 'board-id-1',
            boards: {
                [board1.id]: board1,
            },
        },
        clientConfig: {
            value: {},
        },
    }
    const store = mockAppStore(state)

    test('should match snapshot', async () => {
        const {container} = render(() => wrapIntl(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Editor
                    id='block-id'
                    boardId='fake-board-id'
                    initialValue='test-value'
                    initialContentType='text'
                    onSave={vi.fn()}
                />
            </AppStoreProvider>,
        )))
        expect(container).toMatchSnapshot()
    })

    test('shows an empty input', () => {
        render(() => wrapIntl(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Editor
                    boardId='fake-board-id'
                    onSave={vi.fn()}
                />
            </AppStoreProvider>,
        )))
        expect(screen.getByRole('combobox')).toHaveValue('')
        expect(screen.getByText('Type your text, or a slash command')).toBeInTheDocument()
    })

    test('should call onSave after introduce text and hit enter', async () => {
        const onSave = vi.fn()
        render(() => wrapIntl(() => wrapDNDIntl(() =>
            <AppStoreProvider store={store}>
                <Editor
                    boardId='fake-board-id'
                    onSave={onSave}
                />
            </AppStoreProvider>,
        )))
        let input = screen.getByDisplayValue('')
        expect(onSave).not.toHaveBeenCalled()
        fireEvent.input(input, {target: {value: '/title'}})
        fireEvent.keyDown(input, {key: 'Enter'})
        expect(onSave).not.toHaveBeenCalled()

        input = screen.getByDisplayValue('')
        fireEvent.input(input, {target: {value: 'test'}})
        fireEvent.keyDown(input, {key: 'Enter'})

        expect(onSave).toHaveBeenCalledWith(expect.objectContaining({value: 'test'}))
    })
})
