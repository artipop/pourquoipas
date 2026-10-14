import {render} from '@solidjs/testing-library'
import {createRouter, memoryHistory} from '@solidjs/router'

import {TestBlockFactory} from '../../test/testBlockFactory'
import {wrapDNDIntl} from '../../testUtils'

import ShareBoardLoginButton from './shareBoardLoginButton'
vi.useFakeTimers()

const boardId = '1'

const board = TestBlockFactory.createBoard()
board.id = boardId

describe('src/components/shareBoard/shareBoardLoginButton', () => {
    test('should match snapshot', async () => {
        const history = memoryHistory()
        history.set({value: '/team1/boardId1/viewId1/cardId1'})
        const Instance = createRouter({
            history,
            routes: [{path: '/:teamId/:boardId?/:viewId?/:cardId?', component: ShareBoardLoginButton}],
        })
        const result = render(() =>
            wrapDNDIntl(() => <Instance/>))
        const renderer = result.container

        expect(renderer).toMatchSnapshot()
    })
})
