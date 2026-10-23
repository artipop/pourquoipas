import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {Board, createBoard} from '../../blocks/board'

import {fakeGoSide} from '../../test/goSide'
import {setHosted} from '../../buildKind'

import PlanningDialog, {isPlanningAvailable} from './planningDialog'

// The board the dialog was opened from: what bounds where the agent may leave
// the cards it agrees on, and whose folders are offered.
const board: Board = createBoard({id: 'board-1'} as any)

// A conversation as Go reports one: the agent, where it reads, and what has
// been said in it so far.
const chat = {
    id: 'plan-1',
    boardId: 'board-1',
    agent: 'planner',
    folder: 'app',
    busy: false,
    messages: [],
}

function planningBindings(open: unknown[] = []) {
    return {
        ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'app', path: '/src/app'}])),
        ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'planner', kind: 'claude'}])),
        ListPlanning: vi.fn().mockResolvedValue(JSON.stringify(open)),
        StartPlanning: vi.fn().mockResolvedValue(JSON.stringify(chat)),
        SayToPlanning: vi.fn().mockResolvedValue(undefined),
        StopPlanningReply: vi.fn().mockResolvedValue(undefined),
        EndPlanning: vi.fn().mockResolvedValue(undefined),
        ListAttention: vi.fn().mockResolvedValue('[]'),
    }
}

describe('components/acp/planningDialog', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        fakeGoSide()
    })

    it('is inert without desktop bindings', () => {
        expect(isPlanningAvailable()).toBe(false)
    })

    // One registered agent answers the who-question by itself, so the dialog
    // opens straight onto the folder question — and a folder's name is an
    // answer, not an option in a list: clicking it starts the conversation.
    it('skips the one agent, and a folder name starts the conversation', async () => {
        const bindings = planningBindings()
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'app'}))

        await waitFor(() => expect(bindings.StartPlanning).toHaveBeenCalledWith('board-1', 'app', 'planner'))
    })

    // No folder is an answer of its own: the agent reads the board's own drafts
    // folder, which is what Go resolves an empty name into.
    it('offers the board’s drafts and starts in them', async () => {
        const bindings = planningBindings()
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'The board’s drafts'}))

        await waitFor(() => expect(bindings.StartPlanning).toHaveBeenCalledWith('board-1', '', 'planner'))
    })

    // A server has no folders (docs/modes.md): nothing is asked about one,
    // and the conversation is held in the board's drafts.
    it('on a server asks no folder and starts in the drafts', async () => {
        const bindings = planningBindings()
        fakeGoSide(bindings)
        setHosted(true)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'Start'}))
        expect(screen.queryByRole('button', {name: 'Add a folder…'})).toBeNull()
        await waitFor(() => expect(bindings.StartPlanning).toHaveBeenCalledWith('board-1', '', 'planner'))
        setHosted(false)
    })

    // A conversation outlives the dialog, so opening it again has to find what
    // was said rather than starting a second one about the same thing.
    it('lists the conversations already open and reopens one', async () => {
        const bindings = planningBindings([{
            ...chat,
            messages: [
                {role: 'person', text: 'Надо починить логин'},
                {role: 'agent', text: 'Смотрю, что в форме'},
            ],
        }])
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'planner · app'}))

        expect(await screen.findByText('Надо починить логин')).toBeInTheDocument()
        expect(screen.getByText('Смотрю, что в форме')).toBeInTheDocument()
    })

    // What a person types is the conversation's next turn.
    it('sends what was typed as the next turn', async () => {
        const bindings = planningBindings([chat])
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'planner · app'}))
        await userEvent.type(screen.getByPlaceholderText('What has to be done?'), 'Почини логин')
        await userEvent.click(screen.getByRole('button', {name: 'Send'}))

        await waitFor(() => expect(bindings.SayToPlanning).toHaveBeenCalledWith('plan-1', 'Почини логин'))
    })

    // Ending a conversation ends the agent with it, which is why it is asked
    // about first.
    it('asks before ending a conversation, and then ends it', async () => {
        const bindings = planningBindings([chat])
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'End the conversation'}))
        expect(bindings.EndPlanning).not.toHaveBeenCalled()

        await userEvent.click(await screen.findByRole('button', {name: 'End'}))
        await waitFor(() => expect(bindings.EndPlanning).toHaveBeenCalledWith('plan-1'))
    })

    // A conversation that could not start says why, where it was asked for.
    it('says what went wrong instead of failing silently', async () => {
        const bindings = planningBindings()
        bindings.StartPlanning = vi.fn().mockRejectedValue(new Error('агент не зарегистрирован'))
        fakeGoSide(bindings)

        render(() => wrapIntl(() => <PlanningDialog board={board} onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByRole('button', {name: 'app'}))

        expect(await screen.findByText(/агент не зарегистрирован/)).toBeInTheDocument()
    })
})
