// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {IUser} from '../../user'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'

// Which of a board's people may be put in a card's «кто занимается». Every
// agent registered on this machine gets an account on every board, and a board
// keeps no crew of its own, so with the agents on the answer is everybody.
//
// The subtraction left is the switch: with the agents off an agent is nobody to
// hand a card to, and the field has to stop offering one. That needs to know
// which of the board's members are agents, which is what the registry says
// (ListAgentAccounts) — the fold from an agent's name to its board username is
// Go's, and writing it a second time in TypeScript would be two answers to "is
// this an agent".

// Asked once and shared: a card dialog draws one person property, a table draws
// one per row, and they all want the same list.
let asked: Promise<string[]> | undefined

// invalidateAgentUsers drops the answer. There is no event to listen for —
// nothing on the Go side announces a registry edit — so this is called where
// one is made: the agents panel, and the sync a board runs when it is opened.
export function invalidateAgentUsers(): void {
    asked = undefined
}

// loadAgentUsers answers with the board usernames of this machine's agents, and
// with none at all outside the desktop app: a browser or plugin build has no
// registry, so there are no agents to tell from people.
export function loadAgentUsers(): Promise<string[]> {
    const bindings = goApp()
    if (!bindings?.ListAgentAccounts) {
        return Promise.resolve([])
    }
    if (asked) {
        return asked
    }
    asked = bindings.ListAgentAccounts().
        then((raw) => ((JSON.parse(raw) || []) as Array<{username?: string}>).
            map((account) => account.username || '').
            filter(Boolean)).
        catch(() => {
            // A registry that could not be read narrows nothing, which is the
            // safe direction: the field offers too many rather than none.
            asked = undefined
            return []
        })
    return asked
}

// keepAssignable is the subtraction itself, kept apart from the loading so the
// rule can be read on its own. Matching is by username, which is what the board
// holds and what Go folded the registry to.
export function keepAssignable(users: IUser[], agents: string[]): IUser[] {
    if (aiEnabled() || agents.length === 0) {
        return users
    }
    const isAgent = new Set(agents)
    return users.filter((user) => !isAgent.has(user.username))
}

// assignableFilter is what a person property hands to the selector: the list is
// fetched (once, and shared) and the subtraction applied to whatever the search
// found.
export function assignableFilter(): (users: IUser[]) => Promise<IUser[]> {
    return (users: IUser[]) => loadAgentUsers().then((agents) => keepAssignable(users, agents))
}
