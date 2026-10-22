import {render, screen, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'
import {afterEach, describe, expect, test, vi} from 'vitest'

import userEvent from '@testing-library/user-event'

import {wrapIntl} from '../../testUtils'
import {fakeGoSide} from '../../test/goSide'
import {setHosted} from '../../buildKind'
import {isWorkdirsAvailable} from '../acp/workdirsPanel'
import {agentKindOptions} from '../acp/agentsPanel'

import AIPanel from './aiPanel'

describe('components/settings/aiPanel', () => {
    afterEach(() => setHosted(false))

    // A key goes to the Go side and is never shown again: the panel only says
    // whether there is one.
    test('saves a key without ever showing one', async () => {
        const setKey = vi.fn().mockResolvedValue({agent: 'OpenAI'})
        fakeGoSide({
            ListAgents: vi.fn(),
            AIProviders: vi.fn().mockResolvedValue([
                {id: 'anthropic', keySet: true, keyHint: '…ab12'},
                {id: 'openai', keySet: false},
            ]),
            SetAIProviderKey: setKey,
        })
        render(() => wrapIntl(() => <AIPanel/>))

        await waitFor(() => expect(screen.getByText('…ab12')).toBeInTheDocument())
        expect(screen.getByText('No key')).toBeInTheDocument()

        const keys = screen.getAllByLabelText('API key') as HTMLInputElement[]
        expect(keys.every((input) => input.value === '' && input.type === 'password')).toBe(true)

        userEvent.type(keys[1], 'sk-new')
        userEvent.click(screen.getAllByRole('button', {name: 'Check and save'})[1])
        await waitFor(() => expect(setKey).toHaveBeenCalledWith('openai', 'sk-new', ''))
        await waitFor(() => expect(screen.getByText('The provider accepted the key.')).toBeInTheDocument())

        // A key nothing can pick looks broken; the first one makes an agent,
        // and the panel says where it is offered.
        expect(screen.getByText(/The agent «OpenAI» is made on this key/)).toBeInTheDocument()
    })

    // A hosted server has no machine to work on: no folders, and the only
    // kind of agent is a model.
    test('a hosted server offers no folders and only model agents', () => {
        fakeGoSide({ListAgents: vi.fn(), ListAgentWorkdirs: vi.fn()})
        expect(isWorkdirsAvailable()).toBe(true)
        expect(agentKindOptions('Model').map((o) => o.value)).toContain('claude')

        setHosted(true)
        expect(isWorkdirsAvailable()).toBe(false)
        expect(agentKindOptions('Model').map((o) => o.value)).toEqual(['model'])
    })

    // The agents are listed here too, both kinds in one list, so they can be
    // found and edited with no board open.
    test('lists the agents of both kinds', async () => {
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([
                {name: 'claude', kind: 'claude'},
                {name: 'OpenAI', kind: 'model', provider: 'openai', model: 'gpt-5'},
            ])),
            AIProviders: vi.fn().mockResolvedValue([{id: 'openai', keySet: true}]),
        })
        render(() => wrapIntl(() => <AIPanel/>))

        await waitFor(() => expect(screen.getByText('Model by API key · OpenAI · gpt-5')).toBeInTheDocument())
        expect(screen.getByText('Claude')).toBeInTheDocument()
        expect(screen.getByRole('button', {name: 'Add agent…'})).toBeInTheDocument()
    })
})
