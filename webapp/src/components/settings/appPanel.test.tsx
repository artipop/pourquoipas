import {fireEvent, render, screen, waitFor, within} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {TestRouter, mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {Constants} from '../../constants'
import {
    defaultFamilyId,
    setCustomThemeSource,
    setTheme,
    setThemeMode,
    THEME_FAMILIES,
} from '../../theme'

import AppPanel from './appPanel'

describe('components/settings/appPanel', () => {
    let store = mockAppStore({})

    beforeEach(() => {
        vi.clearAllMocks()
        store = mockAppStore({})
        setCustomThemeSource('')

        // The polarity first: `setTheme` applies with whatever mode is
        // current, and `system` asks `matchMedia`, which jsdom has not got.
        setThemeMode('light')
        setTheme(defaultFamilyId)
    })

    const open = () => render(() => wrapIntl(() =>
        <AppStoreProvider store={store}>
            <TestRouter>
                <AppPanel/>
            </TestRouter>
        </AppStoreProvider>,
    ))

    // How the app looks used to be an icon menu in the corner of the board. It
    // is a setting of the install like every other one here — and the themes
    // are the app's own dropdown, because there are eleven of them and a row of
    // eleven chips is a wall.
    // The trigger says the current theme's name and so does the option for it,
    // and the group headings repeat a name too — so everything the list is
    // asked about is asked of the list.
    const openThemes = () => {
        open()
        userEvent.click(screen.getByRole('button', {name: 'Theme'}))
        return () => within(document.querySelector('.Menu .menu-options') as HTMLElement)
    }

    const pick = async (name: string) => {
        const menu = openThemes()
        await waitFor(() => expect(document.querySelector('.Menu .menu-options')).toBeInTheDocument())
        userEvent.click(menu().getByRole('button', {name}))
    }

    test('switches the theme', async () => {
        await pick('Windows')

        // The light half, because the polarity is the other control's answer
        // and this test set it to light.
        await waitFor(() => expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('243, 243, 243'))
        expect(document.documentElement.dataset.theme).toBe('light')
    })

    // The split the two controls exist for: the design stays put while the
    // polarity moves. One list could not do this — following the system meant
    // giving up every design but the product's own.
    test('turns the theme it is on from light to dark', async () => {
        await pick('Gruvbox')
        await waitFor(() => expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('251, 241, 199'))

        userEvent.click(screen.getByRole('button', {name: /^Dark/}))

        await waitFor(() => expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('60, 56, 54'))
        expect(document.documentElement.dataset.theme).toBe('dark')
    })

    test('offers every theme this build ships', async () => {
        const menu = openThemes()
        await waitFor(() => expect(document.querySelector('.Menu .menu-options')).toBeInTheDocument())

        for (const family of THEME_FAMILIES) {
            expect(menu().getByRole('button', {name: family.name})).toBeInTheDocument()
        }
    })

    // The field is the whole point of borrowing Mattermost's format: a theme
    // written for another product is pasted in and worn.
    test('wears a config that was pasted into it', async () => {
        await pick('Custom config')

        fireEvent.input(await screen.findByLabelText('Theme config'), {
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

        await waitFor(() => expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('16, 32, 48'))
        expect(document.documentElement.dataset.theme).toBe('dark')

        // A pasted config is one half, not a pair, so it brings its own
        // polarity and the switch has nothing to say about it.
        expect(screen.queryByRole('button', {name: /^Dark/})).not.toBeInTheDocument()
    })

    // A config nobody can read has to say which way it is wrong, because this
    // field is the only place a person finds out.
    test('says why a config was refused', async () => {
        await pick('Custom config')

        fireEvent.input(await screen.findByLabelText('Theme config'), {target: {value: '{oh no'}})
        userEvent.click(screen.getByRole('button', {name: 'Apply'}))

        expect(await screen.findByText('This is not JSON.')).toBeInTheDocument()
    })

    // A config that stopped parsing is exactly when the field has to be on
    // screen: hanging it off "the theme applied" made a broken one impossible
    // to select and therefore impossible to fix.
    test('opens the field on a stored config that no longer parses', async () => {
        setCustomThemeSource('{"centerChannelBg": "#fff"}')
        await pick('Custom config')

        expect(await screen.findByLabelText('Theme config')).toHaveValue('{"centerChannelBg": "#fff"}')
        expect(screen.getByText(/These keys are missing/)).toBeInTheDocument()
    })

    // Choosing the custom slot with nothing in it opens on a copy of the theme
    // on screen: a config field that opens empty is a format to go and look up.
    test('opens the custom slot on a copy of the theme on screen', async () => {
        await pick('Custom config')

        const field = await screen.findByLabelText('Theme config') as HTMLTextAreaElement
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
