import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'

import {wrapIntl} from '../../testUtils'

import AskDialog from './askDialog'
import {Attention} from './attention'
import {forgetMovedHere, movedHere} from './movedHere'

const waits: Attention[] = []
const answerCardAsk = vi.fn().mockResolvedValue(undefined)

vi.mock('./attention', async () => {
    const real = await vi.importActual<typeof import('./attention')>('./attention')
    return {...real, useAttention: () => () => waits}
})

vi.mock('./bindings', () => ({agentBindings: () => ({AnswerCardAsk: answerCardAsk})}))

const held = (over: Partial<Attention> = {}): Attention => ({
    key: 'card:card1',
    cardId: 'card1',
    boardId: 'board1',
    title: 'Почини логин',
    reason: 'card',
    text: 'Как агенту работать с папкой для этой карточки?',
    options: [{id: 'worktree', label: 'своя копия папки'}, {id: 'branch', label: 'ветка в самой папке'}],
    awaiting: true,
    ...over,
})

beforeEach(() => {
    waits.length = 0
    forgetMovedHere()
    answerCardAsk.mockClear()
})

// The case the question was built for: a person drags a card into work and the
// route wants to know how the work is arranged. They are holding the card, so
// they are asked here rather than sent back to find it.
it('asks the person who moved the card, right there', async () => {
    movedHere('card1')
    waits.push(held())
    render(() => wrapIntl(() => <AskDialog/>))

    await waitFor(() => expect(screen.getByText('Как агенту работать с папкой для этой карточки?')).toBeInTheDocument())
    await userEvent.click(screen.getByText('своя копия папки'))

    // No question is named: the wait is keyed by the card, and one question is
    // asked at a time.
    expect(answerCardAsk).toHaveBeenCalledWith('card1', '', 'worktree')
})

// The route moves cards by itself, and then there may be nobody at the screen.
// A dialog over whatever somebody is doing is the wrong way to say so — that
// wait is the card's strip and the notification, as it always was.
it('says nothing about a card this window did not move', async () => {
    waits.push(held())
    render(() => wrapIntl(() => <AskDialog/>))

    await new Promise((r) => setTimeout(r, 0))
    expect(screen.queryByText('Как агенту работать с папкой для этой карточки?')).toBeNull()
})

// A card parked on a stage where nobody works is a handover with nothing to
// answer. Being told is the whole of it, and a dialog with no buttons would be
// a box in the way.
it('says nothing about a handover with no answers to offer', async () => {
    movedHere('card1')
    waits.push(held({options: [], text: 'карточка ждёт человека'}))
    render(() => wrapIntl(() => <AskDialog/>))

    await new Promise((r) => setTimeout(r, 0))
    expect(screen.queryByText('карточка ждёт человека')).toBeNull()
})

// Closing is not an answer. The card goes on waiting where it is, which is what
// its own strip shows — so nothing is lost and nothing is decided.
it('closes without answering, and asks nothing of the route', async () => {
    movedHere('card1')
    waits.push(held())
    render(() => wrapIntl(() => <AskDialog/>))

    await waitFor(() => expect(screen.getByText('Answer on the card')).toBeInTheDocument())
    await userEvent.click(screen.getByText('Answer on the card'))

    await waitFor(() => expect(screen.queryByText('Как агенту работать с папкой для этой карточки?')).toBeNull())
    expect(answerCardAsk).not.toHaveBeenCalled()
})
