import {IUser} from '../../user'

import {fakeGoSide} from '../../test/goSide'
import {setAIEnabled} from '../../aiSwitch'

import {invalidateAgentUsers, keepAssignable, loadAgentUsers} from './agentAssignees'

const asUser = (username: string): IUser => ({username} as IUser)

describe('components/acp/agentAssignees', () => {
    afterEach(() => {
        fakeGoSide()
        invalidateAgentUsers()
        setAIEnabled(true)
        vi.clearAllMocks()
    })

    // Every agent on the machine has an account on every board, and which one
    // works a card is the card's own answer — so the field offers all of them.
    test('every agent is offered on every board', () => {
        const users = [asUser('artem'), asUser('cl'), asUser('coc')]
        expect(keepAssignable(users, ['cl', 'coc'])).toEqual(users)
    })

    // With the agents switched off there is nothing an agent could do with a
    // card, so it stops being somebody to hand one to. A person is untouched.
    test('with the agents off an agent is not offered, and a person still is', () => {
        setAIEnabled(false)
        const users = [asUser('artem'), asUser('cl'), asUser('coc')]
        expect(keepAssignable(users, ['cl', 'coc']).map((u) => u.username)).toEqual(['artem'])
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
