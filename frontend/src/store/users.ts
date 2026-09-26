import {reconcile, storePath} from 'solid-js'

import {IUser, parseUserProps, UserPreference} from '../user'
import {Subscription} from '../wsclient'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type UsersState = {
    me: IUser|null
    boardUsers: {[key: string]: IUser}
    loggedIn: boolean|null
    blockSubscriptions: Subscription[]
    myConfig: Record<string, UserPreference>
}

export const initialUsersState = (): UsersState => ({
    me: null,
    boardUsers: {},
    loggedIn: null,
    blockSubscriptions: [],
    myConfig: {},
})

export const createUsersActions = ({setState, deps}: StoreContext) => ({
    setMe(me: IUser|null) {
        setState(storePath('users', 'me', me))
        setState(storePath('users', 'loggedIn', Boolean(me)))
    },
    setBoardUsers(users: IUser[]) {
        setState(storePath('users', 'boardUsers', users.reduce((acc: {[key: string]: IUser}, user: IUser) => {
            acc[user.id] = user
            return acc
        }, {})))
    },
    addBoardUsers(users: IUser[]) {
        setState(({users: {boardUsers}}) => {
            users.forEach((user: IUser) => {
                boardUsers[user.id] = user
            })
        })
    },
    removeBoardUsersById(userIds: string[]) {
        setState(({users: {boardUsers}}) => {
            userIds.forEach((userId: string) => {
                delete boardUsers[userId]
            })
        })
    },
    followBlock(subscription: Subscription) {
        setState(storePath('users', 'blockSubscriptions', (subs) => [...subs, subscription]))
    },
    unfollowBlock(subscription: Subscription) {
        setState(storePath('users', 'blockSubscriptions', (subs) => subs.filter((s) => s.blockId !== subscription.blockId)))
    },

    // The response carries every preference the person has left, so it replaces
    // the config rather than being merged into it — and `setState` with a plain
    // object merges. A preference *deleted* server-side therefore survived in
    // the store for ever: settings could forget that the welcome screen had been
    // shown, and the page went on believing it had.
    patchProps(props: UserPreference[]) {
        setState((draft) => {
            reconcile(parseUserProps(props))(draft.users.myConfig)
        })
    },
    async fetchMe(): Promise<void> {
        try {
            const [me, myConfig] = await Promise.all([
                deps.client.getMe(),
                deps.client.getMyConfig(),
            ])
            setState(storePath('users', 'me', me || null))
            setState(storePath('users', 'loggedIn', Boolean(me)))
            if (myConfig) {
                setState((draft) => {
                    reconcile(parseUserProps(myConfig))(draft.users.myConfig)
                })
            }
        } catch (e) {
            setState(storePath('users', 'me', null))
            setState(storePath('users', 'loggedIn', false))
            setState(storePath('users', 'myConfig', {}))
        }
    },

    // What this person has asked to be told about. It was a stub setting an
    // empty list, which made every reader of it wrong in the same direction:
    // nobody follows anything, so nobody is told (docs/teamwork.md).
    async fetchUserBlockSubscriptions(userId: string): Promise<void> {
        try {
            setState(storePath('users', 'blockSubscriptions', await deps.client.getUserBlockSubscriptions(userId)))
        } catch (e) {
            setState(storePath('users', 'blockSubscriptions', []))
        }
    },
})

export const getMe = (state: RootState): IUser|null => state.users.me
export const getLoggedIn = (state: RootState): boolean|null => state.users.loggedIn
export const getBoardUsers = (state: RootState): {[key: string]: IUser} => state.users.boardUsers
export const getMyConfig = (state: RootState): Record<string, UserPreference> => state.users.myConfig || {} as Record<string, UserPreference>

export const getBoardUsersList = (state: RootState): IUser[] =>
    Object.values(getBoardUsers(state)).sort((a, b) => a.username.localeCompare(b.username))

export const getUser = (userId: string): (state: RootState) => IUser|undefined => {
    return (state: RootState): IUser|undefined => {
        const users = getBoardUsers(state)
        return users[userId]
    }
}

export const getOnboardingTourStarted = (state: RootState): boolean => {
    const myConfig = getMyConfig(state)
    if (!myConfig) {
        return false
    }
    return Boolean(myConfig.onboardingTourStarted?.value)
}

export const getOnboardingTourStep = (state: RootState): string => {
    const myConfig = getMyConfig(state)
    if (!myConfig) {
        return ''
    }
    return myConfig.onboardingTourStep?.value
}

export const getOnboardingTourCategory = (state: RootState): string => {
    const myConfig = getMyConfig(state)
    return myConfig.tourCategory ? myConfig.tourCategory.value : ''
}

// The board the tour runs on, by id. Written when the tour starts, so the tour
// is on whatever board somebody was shown it from, whatever that board is
// called.
export const getTourBoard = (state: RootState): string => {
    const myConfig = getMyConfig(state)
    return myConfig.tourBoard ? String(myConfig.tourBoard.value || '') : ''
}

// The hints already shown, as a set of ids. A set rather than a counter: hints
// arrive at moments of their own, in no order, and one can be added, removed
// or shown again without touching the others (docs/onboarding.md).
export const getShownHints = (state: RootState): string[] => {
    const raw = getMyConfig(state).shownHints?.value
    if (typeof raw !== 'string' || !raw) {
        return []
    }
    try {
        const parsed = JSON.parse(raw)
        return Array.isArray(parsed) ? parsed.filter((id) => typeof id === 'string') : []
    } catch {
        return []
    }
}
