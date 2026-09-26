// The tour is Focalboard's: four categories walked in order, each a few tips
// over the real page, progress kept in the user's preferences. What changed is
// what the tips are about — the one thing no other board does, a card in a
// column that an agent works — and that the steps are no longer a constant.
//
// A step is named, not numbered. Which steps a category has depends on what is
// switched on: with the agents off nothing about an agent is shown, and a
// Focalboard feature that only works with a second person (comments, sharing,
// personal categories) is shown only in team mode. The number a tip is saved
// under is its position in the list for the current switches, so the dots, the
// «Next» and the «Done» all read one answer (tourSteps).

export const BaseTourSteps = {
    OPEN_A_CARD: 'OPEN_A_CARD',
}

export const CardTourSteps = {
    ASSIGN_AGENT: 'ASSIGN_AGENT',
    PICK_FOLDER: 'PICK_FOLDER',
    ADD_DESCRIPTION: 'ADD_DESCRIPTION',
    ADD_COMMENTS: 'ADD_COMMENTS',
}

export const BoardTourSteps = {
    COLUMNS: 'COLUMNS',
    PLAN_TASK: 'PLAN_TASK',
    BOARD_MENU: 'BOARD_MENU',
    ADD_VIEW: 'ADD_VIEW',
    COPY_LINK: 'COPY_LINK',
    SHARE_BOARD: 'SHARE_BOARD',
}

export const SidebarTourSteps = {
    INBOX: 'INBOX',
    SIDE_BAR: 'SIDE_BAR',
    MANAGE_CATEGORIES: 'MANAGE_CATEGORIES',
    SEARCH_FOR_BOARDS: 'SEARCH_FOR_BOARDS',
}

export const FINISHED = 999

// The class the board's first card carries, which the first tip is drawn over.
export const OnboardingCardClassName = 'onboardingCard'

export const TOUR_BASE = 'onboarding'
export const TOUR_CARD = 'card'
export const TOUR_BOARD = 'board'
export const TOUR_SIDEBAR = 'sidebar'

export const TOUR_ORDER = [
    TOUR_BASE,
    TOUR_CARD,
    TOUR_BOARD,
    TOUR_SIDEBAR,
]

// What the tour may talk about: whether agents are switched on, and whether
// there is a second person.
export type TourFlags = {
    ai: boolean
    team: boolean
}

// tourScript is the steps of one category, in order, for these switches.
export function tourScript(category: string, flags: TourFlags): string[] {
    const when = (on: boolean, ...steps: string[]) => (on ? steps : [])
    switch (category) {
    case TOUR_BASE:
        return [BaseTourSteps.OPEN_A_CARD]
    case TOUR_CARD:
        return [
            ...when(flags.ai, CardTourSteps.ASSIGN_AGENT, CardTourSteps.PICK_FOLDER),
            CardTourSteps.ADD_DESCRIPTION,
            ...when(flags.team, CardTourSteps.ADD_COMMENTS),
        ]
    case TOUR_BOARD:
        return [
            ...when(flags.ai, BoardTourSteps.COLUMNS, BoardTourSteps.PLAN_TASK, BoardTourSteps.BOARD_MENU),
            BoardTourSteps.ADD_VIEW,
            BoardTourSteps.COPY_LINK,
            ...when(flags.team, BoardTourSteps.SHARE_BOARD),
        ]
    case TOUR_SIDEBAR:
        return [
            SidebarTourSteps.INBOX,
            SidebarTourSteps.SIDE_BAR,
            ...when(flags.team, SidebarTourSteps.MANAGE_CATEGORIES),
            SidebarTourSteps.SEARCH_FOR_BOARDS,
        ]
    default:
        return []
    }
}

// tourSteps is the same list as the manager reads it: step name → number.
export function tourSteps(category: string, flags: TourFlags): Record<string, number> {
    const steps: Record<string, number> = {}
    tourScript(category, flags).forEach((step, i) => {
        steps[step] = i
    })
    return steps
}

// tourStepNumber is where a step stands in its category, or -1 when these
// switches leave it out.
export function tourStepNumber(category: string, step: string, flags: TourFlags): number {
    return tourScript(category, flags).indexOf(step)
}
