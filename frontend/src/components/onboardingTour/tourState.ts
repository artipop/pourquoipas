import {createEffect} from 'solid-js'
import type {Accessor} from 'solid-js'

import {aiEnabled} from '../../aiSwitch'
import {isHosted} from '../../buildKind'
import octoClient from '../../octoClient'
import {getTeamMode} from '../../store/clientConfig'
import {getCurrentBoard} from '../../store/boards'
import {getCurrentCard} from '../../store/cards'
import {useAppSelector, useAppStore} from '../../store/hooks'
import {
    getMe,
    getOnboardingTourCategory,
    getOnboardingTourStarted,
    getOnboardingTourStep,
    getTourBoard,
} from '../../store/users'
import {UserConfigPatch, UserPreference} from '../../user'

import {BaseTourSteps, TOUR_BASE, TOUR_CARD, TourFlags, tourStepNumber} from './index'

// What every tip asks before it draws itself, in one place: which switches are
// on, whether the tour is on this board, whether it stands at this step.

export function useTourFlags(): Accessor<TourFlags> {
    const team = useAppSelector(getTeamMode)
    return () => ({ai: aiEnabled(), team: team(), folders: !isHosted()})
}

// Whether the board on screen is the one the tour was started on. Asked by id:
// the tour used to look for a board called «Welcome to Boards!», which no
// template was, so nobody ever saw it.
export function useOnTourBoard(): Accessor<boolean> {
    const board = useAppSelector(getCurrentBoard)
    const tourBoard = useAppSelector(getTourBoard)
    return () => Boolean(board()?.id && board()!.id === tourBoard())
}

// Whether the tour stands at this step of this category now. A step these
// switches leave out is never current.
export function useTourAt(category: string, step: string): Accessor<boolean> {
    const flags = useTourFlags()
    const started = useAppSelector(getOnboardingTourStarted)
    const current = useAppSelector(getOnboardingTourCategory)
    const at = useAppSelector(getOnboardingTourStep)
    return () => {
        const number = tourStepNumber(category, step, flags())
        return number >= 0 && started() && current() === category && at() === number.toString()
    }
}

type PatchProps = (props: UserPreference[]) => void

// startTour puts the tour at its first tip on this board.
export async function startTour(userId: string, boardId: string, patchProps: PatchProps): Promise<void> {
    await patchPreferences(userId, {
        onboardingTourStarted: '1',
        tourCategory: TOUR_BASE,
        onboardingTourStep: tourStepNumber(TOUR_BASE, BaseTourSteps.OPEN_A_CARD, {ai: false, team: false}).toString(),
        tourBoard: boardId,
    }, patchProps)
}

// patchPreferences writes preferences and takes back what the server now holds:
// the tour's, and the hints' set.
export async function patchPreferences(userId: string, fields: Record<string, string>, patchProps: PatchProps): Promise<void> {
    const patch: UserConfigPatch = {updatedFields: fields}
    const patched = await octoClient.patchUserConfig(userId, patch)
    if (patched) {
        patchProps(patched)
    }
}

// useStartCardTour moves the tour from «open a card» to the card's own tips as
// soon as a card is open on the tour's board — whichever card it is. Mounted
// by the card's property list, which every open card draws.
export function useStartCardTour(): void {
    const {actions} = useAppStore()
    const me = useAppSelector(getMe)
    const card = useAppSelector(getCurrentCard)
    const onTourBoard = useOnTourBoard()
    const waiting = useTourAt(TOUR_BASE, BaseTourSteps.OPEN_A_CARD)

    createEffect(() => (waiting() && onTourBoard() && card()?.id ? me()?.id : undefined), (userId) => {
        if (userId) {
            patchPreferences(userId, {tourCategory: TOUR_CARD, onboardingTourStep: '0'}, actions.users.patchProps)
        }
    })
}
