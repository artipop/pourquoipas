import {fireEvent, render, screen} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {wrapIntl} from './testUtils'
import {fakeGoSide} from './test/goSide'
import {TestBlockFactory} from './test/testBlockFactory'
import {UserSettingKey, UserSettings} from './userSettings'
import {aiEnabled, setAIEnabled} from './aiSwitch'
import {isPlanningAvailable} from './components/acp/planningDialog'
import {withoutAgentProperties} from './components/acp/agentProperties'
import {BOARD_PROP_BRANCH_PROPERTY, BOARD_PROP_PROJECT_PROPERTY} from './components/acp/automation'
import AIPanel, {isAIPanelAvailable} from './components/acp/aiPanel'

describe('the AI switch', () => {
    afterEach(() => {
        UserSettings.set(UserSettingKey.AIEnabled, null)
        fakeGoSide()
    })

    // An install that has always run agents must not lose them to an upgrade:
    // nobody has said no yet.
    test('an install that never touched it runs agents', () => {
        expect(aiEnabled()).toBe(true)
    })

    // Off is the install's decision: the agents run on this machine, and a
    // person signed in elsewhere has no say over them.
    test('switching it off is kept by the install', () => {
        const SetMachineSetting = vi.fn().mockResolvedValue(undefined)
        fakeGoSide({SetMachineSetting})

        setAIEnabled(false)

        expect(aiEnabled()).toBe(false)
        expect(SetMachineSetting).toHaveBeenCalledWith(UserSettingKey.AIEnabled, false)
    })

    // Off, the app is a kanban board: talking a task over with an agent is not
    // offered even though the build has it.
    test('switched off, nothing about agents is offered', () => {
        fakeGoSide({StartPlanning: vi.fn(), SetMachineSetting: vi.fn().mockResolvedValue(undefined)})
        expect(isPlanningAvailable()).toBe(true)

        setAIEnabled(false)

        expect(isPlanningAvailable()).toBe(false)
    })

    // The folder and the branch are the agents' fields; the board's own stay.
    // Hidden, not removed: switching back on shows them again.
    test('switched off, a card does not show the agents’ fields', () => {
        const board = TestBlockFactory.createBoard()
        const own = {id: 'own', name: 'Own', type: 'text' as const, options: []}
        const folder = {id: 'folder', name: 'Folder', type: 'select' as const, options: []}
        const branch = {id: 'branch', name: 'Branch', type: 'text' as const, options: []}
        board.cardProperties = [own, folder, branch]
        board.properties = {...board.properties, [BOARD_PROP_PROJECT_PROPERTY]: folder.id, [BOARD_PROP_BRANCH_PROPERTY]: branch.id}

        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id, folder.id, branch.id])

        fakeGoSide({SetMachineSetting: vi.fn().mockResolvedValue(undefined)})
        setAIEnabled(false)
        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id])

        setAIEnabled(true)
        expect(withoutAgentProperties(board, board.cardProperties).map((t) => t.id)).toEqual([own.id, folder.id, branch.id])
    })

    // The switch is in the AI section of the settings, and only where the app
    // can run agents at all.
    test('is switched from the app’s settings', () => {
        fakeGoSide({ListAgents: vi.fn(), SetMachineSetting: vi.fn().mockResolvedValue(undefined), AIProviders: vi.fn().mockResolvedValue([])})
        const {container} = render(() => wrapIntl(() => <AIPanel/>))

        expect(screen.getByText('Agents work on the cards')).toBeInTheDocument()
        fireEvent.click(container.querySelector('.AppPanel__action .Switch') as HTMLElement)
        expect(aiEnabled()).toBe(false)
    })

    test('is not offered where there are no agents to switch off', () => {
        expect(isAIPanelAvailable()).toBe(false)
    })
})
