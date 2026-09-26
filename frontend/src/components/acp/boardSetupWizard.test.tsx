import {fireEvent, render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {Board} from '../../blocks/board'
import {TestBlockFactory} from '../../test/testBlockFactory'
import {wrapIntl} from '../../testUtils'

import mutator from '../../mutator'

import {fakeGoSide} from '../../test/goSide'

import {SetupPlan, SetupStep, SetupStepKind} from './boardSetup'

import BoardSetupWizard, {readRegistry} from './boardSetupWizard'

vi.mock('../../mutator')
const mockedMutator = vi.mocked(mutator)

// One board for the whole file: its id is what the wizard reports a skip
// under, so a fresh one per call would compare against somebody else's.
const testBoard: Board = TestBlockFactory.createBoard()

// A plan is what Go answers with; the wizard's whole shape comes from it.
function plan(kinds: Array<SetupStepKind | Partial<SetupStep>>, extra: Partial<SetupPlan> = {}): string {
    const steps = kinds.map((kind) => {
        if (typeof kind === 'string') {
            return {kind, optional: kind === 'deploy', status: 'pending'}
        }
        return {kind: 'project', optional: false, status: 'pending', ...kind}
    })
    return JSON.stringify({boardId: testBoard.id, steps, declared: true, automated: true, ...extra})
}

// The developer board: it publishes, so it is asked everything.
const FULL_PLAN = plan(['project', 'agent', 'deploy', 'done'])

// A board of household chores: an agent and nothing else.
const CHORES_PLAN = plan([{kind: 'project', hint: 'Папка с домашними заметками'}, {kind: 'agent'}, {kind: 'done'}])

function stubBindings(overrides: Record<string, unknown> = {}) {
    const bindings = {
        BoardSetupPlan: vi.fn().mockResolvedValue(FULL_PLAN),
        RecordBoardSetupStep: vi.fn().mockResolvedValue(undefined),
        CheckBoardSetupAnswer: vi.fn().mockResolvedValue(undefined),
        ListAgentWorkdirs: vi.fn().mockResolvedValue('[]'),
        ListAgents: vi.fn().mockResolvedValue('[]'),
        PickDirectory: vi.fn().mockResolvedValue('/Users/me/src/webapp'),
        ListAgentAdapters: vi.fn().mockResolvedValue('[]'),
        AddAgentWorkdir: vi.fn().mockResolvedValue('{}'),
        AddAgent: vi.fn().mockResolvedValue('{}'),
        UpdateAgent: vi.fn().mockResolvedValue('{}'),
        SyncAgentUsers: vi.fn().mockResolvedValue('[]'),
        AddDeployTarget: vi.fn().mockResolvedValue('{}'),
        SeedBoardAutomation: vi.fn().mockResolvedValue(undefined),
        ...overrides,
    }
    fakeGoSide(bindings)
    return bindings
}

describe('components/acp/boardSetupWizard', () => {
    afterEach(() => {
        fakeGoSide()
        localStorage.clear()
        vi.clearAllMocks()
    })

    const renderWizard = (onClose = vi.fn()) => render(() => wrapIntl(() =>
        <BoardSetupWizard
            board={testBoard}
            onClose={onClose}
        />,
    ))

    // The one question that is not about the machine, and the one nobody may be
    // let past: a board arrives called what its template is called, so the
    // second the developer template is a second the developer template in the sidebar.
    describe("the board's name", () => {
        test('is the first step, and is not skippable', async () => {
            stubBindings({BoardSetupPlan: vi.fn().mockResolvedValue(plan(['name', 'project', 'agent', 'done']))})
            renderWizard()

            await waitFor(() => expect(screen.getByLabelText('Board name')).toBeInTheDocument())
            expect(screen.queryByRole('button', {name: 'Skip'})).toBeNull()
        })

        test('renames the board and records the step', async () => {
            const bindings = stubBindings({
                BoardSetupPlan: vi.fn().mockResolvedValue(plan(['name', 'project', 'done'])),
            })
            renderWizard()

            const field = await screen.findByLabelText('Board name')
            fireEvent.input(field, {target: {value: '  Ремонт кухни  '}})
            userEvent.click(screen.getByRole('button', {name: 'Next'}))

            // The record is the last thing the answer does, so waiting for it
            // waits for the rename too. Trimmed, and filed against the board
            // being set up; the old title is whatever it arrived with.
            await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'name', 'done'))
            expect(mockedMutator.changeBoardTitle).toHaveBeenCalledWith(testBoard.id, expect.any(String), 'Ремонт кухни')
        })

        // Asked with the name, because the inbox sorter decides by it and the
        // moment a board is made is the moment somebody knows what it is for.
        test('saves what the board is for along with its name', async () => {
            const bindings = stubBindings({
                BoardSetupPlan: vi.fn().mockResolvedValue(plan(['name', 'project', 'done'])),
            })
            renderWizard()

            const field = await screen.findByLabelText('Description')
            fireEvent.input(field, {target: {value: '  Ремонт: материалы, мастера, сроки  '}})
            userEvent.click(screen.getByRole('button', {name: 'Next'}))

            await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'name', 'done'))
            expect(mockedMutator.changeBoardDescription).toHaveBeenCalledWith(testBoard.id, testBoard.id, testBoard.description, 'Ремонт: материалы, мастера, сроки')
        })

        test('refuses a name another board already has', async () => {
            stubBindings({
                BoardSetupPlan: vi.fn().mockResolvedValue(plan(['name', 'project', 'done'])),
                ListBoards: vi.fn().mockResolvedValue(JSON.stringify([
                    {id: 'other-board', title: 'Разработка'},
                ])),
            })
            renderWizard()

            const field = await screen.findByLabelText('Board name')
            fireEvent.input(field, {target: {value: 'разработка'}})

            await waitFor(() => expect(screen.getByText(/Another board is already called that/)).toBeInTheDocument())
            expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled()
            expect(mockedMutator.changeBoardTitle).not.toHaveBeenCalled()
        })

        test('an empty name is no answer either', async () => {
            stubBindings({BoardSetupPlan: vi.fn().mockResolvedValue(plan(['name', 'done']))})
            renderWizard()

            const field = await screen.findByLabelText('Board name')
            fireEvent.input(field, {target: {value: '   '}})
            expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled()
        })
    })

    test('outside the desktop app there is no registry to read', async () => {
        expect(await readRegistry()).toBeNull()
    })

    test('the project step will not pass until there is one', async () => {
        // The registry answers empty until the folder is added, and with it
        // afterwards — which is what the wizard reads back to put it on the
        // board's own field.
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().
                mockResolvedValueOnce('[]').
                mockResolvedValue(JSON.stringify([{name: 'webapp', path: '/Users/me/src/webapp'}])),
        })
        renderWizard()
        await waitFor(() => expect(bindings.ListAgentWorkdirs).toHaveBeenCalled())

        expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled()

        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('webapp')).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Next'}))

        // Filed against the board being set up: the wizard is that board's.
        await waitFor(() => expect(bindings.AddAgentWorkdir).toHaveBeenCalledWith('webapp', '/Users/me/src/webapp', testBoard.id, '', false))

        // And onto the board's own field, or the person who has just answered
        // "which folder" has nothing to pick on the card: a card names its
        // folder with an option of that field and with nothing else.
        await waitFor(() => expect(mockedMutator.updateBoardCardProperties).toHaveBeenCalled())
        const added = mockedMutator.updateBoardCardProperties.mock.calls[0][2].
            flatMap((p: {options?: Array<{value: string}>}) => p.options || []).
            map((o: {value: string}) => o.value)
        expect(added).toContain('webapp')

        // And having added it, the wizard is on the agent step.
        await waitFor(() => expect(screen.getByText('Kind')).toBeInTheDocument())
    })

    // The name is filled in from the folder, so changing the folder has to
    // refill it: a second choice registered under the first one's name is a
    // folder nobody can find by what it is called.
    test('picking another folder renames it with it, unless the name was typed', async () => {
        const bindings = stubBindings({
            PickDirectory: vi.fn().
                mockResolvedValueOnce('/Users/me/src/webapp').
                mockResolvedValueOnce('/Users/me/src/server').
                mockResolvedValue('/Users/me/src/mobile'),
        })
        renderWizard()
        await waitFor(() => expect(bindings.ListAgentWorkdirs).toHaveBeenCalled())

        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('webapp')).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('server')).toBeInTheDocument())

        // A name somebody typed is theirs and survives the next pick.
        fireEvent.input(screen.getByDisplayValue('server'), {target: {value: 'мой сервер'}})
        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('мой сервер')).toBeInTheDocument())
    })

    test('a refusal from Go is shown rather than swallowed', async () => {
        const bindings = stubBindings({AddAgentWorkdir: vi.fn().mockRejectedValue('/Users/me/src не является git-проектом')})
        renderWizard()
        await waitFor(() => expect(bindings.ListAgentWorkdirs).toHaveBeenCalled())

        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('webapp')).toBeInTheDocument())
        userEvent.click(screen.getByRole('button', {name: 'Next'}))

        await waitFor(() => expect(screen.getByText(/не является git-проектом/)).toBeInTheDocument())

        // And it stays on the step, rather than walking on past the problem.
        expect(screen.getByRole('button', {name: 'Choose a folder…'})).toBeInTheDocument()
    })

    test('an agent is registered and made assignable', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'webapp', path: '/src'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(
                plan([{kind: 'project', status: 'done'}, {kind: 'agent'}, {kind: 'done'}]),
            ),
        })

        // Registered, so the registry has it from then on — which is what makes
        // it assignable on the board.
        bindings.AddAgent.mockImplementation(async () => {
            bindings.ListAgents.mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}]))
            return '{}'
        })
        renderWizard()

        // The project is answered, so the wizard opens on the question that is
        // not — asked by the same short form a card offers.
        await waitFor(() => expect(screen.getByText('Kind')).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Add'}))
        await waitFor(() => expect(bindings.AddAgent).toHaveBeenCalled())
        expect(JSON.parse(bindings.AddAgent.mock.calls[0][0])).toEqual({name: 'claude', kind: 'claude'})
        await waitFor(() => expect(bindings.SyncAgentUsers).toHaveBeenCalledWith(testBoard.id))
        await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'agent', 'done'))
    })

    // The machine being configured is not this board being set up: passing a
    // step because a project is already registered still answers it, for this
    // board, or the second board you make is created in silence.
    test('a step passed because the machine already has one is answered too', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'webapp', path: '/src'}])),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([
                {kind: 'project', ready: true},
                {kind: 'agent', ready: true},
                {kind: 'done'},
            ])),
        })
        renderWizard()
        await waitFor(() => expect(screen.getByRole('button', {name: 'Next'})).toBeEnabled())

        userEvent.click(screen.getByRole('button', {name: 'Next'}))
        await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'project', 'done'))
        expect(bindings.AddAgentWorkdir).not.toHaveBeenCalled()

        // The machine already has an agent, so the step says so and offers Next
        // rather than a form. ("claude" is the agent; the project is "webapp".)
        await waitFor(() => expect(screen.getByText('claude')).toBeInTheDocument())
        userEvent.click(screen.getByRole('button', {name: 'Next'}))
        await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'agent', 'done'))
        expect(bindings.AddAgent).not.toHaveBeenCalled()
    })

    // The step that lists what is already registered is also the step to add
    // another one on: the alternative was an errand to the settings for the one
    // thing this screen is about.
    test('another agent can be added on a step that already has one', async () => {
        const bindings = stubBindings({
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'клаус'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([{kind: 'agent', ready: true}, {kind: 'done'}])),
        })
        bindings.AddAgent.mockImplementation(async () => {
            bindings.ListAgents.mockResolvedValue(JSON.stringify([{name: 'клаус'}, {name: 'тестер'}]))
            return '{}'
        })
        renderWizard()

        await waitFor(() => expect(screen.getByText('клаус')).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Add an agent…'}))
        await waitFor(() => expect(screen.getByText('Kind')).toBeInTheDocument())
        await userEvent.clear(screen.getByDisplayValue('claude'))
        await userEvent.type(screen.getByRole('textbox'), 'тестер')
        userEvent.click(screen.getByRole('button', {name: 'Add'}))

        await waitFor(() => expect(screen.getByText('тестер')).toBeInTheDocument())
        expect(screen.getByText('клаус')).toBeInTheDocument()

        // Adding one is not answering the step: «Next» is what moves on, so a
        // second agent can be added straight after the first.
        expect(bindings.RecordBoardSetupStep).not.toHaveBeenCalledWith(testBoard.id, 'agent', 'done')
        expect(screen.getByRole('button', {name: 'Next'})).toBeInTheDocument()
    })

    // Who works a card is the card's own «кто занимается», so this step asks
    // nothing: it is answered by an agent existing on the machine at all, and
    // walking past it writes nothing anywhere.
    test('the agent step registers nothing and answers itself', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'webapp', path: '/src'}])),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'клаус'}, {name: 'тестер'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([{kind: 'agent', ready: true}, {kind: 'done'}])),
        })
        renderWizard()

        // Both are shown, because both may be assigned a card here — the
        // registry is the machine's and no board keeps a crew of its own.
        await waitFor(() => expect(screen.getByText('клаус')).toBeInTheDocument())
        expect(screen.getByText('тестер')).toBeInTheDocument()

        userEvent.click(screen.getByRole('button', {name: 'Next'}))
        await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'agent', 'done'))
        expect(bindings.AddAgent).not.toHaveBeenCalled()
    })

    // The folder is the one question somebody may not be able to answer at the
    // moment they are asked, and the next button stays disabled until it is answered —
    // so without this the only way past the first screen was out of the wizard.
    test('the folder step can be passed over, and the pass is remembered', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue('[]'),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'клаус'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(plan(['project', 'agent', 'done'])),
        })
        renderWizard()

        // The plan is what says which step comes after this one, so the wizard
        // has to have it before a skip can go anywhere.
        await waitFor(() => expect(screen.getByText('Ready')).toBeInTheDocument())

        // Nothing has been picked, so there is nothing to accept — and the way
        // on is the skip beside it.
        expect(screen.getByRole('button', {name: 'Next'})).toBeDisabled()
        userEvent.click(screen.getByRole('button', {name: 'Skip'}))

        await waitFor(() => expect(screen.getByText('клаус')).toBeInTheDocument())
        expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'project', 'skipped')
        expect(bindings.AddAgentWorkdir).not.toHaveBeenCalled()
    })

    test('deploying is skippable, and skipping is remembered', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'webapp', path: '/src'}])),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([
                {kind: 'project', status: 'done'},
                {kind: 'agent', status: 'done'},
                {kind: 'deploy', optional: true},
                {kind: 'done'},
            ])),
        })
        const onClose = vi.fn()
        renderWizard(onClose)

        await waitFor(() => expect(screen.getByText('Dokku host')).toBeInTheDocument())
        userEvent.click(screen.getByRole('button', {name: 'Skip'}))

        await waitFor(() => expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument())
        expect(bindings.AddDeployTarget).not.toHaveBeenCalled()

        // A skip is the one answer no registry can be read for later, so it is
        // the one the app has to remember.
        expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'deploy', 'skipped')

        userEvent.click(screen.getByRole('button', {name: 'Done'}))
        await waitFor(() => expect(bindings.SeedBoardAutomation).toHaveBeenCalled())
        expect(onClose).toHaveBeenCalled()
    })

    test('a board is asked only what it asked to be asked', async () => {
        const bindings = stubBindings({
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([{name: 'notes', path: '/src'}])),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude'}])),
            BoardSetupPlan: vi.fn().mockResolvedValue(CHORES_PLAN),
        })
        renderWizard()

        // The board's own sentence about the step it asks for is shown, and it
        // is also what says the plan has arrived.
        await waitFor(() => expect(screen.getByText('Папка с домашними заметками')).toBeInTheDocument())

        // The question this board cannot answer is not even listed.
        expect(screen.queryByText('Deploy')).toBeNull()

        userEvent.click(screen.getByRole('button', {name: 'Next'}))
        await waitFor(() => expect(bindings.RecordBoardSetupStep).toHaveBeenCalledWith(testBoard.id, 'project', 'done'))

        // The machine already has an agent, so this step says so rather than
        // asking again. ("claude" is the agent; the project is "notes".)
        await waitFor(() => expect(screen.getByText('claude')).toBeInTheDocument())
        userEvent.click(screen.getByRole('button', {name: 'Next'}))

        // Straight to the end, and the end names this board's own column.
        await waitFor(() => expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument())
        expect(bindings.AddDeployTarget).not.toHaveBeenCalled()
    })

    // Git is asked for by what the board does, and the answer is checked where
    // it is given: a folder that will not do must not be filed and found out
    // about later, on a card, when a deploy fails.
    test('a board that needs git says so, and refuses a folder without it', async () => {
        const bindings = stubBindings({
            BoardSetupPlan: vi.fn().mockResolvedValue(
                plan([{kind: 'project', requires: ['git']}, {kind: 'agent'}, {kind: 'done'}]),
            ),
            CheckBoardSetupAnswer: vi.fn().mockRejectedValue('в каталоге /Users/me/src/webapp нет git-репозитория, а этой доске он нужен'),
        })
        renderWizard()
        await waitFor(() => expect(screen.getByText(/has to be a git repository/)).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Choose a folder…'}))
        await waitFor(() => expect(screen.getByDisplayValue('webapp')).toBeInTheDocument())
        userEvent.click(screen.getByRole('button', {name: 'Next'}))

        await waitFor(() => expect(screen.getByText(/нет git-репозитория/)).toBeInTheDocument())
        expect(bindings.AddAgentWorkdir).not.toHaveBeenCalled()
    })

    // …and a board of personal tasks is never told about git at all.
    test('a board that does not need git never mentions it', async () => {
        stubBindings({BoardSetupPlan: vi.fn().mockResolvedValue(CHORES_PLAN)})
        renderWizard()
        await waitFor(() => expect(screen.getByText('Папка с домашними заметками')).toBeInTheDocument())
        expect(screen.queryByText(/has to be a git repository/)).toBeNull()
    })

    // Leaving early is offered while there is something left to answer. On the
    // last screen there is not, and the finish button beside the leave-early
    // one read as two ways out of a wizard that had already finished.
    test('the last screen offers only Done, and the ones before it offer to leave', async () => {
        stubBindings({
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([{kind: 'deploy', optional: true}, {kind: 'done'}])),
        })
        const early = renderWizard()

        await waitFor(() => expect(screen.getByRole('button', {name: 'Skip'})).toBeInTheDocument())
        expect(screen.getByRole('button', {name: 'I’ll find my way around'})).toBeInTheDocument()
        early.unmount()

        stubBindings({
            BoardSetupPlan: vi.fn().mockResolvedValue(plan([{kind: 'deploy', status: 'skipped', optional: true}, {kind: 'done'}])),
        })
        renderWizard()

        await waitFor(() => expect(screen.getByRole('button', {name: 'Done'})).toBeInTheDocument())
        expect(screen.queryByRole('button', {name: 'I’ll find my way around'})).toBeNull()
    })

    // The last page is where the product is shown rather than described: one
    // sentence becomes a card an agent starts on (docs/onboarding.md).
    describe('the last page', () => {
        const renderLast = (props: Partial<Parameters<typeof BoardSetupWizard>[0]> = {}) => {
            const onClose = vi.fn()
            render(() => wrapIntl(() =>
                <BoardSetupWizard
                    board={testBoard}
                    onClose={onClose}
                    {...props}
                />,
            ))
            return onClose
        }

        test('offers a first task for the agent just registered, and hands it over', async () => {
            stubBindings({
                BoardSetupPlan: vi.fn().mockResolvedValue(plan(['done'])),
                ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'Claude', kind: 'claude'}])),
            })
            const start = vi.fn().mockResolvedValue(undefined)
            const onClose = renderLast({firstTask: {column: 'In progress', start}})

            const field = await screen.findByLabelText('Your first task, in one sentence')
            expect(screen.getByText(/moves it into “In progress”/)).toBeInTheDocument()
            fireEvent.input(field, {target: {value: '  Fix the build  '}})
            userEvent.click(screen.getByRole('button', {name: 'Set the first task'}))

            await waitFor(() => expect(start).toHaveBeenCalledWith('Fix the build', 'Claude'))
            await waitFor(() => expect(onClose).toHaveBeenCalled())
        })

        // An offer that fails because nothing is installed is worse than none:
        // the page points at the board's own explanation instead.
        test('without an agent offers no first task, and opens the first card of the board', async () => {
            stubBindings({BoardSetupPlan: vi.fn().mockResolvedValue(plan(['done']))})
            const open = vi.fn()
            const onClose = renderLast({
                firstTask: {column: 'In progress', start: vi.fn()},
                readFirst: {title: 'Where to start', open},
            })

            const button = await screen.findByRole('button', {name: 'Open “Where to start”'})
            expect(screen.queryByLabelText('Your first task, in one sentence')).toBeNull()
            userEvent.click(button)

            await waitFor(() => expect(open).toHaveBeenCalled())
            expect(onClose).toHaveBeenCalled()
        })

        test('starts the tour on this board', async () => {
            stubBindings({BoardSetupPlan: vi.fn().mockResolvedValue(plan(['done']))})
            const onShowTour = vi.fn()
            const onClose = renderLast({onShowTour})

            userEvent.click(await screen.findByRole('button', {name: 'Show how the board works'}))

            await waitFor(() => expect(onShowTour).toHaveBeenCalled())
            expect(onClose).toHaveBeenCalled()
        })
    })
})
