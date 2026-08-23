import {fireEvent, render, screen, waitFor, within} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {TestRouter, mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import {Constants} from '../../constants'
import {
    customThemeName,
    defaultFamilyId,
    setCustomThemeName,
    setCustomThemeSource,
    setTheme,
    setThemeMode,
    THEME_FAMILIES,
} from '../../theme'

import AppPanel from './appPanel'

describe('components/settings/appPanel', () => {
    let store = mockAppStore({})

    // Whether the panel is already on screen: `pick` renders it the first time
    // and reopens the list after that.
    let shown = false

    beforeEach(() => {
        vi.clearAllMocks()
        store = mockAppStore({})

        // The theme first: emptying the custom slot while it is the one on
        // screen leaves the mode switch with nothing to draw.
        setTheme(defaultFamilyId)
        setThemeMode('light')
        setCustomThemeSource('light', '')
        setCustomThemeSource('dark', '')
        setCustomThemeName('')
        shown = false
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
    // so everything the list is asked about is asked of the list.
    const dropTheList = () => {
        userEvent.click(screen.getByRole('button', {name: 'Theme'}))
        return () => within(document.querySelector('.Menu .menu-options') as HTMLElement)
    }

    const openThemes = () => {
        open()
        shown = true
        return dropTheList()
    }

    // Renders the panel the first time and reopens the list after that: a test
    // that picks twice was rendering a second panel over the first, and every
    // query then found two of everything.
    const pick = async (name: string) => {
        const menu = shown ? dropTheList() : openThemes()
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

    // Flat, with one line in it. The kinds carried a heading each and that was
    // three lines of furniture over twelve entries; what is left separates the
    // themes this build ships from the slot for one you bring.
    test('draws the themes as one list with the custom slot set apart', async () => {
        openThemes()
        await waitFor(() => expect(document.querySelector('.Menu .menu-options')).toBeInTheDocument())
        const list = document.querySelector('.Menu .menu-options') as HTMLElement

        expect(list.querySelectorAll('.LabelOption')).toHaveLength(0)
        expect(list.querySelectorAll('.MenuSeparator')).toHaveLength(1)
        expect(customThemeName).toBe('custom-theme')
        expect([...list.querySelectorAll('.menu-name')].map((n) => n.textContent)).toEqual([
            ...THEME_FAMILIES.map((family) => family.name),
            'Custom config',
        ])
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
    })

    // The slot is a pair like every other theme, so the switch beside the list
    // works there too — and it is also what says which half the field is
    // editing, because answering that twice would be two controls saying one
    // word.
    test('gives the custom slot two halves and edits the one on screen', async () => {
        await pick('Custom config')

        const light = await screen.findByLabelText('Theme config') as HTMLTextAreaElement
        expect(JSON.parse(light.value).centerChannelBg).toBe('#f0ede5')

        userEvent.click(screen.getByRole('button', {name: /^Dark/}))

        await waitFor(() => {
            const dark = screen.getByLabelText('Theme config') as HTMLTextAreaElement
            expect(JSON.parse(dark.value).centerChannelBg).toBe('#0c0c0e')
        })
        expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe('12, 12, 14')
    })

    // The switch on the custom slot is also what says which half the field
    // holds, so a third answer would mean the text under the cursor changing
    // when the machine goes dark at sunset. Picking the slot resolves the
    // machine's answer rather than defaulting past it, so nothing on screen
    // moves at the moment it is picked.
    test('asks the custom slot for a half and not for the machine', async () => {
        setThemeMode('system')
        const before = document.documentElement.style.getPropertyValue('--canvas-rgb')

        await pick('Custom config')
        await screen.findByLabelText('Theme config')

        expect(screen.queryByRole('button', {name: /^As the system is/})).not.toBeInTheDocument()
        expect(screen.getByRole('button', {name: /^Light/})).toBeInTheDocument()
        expect(screen.getByRole('button', {name: /^Dark/})).toBeInTheDocument()
        expect(document.documentElement.style.getPropertyValue('--canvas-rgb')).toBe(before)

        // And it is on offer again the moment the theme is one this build keeps
        // both halves of.
        await pick('Gruvbox')
        expect(screen.getByRole('button', {name: /^As the system is/})).toBeInTheDocument()
    })

    // Not "go back to the theme I had" — the list does that. This is the only
    // thing an edit in the field can undo.
    test('puts the field back where it was', async () => {
        await pick('Custom config')
        const field = await screen.findByLabelText('Theme config') as HTMLTextAreaElement
        const applied = field.value

        fireEvent.input(field, {target: {value: '{oh no'}})
        userEvent.click(screen.getByRole('button', {name: 'Cancel'}))

        await waitFor(() => expect(screen.getByLabelText('Theme config')).toHaveValue(applied))
    })

    // Neither button has anything to do until something is typed, and a
    // control that can do nothing says so rather than doing nothing quietly.
    test('offers nothing to apply or undo until the field is edited', async () => {
        await pick('Custom config')
        await screen.findByLabelText('Theme config')

        expect(screen.getByRole('button', {name: 'Apply'})).toBeDisabled()
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled()

        fireEvent.input(screen.getByLabelText('Theme config'), {target: {value: '{}'}})

        await waitFor(() => expect(screen.getByRole('button', {name: 'Apply'})).toBeEnabled())
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeEnabled()
    })

    // A slot called «Свой конфиг» is "the other one"; a name is what makes it
    // somebody's own theme, so the list says it.
    test('calls the slot what it was named', async () => {
        await pick('Custom config')

        fireEvent.input(await screen.findByLabelText('Name'), {target: {value: 'Ночная смена'}})

        const menu = dropTheList()
        await waitFor(() => expect(document.querySelector('.Menu .menu-options')).toBeInTheDocument())
        expect(menu().getByRole('button', {name: 'Ночная смена'})).toBeInTheDocument()
        expect(menu().queryByRole('button', {name: 'Custom config'})).not.toBeInTheDocument()
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
        setCustomThemeSource('light', '{"centerChannelBg": "#fff"}')
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
