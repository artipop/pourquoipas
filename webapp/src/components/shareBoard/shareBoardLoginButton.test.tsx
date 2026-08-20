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
    const savedLocation = window.location

    afterEach(() => {
        window.location = savedLocation as string & Location
    })

    test('should match snapshot', async () => {
        window.location = Object.assign(new URL('https://example.org/mattermost'))
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
