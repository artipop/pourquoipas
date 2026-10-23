import {IUser} from '../../user'

import {fakeGoSide} from '../../test/goSide'
import {setAIEnabled} from '../../aiSwitch'

import {agentBoardUsers, invalidateAgentUsers, keepAssignable, loadAgentUsers} from './agentAssignees'

const asUser = (username: string, kind?: string): IUser => ({username, kind} as IUser)

describe('components/acp/agentAssignees', () => {
    afterEach(() => {
        fakeGoSide()
        invalidateAgentUsers()
        setAIEnabled(true)
        vi.clearAllMocks()
    })

    // An agent is its owner's, and any of the owner's agents may work a card
    // on any board — none of them is a member of it.
    test('every agent of mine is offered on every board', () => {
        const users = [asUser('artem'), asUser('cl', 'agent'), asUser('coc', 'agent')]
        expect(keepAssignable(users, ['cl', 'coc'])).toEqual(users)
    })

    // On a server a search of the team finds everybody's agents.
    test('somebody else’s agent is not offered', () => {
        const users = [asUser('artem'), asUser('cl', 'agent'), asUser('theirs', 'agent')]
        expect(keepAssignable(users, ['cl']).map((u) => u.username)).toEqual(['artem', 'cl'])
    })

    // With the agents switched off there is nothing an agent could do with a
    // card, so it stops being somebody to hand one to. A person is untouched.
    test('with the agents off an agent is not offered, and a person still is', () => {
        setAIEnabled(false)
        const users = [asUser('artem'), asUser('cl', 'agent'), asUser('coc')]
        expect(keepAssignable(users, ['cl', 'coc']).map((u) => u.username)).toEqual(['artem'])
    })

    // The board learns about the agents from here, not from its members.
    test('a board knows my agents as accounts', async () => {
        fakeGoSide({ListAgentAccounts: vi.fn().mockResolvedValue(JSON.stringify([{name: 'Claude', username: 'claude', userId: 'u-1'}]))})
        const users = await agentBoardUsers()
        expect(users.map((u) => [u.id, u.username, u.kind])).toEqual([['u-1', 'claude', 'agent']])

        setAIEnabled(false)
        expect(await agentBoardUsers()).toEqual([])
    })

    // Outside the desktop app there is no registry, so there are no agents to
    // tell from people and nothing to narrow by.
    test('a build with no bindings offers everybody', async () => {
        expect(await loadAgentUsers()).toEqual([])
    })

    // One call however many person properties are drawn: a table row draws one
    // each, and they all ask the same question.
    test('the registry is asked for once and shared', async () => {
        const ListAgentAccounts = vi.fn().mockResolvedValue(JSON.stringify([{name: 'клаус', username: 'cl'}]))
        fakeGoSide({ListAgentAccounts})

        const [first, second] = await Promise.all([loadAgentUsers(), loadAgentUsers()])
        expect(first).toEqual(['cl'])
        expect(second).toEqual(first)
        expect(ListAgentAccounts).toHaveBeenCalledTimes(1)

        // And asked again once something says the registry may have moved.
        invalidateAgentUsers()
        await loadAgentUsers()
        expect(ListAgentAccounts).toHaveBeenCalledTimes(2)
    })

    // A refusal narrows nothing rather than emptying the field: wrong in the
    // direction that still lets somebody assign the card.
    test('a registry that cannot be read offers everybody', async () => {
        fakeGoSide({ListAgentAccounts: vi.fn().mockRejectedValue('нет реестра')})
        expect(await loadAgentUsers()).toEqual([])
    })
})
