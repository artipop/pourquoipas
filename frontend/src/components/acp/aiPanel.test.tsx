import {render, screen, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'
import {afterEach, describe, expect, test, vi} from 'vitest'

import userEvent from '@testing-library/user-event'

import {wrapIntl} from '../../testUtils'
import {fakeGoSide} from '../../test/goSide'
import {setHosted} from '../../buildKind'

import {isWorkdirsAvailable} from './workdirsPanel'

import AIPanel from './aiPanel'

describe('components/acp/aiPanel', () => {
    afterEach(() => {
        setHosted(false)
        fakeGoSide()
    })

    // One way to connect an agent by key: the key is typed into the agent's
    // own form, checked with the provider, and the agent is saved on it.
    test('connects a model agent with a new key', async () => {
        const SetAIProviderKey = vi.fn().mockResolvedValue({models: [{id: 'claude-x'}], suggested: 'claude-x'})
        const AddAgent = vi.fn().mockResolvedValue('{}')
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue('[]'),
            AIProviders: vi.fn().mockResolvedValue([{id: 'anthropic', keySet: false, defaultModel: 'claude-opus'}, {id: 'openai', keySet: false}]),
            SetAIProviderKey,
            AddAgent,
        })
        render(() => wrapIntl(() => <AIPanel/>))

        userEvent.click(await screen.findByRole('button', {name: 'Connect an agent…'}))
        userEvent.click(screen.getByRole('radio', {name: /Model by API key/}))

        const key = await screen.findByLabelText('API key') as HTMLInputElement
        expect(key.type).toBe('password')
        userEvent.type(key, 'sk-new')
        expect(screen.getByPlaceholderText('Name')).toHaveValue('Claude')

        userEvent.click(screen.getByRole('button', {name: 'Connect'}))
        await waitFor(() => expect(AddAgent).toHaveBeenCalled())
        expect(SetAIProviderKey).toHaveBeenCalledWith('anthropic', 'sk-new', '')
        expect(JSON.parse(AddAgent.mock.calls[0][0])).toMatchObject({name: 'Claude', kind: 'model', provider: 'anthropic', model: expect.stringMatching(/^claude-/)})
    })

    // A key the provider refuses is not stored, and no agent is made on it.
    test('a refused key makes no agent', async () => {
        const AddAgent = vi.fn()
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue('[]'),
            AIProviders: vi.fn().mockResolvedValue([{id: 'anthropic', keySet: false, defaultModel: 'claude-opus'}]),
            SetAIProviderKey: vi.fn().mockRejectedValue(new Error('bad key')),
            AddAgent,
        })
        render(() => wrapIntl(() => <AIPanel/>))

        userEvent.click(await screen.findByRole('button', {name: 'Connect an agent…'}))
        userEvent.click(screen.getByRole('radio', {name: /Model by API key/}))
        userEvent.type(await screen.findByLabelText('API key'), 'sk-bad')
        userEvent.click(screen.getByRole('button', {name: 'Connect'}))

        await waitFor(() => expect(screen.getByText(/bad key/)).toBeInTheDocument())
        expect(AddAgent).not.toHaveBeenCalled()
    })

    // The provider's key is shared by its agents: a second agent on it is not
    // asked for the key again, and the key itself never comes back.
    test('a key already set is named, not asked for', async () => {
        const SetAIProviderKey = vi.fn()
        const AddAgent = vi.fn().mockResolvedValue('{}')
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue('[]'),
            AIProviders: vi.fn().mockResolvedValue([{id: 'anthropic', keySet: true, keyHint: '…ab12', defaultModel: 'claude-opus'}]),
            AIModels: vi.fn().mockResolvedValue([]),
            SetAIProviderKey,
            AddAgent,
        })
        render(() => wrapIntl(() => <AIPanel/>))

        userEvent.click(await screen.findByRole('button', {name: 'Connect an agent…'}))
        userEvent.click(screen.getByRole('radio', {name: /Model by API key/}))
        await waitFor(() => expect(screen.getByText('…ab12')).toBeInTheDocument())
        expect(screen.queryByLabelText('API key')).toBeNull()

        userEvent.click(screen.getByRole('button', {name: 'Connect'}))
        await waitFor(() => expect(AddAgent).toHaveBeenCalled())
        expect(SetAIProviderKey).not.toHaveBeenCalled()
    })

    // A hosted server has no machine to work on: no folders, no programs, no
    // proxies — the one kind of agent there is a model.
    test('a hosted server connects models only', async () => {
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue('[]'),
            ListAgentWorkdirs: vi.fn(),
            ListProxies: vi.fn().mockResolvedValue('[]'),
            AIProviders: vi.fn().mockResolvedValue([{id: 'anthropic', keySet: false}]),
        })
        expect(isWorkdirsAvailable()).toBe(true)
        setHosted(true)
        expect(isWorkdirsAvailable()).toBe(false)

        render(() => wrapIntl(() => <AIPanel/>))
        userEvent.click(await screen.findByRole('button', {name: 'Connect an agent…'}))

        expect(await screen.findByLabelText('API key')).toBeInTheDocument()
        expect(screen.queryByRole('radio', {name: /Program on this computer/})).toBeNull()
        expect(screen.queryByText('Proxy configurations')).toBeNull()
    })

    // Both kinds in one list, with the proxies under them.
    test('lists the agents of both kinds, and the proxies', async () => {
        fakeGoSide({
            ListAgents: vi.fn().mockResolvedValue(JSON.stringify([
                {name: 'claude', kind: 'claude'},
                {name: 'OpenAI', kind: 'model', provider: 'openai', model: 'gpt-5'},
            ])),
            ListProxies: vi.fn().mockResolvedValue('[]'),
            AIProviders: vi.fn().mockResolvedValue([{id: 'openai', keySet: true}]),
        })
        render(() => wrapIntl(() => <AIPanel/>))

        await waitFor(() => expect(screen.getByText('OpenAI · gpt-5 · Model by API key')).toBeInTheDocument())
        expect(screen.getByText('Claude · program on this computer')).toBeInTheDocument()
        expect(screen.getByText('Proxy configurations')).toBeInTheDocument()
    })
})
