import {fireEvent, render, screen} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {TestRouter, mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {Constants} from '../../constants'
import {
    customThemeName,
    lightThemeName,
    setCustomThemeSource,
    setTheme,
    SHIPPED_THEMES,
} from '../../theme'

import AppPanel from './appPanel'

describe('components/settings/appPanel', () => {
    let store = mockAppStore({})

    beforeEach(() => {
        vi.clearAllMocks()
        store = mockAppStore({})
        setCustomThemeSource('')
        setTheme(lightThemeName)
    })

    const open = () => render(() => wrapIntl(() =>
        <AppStoreProvider store={store}>
            <TestRouter>
                <AppPanel/>
            </TestRouter>
        </AppStoreProvider>,
    ))

    // How the app looks used to be an icon menu in the corner of the board.
    // It is a setting of the install like every other one here — and a
    // dropdown rather than a row of chips, because there are fourteen themes
    // and there were three.
    test('switches the theme', () => {
        open()

        fireEvent.change(screen.getByLabelText('Theme'), {target: {value: 'windows-dark'}})

        expect(document.documentElement.dataset.theme).toBe('dark')
        expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('31, 31, 31')
    })

    test('offers every theme this build ships', () => {
        open()

        for (const theme of SHIPPED_THEMES) {
            expect(screen.getByRole('option', {name: theme.name!})).toBeInTheDocument()
        }
    })

    // The field is the whole point of borrowing Mattermost's format: a theme
    // written for another product is pasted in and worn.
    test('wears a config that was pasted into it', () => {
        open()

        fireEvent.change(screen.getByLabelText('Theme'), {target: {value: customThemeName}})
        fireEvent.input(screen.getByLabelText('Custom config'), {
            target: {
                value: JSON.stringify({
                    centerChannelBg: '#102030',
                    centerChannelColor: '#eeeeee',
                    sidebarBg: '#000000',
                    buttonBg: '#4488ff',
                }),
            },
        })
        userEvent.click(screen.getByRole('button', {name: 'Apply'}))

        expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('16, 32, 48')
        expect(document.documentElement.dataset.theme).toBe('dark')
    })

    // A config nobody can read has to say which way it is wrong, because this
    // field is the only place a person finds out.
    test('says why a config was refused', () => {
        open()

        fireEvent.change(screen.getByLabelText('Theme'), {target: {value: customThemeName}})
        fireEvent.input(screen.getByLabelText('Custom config'), {target: {value: '{oh no'}})
        userEvent.click(screen.getByRole('button', {name: 'Apply'}))

        expect(screen.getByText('This is not JSON.')).toBeInTheDocument()
    })

    // A config that stopped parsing is exactly when the field has to be on
    // screen: hanging it off "the theme applied" made a broken one impossible
    // to select and therefore impossible to fix.
    test('opens the field on a stored config that no longer parses', () => {
        setCustomThemeSource('{"centerChannelBg": "#fff"}')
        open()

        fireEvent.change(screen.getByLabelText('Theme'), {target: {value: customThemeName}})

        expect(screen.getByLabelText('Custom config')).toHaveValue('{"centerChannelBg": "#fff"}')
        expect(screen.getByText(/These keys are missing/)).toBeInTheDocument()
    })

    // Choosing the custom slot with nothing in it opens on a copy of the theme
    // on screen: a config field that opens empty is a format to go and look up.
    test('opens the custom slot on a copy of the theme on screen', () => {
        open()

        fireEvent.change(screen.getByLabelText('Theme'), {target: {value: customThemeName}})

        const field = screen.getByLabelText('Custom config') as HTMLTextAreaElement
        expect(JSON.parse(field.value).centerChannelBg).toBe('#f0ede5')
    })

    test('switches the language', () => {
        open()

        userEvent.click(screen.getByRole('button', {name: 'Deutsch'}))

        expect(store.state.language.value).toBe('de')
    })

    test('offers every language it speaks', () => {
        open()

        for (const language of Constants.languages) {
            expect(screen.getByRole('button', {name: language.displayName})).toBeInTheDocument()
        }
    })

    // The question mark in the corner of the board was a link and nothing else,
    // which is a thing a person looks for in the settings and not on a board.
    // Where it leads is the manual, not a source tree: somebody opening
    // the guide link wants to be told how the board works.
    test('leads to the guide', () => {
        open()

        expect(screen.getByRole('link', {name: 'Open'})).toHaveAttribute('href', Constants.guideUrl)
    })

    // The last thing left in the corner of the board, and the reason that
    // corner outlived the theme and the language. Somewhere to say that
    // something is broken is looked for once, which is here — and it is a
    // letter, so that saying it costs nobody an account anywhere.
    test('offers somewhere to say that something is broken', () => {
        open()

        expect(screen.getByRole('link', {name: 'Write'})).toHaveAttribute('href', `mailto:${Constants.feedbackEmail}`)
    })

    // The address is on the panel and not only in the link, because a webview
    // that refuses to open a mailto: leaves a person with nothing to copy.
    test('shows the address it writes to', () => {
        open()

        expect(screen.getByText(`Bugs and requests go by email, to ${Constants.feedbackEmail}.`)).toBeInTheDocument()
    })
})
