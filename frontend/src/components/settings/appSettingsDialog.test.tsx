import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'
import {createSignal, flush} from 'solid-js'

import {TestRouter, mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {IntlProvider} from '../../intl'
import messagesRu from '../../../i18n/ru.json'

import {fakeGoSide} from '../../test/goSide'

import {installAgentFeatures} from '../acp/install'
import {resetExtensions} from '../../extensions'

import AppSettingsDialog from './appSettingsDialog'

describe('components/settings/appSettingsDialog', () => {
    // Drawn as main.tsx draws it: with the agents' sections plugged in.
    beforeEach(() => installAgentFeatures())
    afterEach(() => resetExtensions())

    const store = mockAppStore({teams: {current: {id: 'team-id'}}})

    const bindings = () => ({
        ListAgents: vi.fn().mockResolvedValue(JSON.stringify([{name: 'claude', kind: 'claude'}])),
        ListAgentAdapters: vi.fn().mockResolvedValue('[]'),
        ListProxies: vi.fn().mockResolvedValue(JSON.stringify([{name: 'office', proxy: 'http://proxy.example.com:8080'}])),
        GetPlanningPrompt: vi.fn().mockResolvedValue('Think it through.'),
        GetServerAttachment: vi.fn().mockResolvedValue(JSON.stringify({url: '', username: '', running: ''})),
    })

    const open = () => render(() => wrapIntl(() =>
        <AppStoreProvider store={store}>
            <TestRouter>
                <AppSettingsDialog onClose={vi.fn()}/>
            </TestRouter>
        </AppStoreProvider>,
    ))

    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // The point of the move: none of this is about a board, so none of it needs
    // one open. The dialog is handed no board and asks for none.
    test('opens a machine registry without a board', async () => {
        fakeGoSide(bindings())

        open()

        userEvent.click(screen.getByRole('button', {name: 'Server'}))
        await waitFor(() => expect(screen.getByText('Server address')).toBeInTheDocument())
    })

    // Agents and proxies are not sections of their own: they are in «AI».
    test('keeps the agents and their proxies inside the AI section', async () => {
        fakeGoSide(bindings())

        open()

        await waitFor(() => expect(screen.getByRole('button', {name: 'AI'})).toBeInTheDocument())
        expect(screen.queryByRole('button', {name: 'Agents'})).toBeNull()
        expect(screen.queryByRole('button', {name: 'Proxy configurations'})).toBeNull()
    })

    // What a card-less conversation opens with is an instruction to the agent
    // with a working default, and is not offered (OFFER_PLANNING_PROMPT); nor
    // are agent-named branches (git is kept off the screen) or the switch for
    // filling a new card's folder in, which is simply on.
    test('offers no planning prompt, named branches or folder prefill switch', async () => {
        fakeGoSide(bindings())

        open()
        userEvent.click(screen.getByRole('button', {name: 'Other'}))

        await waitFor(() => expect(screen.getByText(/config\.json/)).toBeInTheDocument())
        expect(screen.queryByRole('textbox')).toBeNull()
        expect(screen.queryByText('The agent names each card’s branch')).toBeNull()
        expect(screen.queryByText('Fill a new card’s folder in when the board has only one')).toBeNull()
    })

    // Whether an agent may interrupt is an AI setting, beside the switch that
    // turns the agents on, and not in «Other».
    test('carries whether an agent waiting may interrupt, in AI', async () => {
        fakeGoSide(bindings())

        open()
        userEvent.click(screen.getByRole('button', {name: 'AI'}))
        await waitFor(() => expect(screen.getByText('Notify me when an agent is waiting')).toBeInTheDocument())

        userEvent.click(screen.getByRole('button', {name: 'Other'}))
        await waitFor(() => expect(screen.getByText(/config\.json/)).toBeInTheDocument())
        expect(screen.queryByText('Notify me when an agent is waiting')).toBeNull()
    })

    // Carrying boards in and out is not part of the agent integration: a build
    // with no agents at all still opens on it.
    test('offers import and export whatever else the machine can do', async () => {
        open()

        userEvent.click(screen.getByRole('button', {name: 'Import and export'}))

        await waitFor(() => expect(screen.getByText('Import archive')).toBeInTheDocument())
        expect(screen.getByText('Export archive')).toBeInTheDocument()
    })

    // Light or dark and the language came here from the corner of the board,
    // and they are what a fresh install with nothing registered opens on: every
    // other section belongs to something this build may not be able to do.
    test('opens on what every install can answer', async () => {
        open()

        await waitFor(() => expect(screen.getByRole('button', {name: /^Dark/})).toBeInTheDocument())
        expect(screen.getByRole('button', {name: 'Deutsch'})).toBeInTheDocument()
    })

    // The language is picked in this dialog, so this dialog is the one place
    // where a name formatted once, in the component body, is visibly wrong: the
    // list down the side and the heading above the panel went on speaking the
    // language the dialog opened in, while everything drawn inside JSX changed
    // under them.
    test('changes language with the rest of the app', async () => {
        const [locale, setLocale] = createSignal('en')

        render(() => (
            <AppStoreProvider store={store}>
                <IntlProvider
                    locale={locale()}
                    messages={locale() === 'ru' ? messagesRu : {}}
                >
                    <TestRouter>
                        <AppSettingsDialog onClose={vi.fn()}/>
                    </TestRouter>
                </IntlProvider>
            </AppStoreProvider>
        ))

        expect(screen.getByRole('button', {name: 'Import and export'})).toBeInTheDocument()

        setLocale('ru')
        flush()

        expect(screen.getByRole('button', {name: 'Импорт и экспорт'})).toBeInTheDocument()
        expect(screen.getByRole('heading', {name: 'Приложение'})).toBeInTheDocument()
    })
})
