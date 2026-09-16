import {render, screen} from '@solidjs/testing-library'

import '@testing-library/jest-dom'
import userEvent from '@testing-library/user-event'

import {mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {TestBlockFactory} from '../../test/testBlockFactory'
import mutator from '../../mutator'
import {Flow} from '../acp/automation'

import ViewHeaderFlowMenu from './viewHeaderFlowMenu'

vi.mock('../../mutator')
const mockedMutator = vi.mocked(mutator)

const board = TestBlockFactory.createBoard()
const activeView = TestBlockFactory.createBoardView(board)

const routes: Flow[] = [
    {id: 'flow-feature', name: 'Фича', nodes: [], edges: []},
    {id: 'flow-hotfix', name: 'Хотфикс', nodes: [], edges: []},
]

describe('components/viewHeader/viewHeaderFlowMenu', () => {
    const store = mockAppStore({users: {me: {id: 'user-id-1', username: 'username_1'}}})
    beforeEach(() => {
        vi.clearAllMocks()
    })

    const draw = (flows: Flow[], view = activeView) => render(() =>
        wrapIntl(() =>
            <AppStoreProvider store={store}>
                <ViewHeaderFlowMenu
                    activeView={view}
                    flows={flows}
                />
            </AppStoreProvider>,
        ),
    )

    // A board with no route has nothing to draw by.
    test('offers nothing on a board with no route', () => {
        expect(draw([]).container.querySelector('.MenuWrapper')).toBeNull()
    })

    // One route is still worth narrowing to: the board's other columns are not
    // on it.
    test('is offered on a board with a single route', () => {
        expect(draw([routes[0]]).container.querySelector('.MenuWrapper')).not.toBeNull()
    })

    test('names the route the board is being drawn by', () => {
        const view = {...activeView, fields: {...activeView.fields, flowId: 'flow-hotfix'}}
        draw(routes, view)
        expect(screen.getByText('Хотфикс')).toBeInTheDocument()
    })

    test('remembers the pick on the view, so it survives a reload', () => {
        draw(routes)
        userEvent.click(screen.getByRole('button', {name: 'menuwrapper'}))
        userEvent.click(screen.getByText('Фича'))
        expect(mockedMutator.changeViewFlowId).toHaveBeenCalledWith(activeView.boardId, activeView.id, undefined, 'flow-feature')
    })

    test('goes back to the whole board', () => {
        const view = {...activeView, fields: {...activeView.fields, flowId: 'flow-feature'}}
        draw(routes, view)
        userEvent.click(screen.getByRole('button', {name: 'menuwrapper'}))
        userEvent.click(screen.getByText('All routes'))
        expect(mockedMutator.changeViewFlowId).toHaveBeenCalledWith(view.boardId, view.id, 'flow-feature', '')
    })
})
