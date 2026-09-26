import {render} from '@solidjs/testing-library'

import {mockAppStore, wrapIntl} from '../../../testUtils'
import {AppStoreProvider} from '../../../store'

import AddDescriptionTourStep from './add_description'

// The tip about the description. The tour runs on the board it was started on, found
// by id: it used to look for a board called «Welcome to Boards!», which no
// template was, and nobody ever saw it.
describe('components/onboardingTour/AddDescriptionTourStep', () => {
    const stateOn = (tourBoard: string) => ({
        users: {
            me: {
                id: 'user_id_1',
            },
            myConfig: {
                onboardingTourStarted: {value: true},
                tourCategory: {value: 'card'},
                onboardingTourStep: {value: '2'},
                tourBoard: {value: tourBoard},
            },
        },
        boards: {
            boards: {
                board_id_1: {id: 'board_id_1', title: 'Any board'},
            },
            current: 'board_id_1',
        },
        cards: {
            cards: {
                card_id_1: {id: 'card_id_1', title: 'Any card'},
            },
            current: 'card_id_1',
        },
        clientConfig: {
            value: {teamMode: false},
        },
    })

    const renderOn = (tourBoard: string) => render(() => wrapIntl(() =>
        <AppStoreProvider store={mockAppStore(stateOn(tourBoard))}>
            <AddDescriptionTourStep/>
        </AppStoreProvider>,
    ))

    test('is drawn on the board the tour runs on, whatever it is called', () => {
        renderOn('board_id_1')

        // The dot and the box.
        expect(document.querySelectorAll('.AddDescriptionTourStep')).toHaveLength(2)
    })

    test('is not drawn on any other board', () => {
        renderOn('another_board')
        expect(document.querySelectorAll('.AddDescriptionTourStep')).toHaveLength(0)
    })
})
