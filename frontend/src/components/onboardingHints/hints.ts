import {createEffect, createSignal, onCleanup} from 'solid-js'
import type {Accessor} from 'solid-js'

import {useAppSelector, useAppStore} from '../../store/hooks'
import {getMe, getShownHints} from '../../store/users'
import {patchPreferences} from '../onboardingTour/tourState'

// Hints: what is worth learning at a moment that arrives on its own — the first
// question an agent asks, the first card it moves — which the tour, walking a
// person through the board once, cannot schedule (docs/onboarding.md).
//
// Each is shown once and remembered as an id in the `shownHints` preference: a
// set, not a counter, because hints have no order. One is drawn at a time: the
// first place that wants a hint holds the slot until its hint is dismissed or
// the place goes away.

export const HINT_QUESTION = 'question'
export const HINT_AGENT_MOVE = 'agentMove'
export const HINT_INBOX = 'inbox'
export const HINT_CHANGES = 'changes'

// Not drawn as a tip but kept in the same set: the strip over the board while
// the first card an agent works is running.
export const HINT_FIRST_RUN = 'firstRun'

export type Hints = {
    shown: (id: string) => boolean
    markShown: (id: string) => Promise<void>
}

export function useHints(): Hints {
    const {actions} = useAppStore()
    const me = useAppSelector(getMe)
    const shownHints = useAppSelector(getShownHints)
    return {
        shown: (id) => shownHints().includes(id),
        markShown: async (id) => {
            const userId = me()?.id
            if (!userId || shownHints().includes(id)) {
                return
            }
            await patchPreferences(userId, {shownHints: JSON.stringify([...shownHints(), id])}, actions.users.patchProps)
        },
    }
}

// The one slot. A token rather than the hint's id, because the same hint can
// be wanted by several places at once — every card with a question on it —
// and it is drawn on one of them.
const [holder, setHolder] = createSignal<symbol | undefined>(undefined)

// useHintSlot says whether this place draws its hint now: it wants to and the
// slot is free or already its own.
export function useHintSlot(wanted: Accessor<boolean>): Accessor<boolean> {
    const token = Symbol('hint')
    createEffect(() => [wanted(), holder()] as const, ([wants, current]) => {
        if (wants && !current) {
            setHolder(() => token)
        } else if (!wants && current === token) {
            setHolder(undefined)
        }
    })
    onCleanup(() => {
        if (holder() === token) {
            setHolder(undefined)
        }
    })
    return () => holder() === token
}

// The column each card was last drawn in. A card moved to another column is
// drawn by another column's list, as a component made anew, so what it was
// before has to be kept outside it.
const lastColumn = new Map<string, string>()

// cardMovedFrom records where a card is drawn now and answers where it was
// drawn before, if that was somewhere else.
export function cardMovedFrom(cardId: string, column: string): string | undefined {
    const before = lastColumn.get(cardId)
    lastColumn.set(cardId, column)
    return before !== undefined && before !== column ? before : undefined
}
