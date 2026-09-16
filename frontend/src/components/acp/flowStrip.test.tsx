import {render, screen, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import FlowStrip, {isFlowStripAvailable, waited} from './flowStrip'

const cardFlow = {
    flow: 'Feature',
    currentNodeId: 'review',
    since: new Date(Date.now() - (90 * 60 * 1000)).toISOString(),
    branch: 'feat/x',
    waitingFor: ['ветка влита в основную'],
    stages: [
        {nodeId: 'agent', column: 'In Progress', action: 'agent', crew: ['dev-1'], current: false, done: true},
        {nodeId: 'review', column: 'In Review', action: 'none', current: true, done: false},
        {nodeId: 'deploy', column: 'Deploy', action: 'deploy', current: false, done: false},
    ],
}

function stubBindings(flow: unknown) {
    const bindings = {
        GetCardFlow: vi.fn().mockResolvedValue(JSON.stringify(flow)),
        MarkCardOutcome: vi.fn().mockResolvedValue(undefined),
        AnswerCardAsk: vi.fn().mockResolvedValue(undefined),
    }
    fakeGoSide(bindings)
    return bindings
}

describe('components/acp/flowStrip', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    test('is unavailable without desktop bindings', () => {
        expect(isFlowStripAvailable()).toBe(false)
    })

    test('a stage entry that was never stamped has no age', () => {
        expect(waited('0001-01-01T00:00:00Z', Date.now())).toBeNull()
    })

    test('waited reports the wait in the units a person would say it in', () => {
        const now = Date.parse('2026-07-31T12:00:00Z')
        expect(waited('2026-07-31T11:40:00Z', now)).toEqual({value: 20, unit: 'minutes'})
        expect(waited('2026-07-31T09:00:00Z', now)).toEqual({value: 3, unit: 'hours'})
        expect(waited('2026-07-28T12:00:00Z', now)).toEqual({value: 3, unit: 'days'})
        expect(waited(undefined, now)).toBeNull()
        expect(waited('nonsense', now)).toBeNull()
    })

    test('shows the route, the stage the card is on and what it waits for', async () => {
        stubBindings(cardFlow)
        render(() => wrapIntl(() => <FlowStrip cardId='card1'/>))

        await waitFor(() => expect(screen.getByText('Feature')).toBeInTheDocument())
        expect(screen.getByText('feat/x')).toBeInTheDocument()
        for (const stage of cardFlow.stages) {
            expect(screen.getByText(stage.column)).toBeInTheDocument()
        }

        // The current stage is marked, so the strip answers "where is it" at a glance.
        expect(screen.getByText('In Review')).toHaveClass('FlowStrip__stage--current')
        expect(screen.getByText('In Progress')).toHaveClass('FlowStrip__stage--done')

        // And the one question a stalled card raises is answered outright,
        // together with how long it has been standing there.
        expect(screen.getByText(/ветка влита в основную/).textContent).toMatch(/1 h/)
    })

    test('a card waiting for a place in the column says so', async () => {
        stubBindings({...cardFlow, waitingFor: [], queued: true})
        render(() => wrapIntl(() => <FlowStrip cardId='card1'/>))
        await waitFor(() => expect(screen.getByText(/free place in the column/)).toBeInTheDocument())
    })

    test('a card with no route draws nothing', async () => {
        stubBindings(null)
        const {container} = render(() => wrapIntl(() => <FlowStrip cardId='card1'/>))
        await waitFor(() => expect(container).toBeEmptyDOMElement())
    })

    // the "agent did not start" comment used to be a comment on the card; it is the card's
    // current state, so the strip says it — verbatim, since the reason arrives
    // as a whole sentence — and drops it as soon as the machinery moves on.
    test('a card whose stage would not start says why on the strip', async () => {
        stubBindings({...cardFlow, waitingFor: [], stalled: 'агент не запущен: проект не найден в реестре'})
        const {container} = render(() => wrapIntl(() => <FlowStrip cardId='card1'/>))
        await waitFor(() => expect(screen.getByText(/агент не запущен: проект не найден/)).toBeInTheDocument())
        expect(container.querySelector('.FlowStrip__status--stalled')).not.toBeNull()
    })

    // Working outranks stalled: a stale reason must not shout over a session
    // that is already running.
    test('a running card never shows a stall reason', async () => {
        stubBindings({...cardFlow, waitingFor: [], running: true, stalled: 'агент не запущен'})
        render(() => wrapIntl(() => <FlowStrip cardId='card1'/>))
        await waitFor(() => expect(screen.getByText(/working now/)).toBeInTheDocument())
        expect(screen.queryByText(/агент не запущен/)).toBeNull()
    })

    // A stage where nobody works and nothing runs waits for a person to say how
    // it went, and saying so used to mean opening the card, finding «Исход» and
    // picking a value out of it. The route knows where each answer leads, so
    // the card asks it as two buttons named by the stage they lead to.
    test('a stage that waits for a person offers its two answers', async () => {
        const bindings = stubBindings({
            ...cardFlow,
            marks: [
                {value: 'не прошло', column: 'In Progress', forward: false},
                {value: 'прошло', column: 'Deploy', forward: true},
            ],
        })
        render(() => wrapIntl(() => <FlowStrip cardId='card-1'/>))

        const back = await screen.findByRole('button', {name: /In Progress/})
        expect(await screen.findByRole('button', {name: /Deploy/})).toBeInTheDocument()

        // The arrow says which way, so the value does not have to be on the
        // button — but it is what gets written.
        back.click()
        await waitFor(() => expect(bindings.MarkCardOutcome).toHaveBeenCalledWith('card-1', 'не прошло'))
    })

    // A transition may ask before it lets the card through, and the card is
    // where it is answered: the question is drawn where the two outcome buttons
    // are, and its answers are the same one-click chips.
    test('a card held at a transition asks its question on the card', async () => {
        const bindings = stubBindings({
            ...cardFlow,
            ask: {
                questionId: 'q1',
                text: 'Как агенту работать с папкой для этой карточки?',
                column: 'In Progress',
                options: [
                    {value: 'worktree', label: 'отдельное рабочее дерево'},
                    {value: 'branch', label: 'ветка в этой же папке'},
                ],
            },
        })
        render(() => wrapIntl(() => <FlowStrip cardId='card-1'/>))

        expect(await screen.findByText('Как агенту работать с папкой для этой карточки?')).toBeInTheDocument();
        (await screen.findByRole('button', {name: 'ветка в этой же папке'})).click()
        await waitFor(() => expect(bindings.AnswerCardAsk).toHaveBeenCalledWith('card-1', 'q1', 'branch'))
    })

    // The card goes nowhere until the question is answered, so the two outcome
    // buttons would be a second way to move it from under the question. Go
    // leaves them out; this is the card taking that answer at face value.
    test('a card being asked something offers no outcome buttons', async () => {
        stubBindings({
            ...cardFlow,
            ask: {questionId: 'q1', text: 'Куда?', column: 'Deploy', options: [{value: 'a', label: 'A'}]},
        })
        render(() => wrapIntl(() => <FlowStrip cardId='card-1'/>))

        await waitFor(() => expect(screen.getByText('Куда?')).toBeInTheDocument())
        expect(screen.queryByRole('button', {name: /In Progress/})).toBeNull()
    })

    // A stage that decides for itself offers nothing: the agent reports, and
    // two buttons beside a running session would be a second way to answer.
    test('a working stage offers no answers', async () => {
        stubBindings({...cardFlow, running: true, marks: [{value: 'прошло', column: 'Deploy', forward: true}]})
        const {container} = render(() => wrapIntl(() => <FlowStrip cardId='card-1'/>))

        await waitFor(() => expect(container.querySelector('.FlowStrip')).toBeInTheDocument())
        expect(container.querySelector('.FlowStrip__mark')).toBeNull()
    })
})
