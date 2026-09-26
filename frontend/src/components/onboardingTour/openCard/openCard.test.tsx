import {render} from '@solidjs/testing-library'

import {mockAppStore, wrapIntl} from '../../../testUtils'
import {AppStoreProvider} from '../../../store'

import OpenCardTourStep from './open_card'

// The tip about opening a card. The tour runs on the board it was started on, found
// by id: it used to look for a board called «Welcome to Boards!», which no
// template was, and nobody ever saw it.
describe('components/onboardingTour/OpenCardTourStep', () => {
    const stateOn = (tourBoard: string) => ({
        users: {
            me: {
                id: 'user_id_1',
            },
            myConfig: {
                onboardingTourStarted: {value: true},
                tourCategory: {value: 'onboarding'},
                onboardingTourStep: {value: '0'},
                tourBoard: {value: tourBoard},
            },
        },
        boards: {
            boards: {
                board_id_1: {id: 'board_id_1', title: 'Any board'},
            },
            current: 'board_id_1',
        },
        clientConfig: {
            value: {teamMode: false},
        },
    })

    const renderOn = (tourBoard: string) => render(() => wrapIntl(() =>
        <AppStoreProvider store={mockAppStore(stateOn(tourBoard))}>
            <OpenCardTourStep/>
        </AppStoreProvider>,
    ))

    test('is drawn on the board the tour runs on, whatever it is called', () => {
        renderOn('board_id_1')

        // The dot and the box.
        expect(document.querySelectorAll('.OpenCardTourStep')).toHaveLength(2)
    })

    test('is not drawn on any other board', () => {
        renderOn('another_board')
        expect(document.querySelectorAll('.OpenCardTourStep')).toHaveLength(0)
    })
})
