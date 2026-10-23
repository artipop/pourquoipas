// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {IUser} from '../../user'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'
import {boardUsersChanged} from '../../extensions'

// Who may be put in a card's «Кто занимается» besides the board's people: the
// agents of the person signed in, on any board. An agent is its owner's, not a
// board's, so it is never a board member (docs/modes.md); the board learns about
// it through extensions.ts. What this build cannot run is not in the list to
// begin with: a server registers model agents only.

type AgentAccount = {name: string, username: string, userId?: string}

// Asked once and shared: a card dialog draws one person property, a table draws
// one per row, and they all want the same list.
let asked: Promise<AgentAccount[]> | undefined

// invalidateAgentUsers drops the answer and tells the open board to ask again.
// Nothing on the Go side announces a registry edit, so this is called where one
// is made: the agents panel.
export function invalidateAgentUsers(): void {
    asked = undefined
    boardUsersChanged()
}

function loadAgentAccounts(): Promise<AgentAccount[]> {
    const bindings = goApp()
    if (!bindings?.ListAgentAccounts) {
        return Promise.resolve([])
    }
    if (asked) {
        return asked
    }
    asked = bindings.ListAgentAccounts().
        then((raw) => ((JSON.parse(raw) || []) as AgentAccount[]).filter((a) => a.username)).
        catch(() => {
            asked = undefined
            return []
        })
    return asked
}

// loadAgentUsers answers with the usernames of this person's agents, which is
// what tells an agent's write from a person's.
export async function loadAgentUsers(): Promise<string[]> {
    return (await loadAgentAccounts()).map((a) => a.username)
}

// agentBoardUsers is what a board knows of the agents: an account each, drawn
// like any other person. None with the agents switched off.
export async function agentBoardUsers(): Promise<IUser[]> {
    if (!aiEnabled()) {
        return []
    }
    return (await loadAgentAccounts()).
        filter((a) => a.userId).
        map((a) => ({
            id: a.userId!,
            username: a.username,
            email: '',
            nickname: '',
            firstname: '',
            lastname: '',
            props: {},
            create_at: 0,
            update_at: 0,
            is_bot: true,
            is_guest: false,
            kind: 'agent',
        }))
}

const isAgent = (user: IUser, mine: Set<string>) => user.kind === 'agent' || mine.has(user.username)

// keepAssignable is the rule itself: people stay; an agent stays only if it is
// this person's and the agents are on. A search of the whole team on a server
// finds everybody's agents, and somebody else's is nobody to hand a card to.
export function keepAssignable(users: IUser[], agents: string[]): IUser[] {
    const mine = new Set(agents)
    return users.filter((user) => !isAgent(user, mine) || (aiEnabled() && mine.has(user.username)))
}

// assignableFilter is what a person property hands to the selector: applied to
// whatever list it found.
export function assignableFilter(): (users: IUser[]) => Promise<IUser[]> {
    return (users: IUser[]) => loadAgentUsers().then((agents) => keepAssignable(users, agents))
}
