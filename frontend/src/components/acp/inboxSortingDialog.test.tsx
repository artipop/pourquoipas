import {fireEvent, render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import InboxSortingDialog, {isInboxSortingAvailable} from './inboxSortingDialog'

const renderDialog = () => render(() => wrapIntl(() => (
    <InboxSortingDialog
        onClose={() => undefined}
    />
)))

const state = (over: Record<string, unknown> = {}) => JSON.stringify({
    enabled: false,
    agent: 'claude',
    available: true,
    boards: [
        {id: 'home', title: 'Дом', description: 'Покупки и ремонт', summary: 'Бытовые дела по квартире.', cardCount: 3},
    ],
    decisions: [
        {id: '1', cardTitle: 'Счёт за свет', boardId: 'home', outcome: 'moved', reason: {text: 'Счёт за квартиру'}, at: '2026-09-22T10:00:00Z'},
    ],
    ...over,
})

describe('components/acp/inboxSortingDialog', () => {
    beforeEach(() => vi.clearAllMocks())

    afterEach(() => {
        fakeGoSide()
    })

    it('is inert without desktop bindings', () => {
        expect(isInboxSortingAvailable()).toBe(false)
    })

    // What the agent decides from is on the screen, and so is what it decided:
    // somebody whose card went to the wrong board has to be able to see why.
    it('shows each board’s description and summary, and where cards went', async () => {
        fakeGoSide({
            GetInboxSorting: vi.fn().mockResolvedValue(state()),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}])),
        })

        renderDialog()

        expect(await screen.findByDisplayValue('Покупки и ремонт')).toBeInTheDocument()
        expect(screen.getByText('Бытовые дела по квартире.')).toBeInTheDocument()
        expect(screen.getByText('Счёт за свет')).toBeInTheDocument()
        expect(screen.getByText('Счёт за квартиру')).toBeInTheDocument()
    })

    it('switches sorting on with the agent that is chosen', async () => {
        const SetInboxSorting = vi.fn().mockResolvedValue('{}')
        fakeGoSide({
            GetInboxSorting: vi.fn().mockResolvedValue(state()),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}])),
            SetInboxSorting,
        })

        renderDialog()

        const box = await screen.findByRole('checkbox')
        await waitFor(() => expect(box).not.toBeDisabled())
        await userEvent.click(box)

        await waitFor(() => expect(SetInboxSorting).toHaveBeenCalledWith(JSON.stringify({enabled: true, agent: 'claude'})))
    })

    // Sorting with nobody to sort is a switch that does nothing, so it cannot
    // be flipped until an agent is chosen.
    it('does not offer to sort until an agent is chosen', async () => {
        fakeGoSide({
            GetInboxSorting: vi.fn().mockResolvedValue(state({agent: ''})),
            ListAgents: vi.fn().mockResolvedValue('[]'),
        })

        renderDialog()

        expect(await screen.findByRole('checkbox')).toBeDisabled()
        expect(screen.getByRole('button', {name: 'Sort now'})).toBeDisabled()
    })

    it('saves a description when the field is left', async () => {
        const SetBoardDescription = vi.fn().mockResolvedValue(undefined)
        fakeGoSide({
            GetInboxSorting: vi.fn().mockResolvedValue(state()),
            ListAgents: vi.fn().mockResolvedValue('[]'),
            SetBoardDescription,
        })

        renderDialog()

        const field = await screen.findByDisplayValue('Покупки и ремонт')
        fireEvent.input(field, {target: {value: 'Покупки, ремонт, счета'}})
        fireEvent.blur(field)

        await waitFor(() => expect(SetBoardDescription).toHaveBeenCalledWith('home', 'Покупки, ремонт, счета'))
    })

    it('sorts now and says what came of it', async () => {
        fakeGoSide({
            GetInboxSorting: vi.fn().mockResolvedValue(state()),
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}])),
            SortInboxNow: vi.fn().mockResolvedValue(JSON.stringify({moved: 2, kept: 1, failed: 0})),
        })

        renderDialog()

        const button = await screen.findByRole('button', {name: 'Sort now'})
        await waitFor(() => expect(button).not.toBeDisabled())
        await userEvent.click(button)

        expect(await screen.findByText('Moved: 2. Left in the inbox: 1. Failed: 0.')).toBeInTheDocument()
    })
})
