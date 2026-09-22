import {fireEvent, render, screen} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {TestRouter, mockAppStore, wrapIntl} from './testUtils'
import {AppStoreProvider} from './store'
import {fakeGoSide} from './test/goSide'
import {TestBlockFactory} from './test/testBlockFactory'
import {UserSettingKey} from './userSettings'
import {aiEnabled, setAIEnabled} from './aiSwitch'
import {isAutomationAvailable} from './components/acp/bindings'
import {isCardTerminalAvailable} from './components/acp/liveTerminals'
import {NO_SETUP, withoutAgents} from './components/acp/boardSetup'
import {withoutAgentProperties} from './components/acp/agentProperties'
import {BOARD_PROP_OUTCOME_PROPERTY, BOARD_PROP_PROJECT_PROPERTY} from './components/acp/automation'
import AppPanel from './components/settings/appPanel'

describe('the AI switch', () => {
    afterEach(() => {
        localStorage.clear()
        fakeGoSide()
    })

    // An install that has always run agents must not lose them to an upgrade:
    // nobody has said no yet.
    test('an install that never touched it runs agents', () => {
        expect(aiEnabled()).toBe(true)
    })

    // Off is the install's decision, so it is kept by the install and not by
    // the browser, whose origin changes every launch.
    test('switching it off is kept by the install', () => {
        const SetUIPreference = vi.fn().mockResolvedValue(undefined)
        fakeGoSide({SetUIPreference})

        setAIEnabled(false)

        expect(aiEnabled()).toBe(false)
        expect(SetUIPreference).toHaveBeenCalledWith(UserSettingKey.AIEnabled, 'false')
    })

    // Off, the app is a kanban board: the routes editor and the card's
    // terminal are not offered even though the build has them.
    test('switched off, nothing about agents is offered', () => {
        fakeGoSide({ListFlows: vi.fn(), OpenCardTerminal: vi.fn(), SetUIPreference: vi.fn().mockResolvedValue(undefined)})
        expect(isAutomationAvailable()).toBe(true)
        expect(isCardTerminalAvailable()).toBe(true)

        setAIEnabled(false)

        expect(isAutomationAvailable()).toBe(false)
        expect(isCardTerminalAvailable()).toBe(false)
    })

    // The board's name is a question about the board; the rest of the setup is
    // about where agents work and who they are, and a board that is not
    // automated never opens the wizard by itself.
    test('switched off, the setup asks only for the board’s name', () => {
        const plan = withoutAgents({
            ...NO_SETUP,
            automated: true,
            steps: [
                {kind: 'name', optional: false, status: 'pending'},
                {kind: 'project', optional: false, status: 'pending'},
                {kind: 'agent', optional: false, status: 'pending'},
            ],
        })
        expect(plan.steps.map((s) => s.kind)).toEqual(['name'])
        expect(plan.automated).toBe(false)
    })

    // The folder and the outcome are the agents' fields; the board's own stay.
    // Hidden, not removed: switching back on shows them again.
    test('switched off, a card does not show the agents’ fields', () => {
        const board = TestBlockFactory.createBoard()
        const own = {id: 'own', name: 'Own', type: 'text' as const, options: []}
        const folder = {id: 'folder', name: 'Folder', type: 'select' as const, options: []}
        const outcome = {id: 'outcome', name: 'Outcome', type: 'select' as const, options: []}
        board.cardProperties = [own, folder, outcome]
        board.properties = {...board.properties, [BOARD_PROP_PROJECT_PROPERTY]: folder.id, [BOARD_PROP_OUTCOME_PROPERTY]: outcome.id}

        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id, folder.id, outcome.id])

        fakeGoSide({SetUIPreference: vi.fn().mockResolvedValue(undefined)})
        setAIEnabled(false)
        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id])

        setAIEnabled(true)
        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id, folder.id, outcome.id])
    })

    // The switch is in the app's own settings, and only where the app can run
    // agents at all.
    test('is switched from the app’s settings', () => {
        fakeGoSide({ListAgents: vi.fn(), SetUIPreference: vi.fn().mockResolvedValue(undefined)})
        const {container} = render(() => wrapIntl(() => (
            <AppStoreProvider store={mockAppStore({})}>
                <TestRouter>
                    <AppPanel/>
                </TestRouter>
            </AppStoreProvider>
        )))

        expect(screen.getByText('Agents work on the cards')).toBeInTheDocument()
        fireEvent.click(container.querySelector('.AppPanel__action .Switch') as HTMLElement)
        expect(aiEnabled()).toBe(false)
    })

    test('is not offered where there are no agents to switch off', () => {
        render(() => wrapIntl(() => (
            <AppStoreProvider store={mockAppStore({})}>
                <TestRouter>
                    <AppPanel/>
                </TestRouter>
            </AppStoreProvider>
        )))

        expect(screen.queryByText('Agents work on the cards')).not.toBeInTheDocument()
    })
})
