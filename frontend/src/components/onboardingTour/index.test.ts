import {
    BoardTourSteps,
    CardTourSteps,
    SidebarTourSteps,
    TOUR_BOARD,
    TOUR_CARD,
    TOUR_SIDEBAR,
    tourScript,
    tourStepNumber,
} from './index'

// Which tips a person is walked through follows what is switched on: nothing
// about an agent with the agents off, nothing that needs a second person on a
// desktop of one. The number a step is saved under is its place in that list,
// so «Next», «Done» and the dots all read the same answer.
describe('onboardingTour/tourScript', () => {
    test('the card is about its agent, its folder, its task and its comments', () => {
        expect(tourScript(TOUR_CARD, {ai: true, team: true})).toEqual([
            CardTourSteps.ASSIGN_AGENT,
            CardTourSteps.PICK_FOLDER,
            CardTourSteps.ADD_DESCRIPTION,
            CardTourSteps.ADD_COMMENTS,
        ])
    })

    // An agent's result is a comment, so the card's comments are toured on a
    // desktop of one person as well.
    test('the comments are toured in every install', () => {
        expect(tourScript(TOUR_CARD, {ai: true, team: false})).toContain(CardTourSteps.ADD_COMMENTS)
    })

    test('with the agents off, nothing about an agent is shown', () => {
        expect(tourScript(TOUR_CARD, {ai: false, team: true})).toEqual([CardTourSteps.ADD_DESCRIPTION, CardTourSteps.ADD_COMMENTS])
        expect(tourScript(TOUR_BOARD, {ai: false, team: false})).toEqual([BoardTourSteps.ADD_VIEW, BoardTourSteps.COPY_LINK])
    })

    test('sharing and personal categories are shown only where there is a second person', () => {
        expect(tourScript(TOUR_BOARD, {ai: true, team: false})).not.toContain(BoardTourSteps.SHARE_BOARD)
        expect(tourScript(TOUR_SIDEBAR, {ai: true, team: false})).not.toContain(SidebarTourSteps.MANAGE_CATEGORIES)
        expect(tourScript(TOUR_BOARD, {ai: true, team: true})).toContain(BoardTourSteps.SHARE_BOARD)
    })

    test('the board starts with its columns and the sidebar with the inbox', () => {
        expect(tourScript(TOUR_BOARD, {ai: true, team: false})[0]).toBe(BoardTourSteps.COLUMNS)
        expect(tourScript(TOUR_SIDEBAR, {ai: false, team: false})[0]).toBe(SidebarTourSteps.INBOX)
    })

    test('a step left out has no number, and the others close up', () => {
        expect(tourStepNumber(TOUR_CARD, CardTourSteps.ASSIGN_AGENT, {ai: false, team: false})).toBe(-1)
        expect(tourStepNumber(TOUR_CARD, CardTourSteps.ADD_DESCRIPTION, {ai: false, team: false})).toBe(0)
        expect(tourStepNumber(TOUR_CARD, CardTourSteps.ADD_DESCRIPTION, {ai: true, team: false})).toBe(2)
    })
})
