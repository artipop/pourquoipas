import {render, screen, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import AgentsDialog, {isAgentsDialogAvailable} from './agentsDialog'

describe('components/acp/agentsDialog', () => {
    const bindings = () => ({
        ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}])),
        ListAgentAdapters: vi.fn().mockResolvedValue('[]'),
        ListProxies: vi.fn().mockResolvedValue(JSON.stringify([{name: 'office', proxy: 'http://proxy.example.com:8080'}])),
    })

    const open = () => render(() => wrapIntl(() => <AgentsDialog onClose={vi.fn()}/>))

    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // One door for one question — who works this board's cards and how that
    // agent reaches the network — and the order is the order it is asked in.
    test('lists the agents, and the proxies under them', async () => {
        fakeGoSide(bindings())

        open()

        // ("claude" is both the agent's name and its kind, so the registered
        // row is matched by its own class.)
        await waitFor(() => expect(document.querySelector('.AgentsPanel__name')).toHaveTextContent('claude'))
        expect(screen.getByText('office')).toBeInTheDocument()

        const agents = document.querySelector('.AgentsPanel')!
        const proxies = document.querySelector('.ProxiesPanel')!
        expect(agents.compareDocumentPosition(proxies) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    // The door is drawn on the binding, not on the board: a build with no agent
    // integration at all has nothing to open here.
    test('is not offered by a build that knows no agents', () => {
        expect(isAgentsDialogAvailable()).toBe(false)
        fakeGoSide(bindings())
        expect(isAgentsDialogAvailable()).toBe(true)
    })
})
