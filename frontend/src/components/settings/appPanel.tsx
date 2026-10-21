import {For, Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Constants} from '../../constants'
import {useAppSelector, useAppStore} from '../../store/hooks'
import {getCurrentBoard} from '../../store/boards'
import {getMe} from '../../store/users'
import {patchPreferences, startTour} from '../onboardingTour/tourState'
import CheckIcon from '../../widgets/icons/check'
import Select, {SelectOption} from '../../widgets/select'
import Switch from '../../widgets/switch'
import {aiEnabled, setAIEnabled} from '../../aiSwitch'
import {goApp} from '../../goBridge'
import {
    activeThemeHalf,
    customThemes,
    isCustomThemeId,
    newCustomTheme,
    removeCustomTheme,
    getActiveThemeMode,
    getActiveThemeName,
    getCustomThemeName,
    getCustomThemeSource,
    setCustomThemeName,
    setCustomThemeSource,
    setTheme,
    setThemeMode,
    ThemeConfigError,
    ThemeHalf,
    ThemeMode,
    themeModes,
    ThemeName,
    THEME_FAMILIES,
    themeChoiceOpen,
    themeFamilyName,
    defaultFamilyId,
} from '../../theme'

import './appPanel.scss'

// The app talking about itself: how it looks, what language it speaks, where
// it is written down, and where to say that something is broken. All four
// lived in the corner of the board — two icon menus, a question mark and a
// link — where an icon had to stand for a word and the corner grew a control
// per answer. They are read once and set once, which is what the rest of this
// dialog is for; the corner is gone with the last of them.
//
// The languages are laid out rather than folded into a menu: there are a dozen
// of them, and a person picking one wants to see what there is.
//
// The themes are a dropdown, and they are the one place that rule turned over:
// there were three of them and there are twelve, and a row of twelve chips is a
// wall rather than a choice. It is `widgets/select` — the app's own field over
// the app's own menu — and not the platform's `<select>`, which keeps the
// system's white on a dark board and drops a list no stylesheet can reach.
//
// The list is flat. It carried a heading over each of the three kinds — ours,
// borrowed, joke — and that was three lines of furniture over twelve entries a
// person picks by name; the order says the same thing and says it silently.
// One line survives, above «Свой конфиг», because that row is not another
// theme: it is the slot for one you bring.
//
// **Light or dark is a second question and gets a second control.** A theme is
// a design and it comes in both; whether this machine wears the light half, the
// dark one or whichever the system is set to has nothing to do with which
// design it is. One list said «Gruvbox Dark» and «Как в системе» in the same
// column, so picking a design meant also deciding a polarity, and following the
// system meant giving up every design but ours. Three answers, so they are laid
// out — the rule the themes are the exception to.
//
// **Which half the field is editing is a third question, and it is only asked
// on the custom slot.** It cannot be the switch above: that one has «как в
// системе» in it, which is a real answer for a board and no answer at all for
// a config being typed — there is no third config to edit. So the slot carries
// its own light/dark, opening on whatever is on screen and moving only when it
// is clicked.

// The last row of the theme list, which is not a theme: it makes one. A value
// no id can be, since a theme somebody brought is `custom-theme` or `custom:N`.
const NEW_THEME = '+new'

// The switch means something only where the app can run agents at all: a
// browser build has no Go side and nothing to switch off.
const hasAgentSide = (): boolean => Boolean(goApp()?.ListAgents)

const AppPanel = (props: {onClose?: () => void}) => {
    const intl = useIntl()
    const {actions} = useAppStore()

    // The tour again, and every hint with it: the tour on the board that is
    // open, the hints wherever their moments come. With no board open there is
    // nowhere for the tour to run.
    const me = useAppSelector(getMe)
    const board = useAppSelector(getCurrentBoard)
    const showTourAgain = async () => {
        const userId = me()?.id
        const boardId = board()?.id
        if (!userId || !boardId) {
            return
        }
        await patchPreferences(userId, {shownHints: '[]'}, actions.users.patchProps)
        await startTour(userId, boardId, actions.users.patchProps)
        props.onClose?.()
    }

    // The theme module writes the tokens onto the document element and is not
    // reactive, so what is current is kept here, seeded with what startup
    // loaded.
    const [themeName, setThemeName] = createSignal(getActiveThemeName())
    const [mode, setMode] = createSignal(getActiveThemeMode())
    const [error, setError] = createSignal('')

    // The list of themes a person brought is not reactive either — it lives in
    // the install — so a revision counter is what tells the options to be read
    // again after one is added, renamed or deleted.
    const [revision, setRevision] = createSignal(0)
    const reread = () => setRevision((n) => n + 1)

    // Which half the field is editing. Its own answer, because the switch above
    // it can say «как в системе» and a config cannot: it opens on whatever is
    // drawn and then stays where it is put.
    const [half, setHalf] = createSignal<ThemeHalf>(activeThemeHalf())
    const [source, setSource] = createSignal(getCustomThemeSource(getActiveThemeName(), activeThemeHalf()))
    const [customName, setCustomName] = createSignal(getCustomThemeName(getActiveThemeName()))

    // What «Отмена» puts back, and what tells the two buttons whether there is
    // anything to do: the text as it was last applied.
    const editing = () => isCustomThemeId(themeName())
    const stored = () => getCustomThemeSource(themeName(), half())
    const dirty = () => source() !== stored()

    // Spelled out rather than built as `Settings.theme-mode-${value}`: a
    // computed id is invisible to `npm run i18n-extract`, so it would reach no
    // catalogue and read as a dead entry in every other one.
    const modeTitle = (value: ThemeMode): string => {
        switch (value) {
        case 'light':
            return intl.formatMessage({id: 'Settings.theme-mode-light', defaultMessage: 'Light'})
        case 'dark':
            return intl.formatMessage({id: 'Settings.theme-mode-dark', defaultMessage: 'Dark'})
        default:
            return intl.formatMessage({id: 'Settings.theme-mode-system', defaultMessage: 'As the system is'})
        }
    }

    // Every way a config can be refused, in the words of the one place a person
    // finds out. The reasons are separate messages rather than one because
    // "this is not JSON" and "this has no sidebar colour" are answered by
    // different things.
    const reasonFor = (e: unknown): string => {
        const raw = e instanceof ThemeConfigError ? e.message : ''
        const [kind, detail] = raw.split(/:(.*)/s)
        switch (kind) {
        case 'empty':
            return intl.formatMessage({id: 'Settings.theme-error-empty', defaultMessage: 'The field is empty.'})
        case 'json':
            return intl.formatMessage({id: 'Settings.theme-error-json', defaultMessage: 'This is not JSON.'})
        case 'object':
            return intl.formatMessage({id: 'Settings.theme-error-object', defaultMessage: 'A theme is one JSON object of colours.'})
        case 'missing':
            return intl.formatMessage({
                id: 'Settings.theme-error-missing',
                defaultMessage: 'These keys are missing, and a theme cannot be built without them: {keys}.',
            }, {keys: detail})
        case 'color':
            return intl.formatMessage({
                id: 'Settings.theme-error-color',
                defaultMessage: 'These are not colours like #1c1a16: {keys}.',
            }, {keys: detail})
        default:
            return intl.formatMessage({id: 'Settings.theme-error-other', defaultMessage: 'This config cannot be read.'})
        }
    }

    // Reading the stored text back rather than keeping a signal per half: what
    // was typed and not applied is not a version of anything.
    const editHalf = (side: ThemeHalf) => {
        setHalf(side)
        setSource(getCustomThemeSource(themeName(), side))
        setError('')
    }

    const showTheme = (name: ThemeName) => {
        setThemeName(name)
        setCustomName(getCustomThemeName(name))
        setHalf(activeThemeHalf())
        setSource(getCustomThemeSource(name, activeThemeHalf()))
    }

    // The selection is recorded before the theme is applied, and stays put when
    // applying fails. A config that stopped parsing is exactly the case where
    // the editor has to be on screen, and hanging it off "the theme applied"
    // made a broken one impossible to select and therefore impossible to fix.
    //
    // What is applied is the name in hand, not `themeName()` read back: in an
    // event handler Solid 2 keeps a write staged, so the read returned the theme
    // being left and the click applied that one.
    const pickTheme = (name: ThemeName) => {
        setError('')
        let chosen = name
        if (name === NEW_THEME) {
            chosen = newCustomTheme().id
            reread()
        }
        showTheme(chosen)
        try {
            setTheme(chosen)
        } catch (e) {
            setError(reasonFor(e))
        }
    }

    const choiceClass = (current: boolean) => `AppPanel__choice${current ? ' AppPanel__choice--current' : ''}`

    const themeOptions = (): SelectOption[] => {
        revision()
        const mine = customThemes()

        return [
            ...THEME_FAMILIES.map((family) => ({value: family.id, label: themeFamilyName(family, intl)})),

            // One line in the list, and it is not a heading: everything above
            // it is a theme this build ships, everything below it is one you
            // brought.
            ...mine.map((theme, index) => ({
                value: theme.id,
                label: theme.name || intl.formatMessage({id: 'Settings.theme-custom', defaultMessage: 'Custom config'}),
                separator: index === 0,
            })),
            {
                value: NEW_THEME,
                label: intl.formatMessage({id: 'Settings.theme-new', defaultMessage: '+ New theme'}),
                separator: mine.length === 0,
            },
        ]
    }

    const applyCustom = () => {
        setCustomThemeSource(themeName(), half(), source())
        try {
            setTheme(themeName())
            setError('')
        } catch (e) {
            setError(reasonFor(e))
        }
    }

    // Deleting takes the person back to the theme the product ships, because
    // the one they were wearing has just stopped existing.
    const deleteCustom = () => {
        removeCustomTheme(themeName())
        reread()
        setError('')
        showTheme(defaultFamilyId)
        setTheme(defaultFamilyId)
    }

    const cancelCustom = () => {
        setSource(stored())
        setError('')
    }

    return (
        <div class='AppPanel'>
            <div class='AppPanel__subtitle'>
                {intl.formatMessage({
                    id: 'Settings.app-subtitle',
                    defaultMessage: 'The app\'s theme and language, the guide, and where to report a problem.',
                })}
            </div>

            <div class='AppPanel__content'>
                <Show when={themeChoiceOpen()}>
                    <section class='AppPanel__group'>
                        <h4 class='AppPanel__groupTitle'>
                            {intl.formatMessage({id: 'Settings.theme', defaultMessage: 'Theme'})}
                        </h4>
                        <Select
                            class='AppPanel__select'
                            label={intl.formatMessage({id: 'Settings.theme', defaultMessage: 'Theme'})}
                            value={themeName()}
                            options={themeOptions()}
                            onChange={pickTheme}
                        />
                    </section>
                </Show>

                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'Settings.theme-mode', defaultMessage: 'Light or dark'})}
                    </h4>
                    <div class='AppPanel__choices'>
                        <For each={themeModes}>
                            {(value) => (
                                <button
                                    type='button'
                                    class={choiceClass(mode() === value)}
                                    aria-pressed={String(mode() === value)}
                                    onClick={() => {
                                        setError('')
                                        try {
                                            setMode(setThemeMode(value))
                                        } catch (e) {
                                            setMode(value)
                                            setError(reasonFor(e))
                                        }
                                    }}
                                >
                                    <span class='AppPanel__choiceName'>{modeTitle(value)}</span>
                                    <Show when={mode() === value}>
                                        <span class='AppPanel__choiceTick'><CheckIcon/></span>
                                    </Show>
                                </button>
                            )}
                        </For>
                    </div>
                </section>

                {/* A frame, because this is a thing being made rather than
                    a setting being read: the panel around it is a column of
                    one-line answers, and an editor loose among them read as
                    more of the same. Not a modal — the settings are already
                    one, and a dialog over a dialog in this app has to be
                    argued with the sidebar's stacking context. */}
                <Show when={themeChoiceOpen() && editing()}>
                    <section class='AppPanel__group AppPanel__editor'>
                        <h4 class='AppPanel__groupTitle'>
                            {customName() || intl.formatMessage({id: 'Settings.theme-custom', defaultMessage: 'Custom config'})}
                        </h4>
                        <div class='AppPanel__custom'>
                            <label class='AppPanel__field'>
                                <span class='AppPanel__fieldName'>
                                    {intl.formatMessage({id: 'Settings.theme-custom-name', defaultMessage: 'Name'})}
                                </span>
                                <input
                                    class='AppPanel__name'
                                    type='text'
                                    value={customName()}
                                    placeholder={intl.formatMessage({id: 'Settings.theme-custom', defaultMessage: 'Custom config'})}
                                    onInput={(event) => {
                                        setCustomName(event.currentTarget.value)
                                        setCustomThemeName(themeName(), event.currentTarget.value)
                                        reread()
                                    }}
                                />
                            </label>

                            <div class='AppPanel__field'>
                                <span class='AppPanel__fieldName'>
                                    {intl.formatMessage({id: 'Settings.theme-custom-half', defaultMessage: 'Which half to edit'})}
                                </span>
                                <div class='AppPanel__choices AppPanel__halves'>
                                    <For each={['light', 'dark'] as ThemeHalf[]}>
                                        {(side) => (
                                            <button
                                                type='button'
                                                class={choiceClass(half() === side)}
                                                aria-pressed={String(half() === side)}
                                                onClick={() => editHalf(side)}
                                            >
                                                <span class='AppPanel__choiceName'>{modeTitle(side)}</span>
                                                <Show when={half() === side}>
                                                    <span class='AppPanel__choiceTick'><CheckIcon/></span>
                                                </Show>
                                            </button>
                                        )}
                                    </For>
                                </div>
                            </div>

                            <div class='AppPanel__actionHint'>
                                {intl.formatMessage({
                                    id: 'Settings.theme-custom-hint',
                                    defaultMessage: 'A theme is a config in Mattermost\'s format. Paste one here, or edit the copy of the theme you were looking at.',
                                })}
                                {' '}
                                <a
                                    href={Constants.themeHelpPage}
                                    target='_blank'
                                    rel='noreferrer'
                                >
                                    {intl.formatMessage({id: 'Settings.theme-custom-help', defaultMessage: 'What the keys mean'})}
                                </a>
                            </div>

                            {/* The field is named apart from the theme list
                                beside it: that list's trigger is named by the
                                value it shows, which is this theme's name
                                exactly when this field is on screen — two
                                controls answering to one name. */}
                            <textarea
                                class='AppPanel__code'
                                aria-label={intl.formatMessage({id: 'Settings.theme-custom-field', defaultMessage: 'Theme config'})}
                                spellcheck={false}
                                rows={12}
                                value={source()}
                                onInput={(event) => {
                                    setSource(event.currentTarget.value)
                                    setError('')
                                }}
                            />
                            <div class='AppPanel__customActions'>
                                <button
                                    type='button'
                                    class='AppPanel__actionLink'
                                    disabled={!dirty()}
                                    onClick={applyCustom}
                                >
                                    {intl.formatMessage({id: 'Settings.theme-custom-apply', defaultMessage: 'Apply'})}
                                </button>

                                {/* Not "go back to the theme I had": the list
                                    above does that. This puts the field back
                                    to the text that is actually applied, which
                                    is the only thing an edit here can undo. */}
                                <button
                                    type='button'
                                    class='AppPanel__actionLink AppPanel__actionLink--quiet'
                                    disabled={!dirty()}
                                    onClick={cancelCustom}
                                >
                                    {intl.formatMessage({id: 'Settings.theme-custom-cancel', defaultMessage: 'Cancel'})}
                                </button>
                                <Show when={error()}>
                                    <span class='AppPanel__customError'>{error()}</span>
                                </Show>
                                <button
                                    type='button'
                                    class='AppPanel__actionLink AppPanel__actionLink--danger'
                                    onClick={deleteCustom}
                                >
                                    {intl.formatMessage({id: 'Settings.theme-delete', defaultMessage: 'Delete theme'})}
                                </button>
                            </div>
                        </div>
                    </section>
                </Show>

                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'Settings.language', defaultMessage: 'Language'})}
                    </h4>
                    <div class='AppPanel__choices'>
                        <For each={Constants.languages}>
                            {(language) => (
                                <button
                                    type='button'
                                    class={choiceClass(intl.locale.toLowerCase() === language.code)}
                                    aria-pressed={String(intl.locale.toLowerCase() === language.code)}
                                    onClick={() => actions.language.storeLanguage(language.code)}
                                >
                                    <span class='AppPanel__choiceName'>{language.displayName}</span>
                                    <Show when={intl.locale.toLowerCase() === language.code}>
                                        <span class='AppPanel__choiceTick'><CheckIcon/></span>
                                    </Show>
                                </button>
                            )}
                        </For>
                    </div>
                </section>

                {/* The one switch for everything the app does with agents
                    (aiSwitch.ts). Here rather than among the agent settings,
                    because switched off there are none of those to be among. */}
                <Show when={hasAgentSide()}>
                    <section class='AppPanel__group'>
                        <h4 class='AppPanel__groupTitle'>
                            {intl.formatMessage({id: 'Settings.ai', defaultMessage: 'AI agents'})}
                        </h4>
                        <div class='AppPanel__action'>
                            <div class='AppPanel__actionText'>
                                <span class='AppPanel__actionName'>
                                    {intl.formatMessage({id: 'Settings.ai-enabled', defaultMessage: 'Agents work on the cards'})}
                                </span>
                                <span class='AppPanel__actionHint'>
                                    {intl.formatMessage({
                                        id: 'Settings.ai-enabled-hint',
                                        defaultMessage: 'When this is off, the app is a plain kanban board: no agents and no agent settings, and cards moved between columns start nothing. The routes and agents already set up are kept and come back when this is turned on again.',
                                    })}
                                </span>
                            </div>
                            <Switch
                                isOn={aiEnabled()}
                                onChanged={setAIEnabled}
                            />
                        </div>
                    </section>
                </Show>

                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'Settings.help', defaultMessage: 'Help'})}
                    </h4>
                    <div class='AppPanel__action'>
                        <div class='AppPanel__actionText'>
                            <span class='AppPanel__actionName'>
                                {intl.formatMessage({id: 'Settings.help-page', defaultMessage: 'The guide'})}
                            </span>
                            <span class='AppPanel__actionHint'>
                                {aiEnabled() ? intl.formatMessage({
                                    id: 'Settings.help-hint',
                                    defaultMessage: 'How the board, its columns and its cards work, screen by screen.',
                                }) : intl.formatMessage({
                                    id: 'Settings.help-hint-plain',
                                    defaultMessage: 'How the board and its columns work, screen by screen.',
                                })}
                            </span>
                        </div>
                        <a
                            class='AppPanel__actionLink'
                            href={Constants.guideUrl}
                            target='_blank'
                            rel='noreferrer'
                        >
                            {intl.formatMessage({id: 'Settings.help-open', defaultMessage: 'Open'})}
                        </a>
                    </div>

                    <div class='AppPanel__action'>
                        <div class='AppPanel__actionText'>
                            <span class='AppPanel__actionName'>
                                {intl.formatMessage({id: 'Settings.tour', defaultMessage: 'The tour'})}
                            </span>
                            <span class='AppPanel__actionHint'>
                                {intl.formatMessage({
                                    id: 'Settings.tour-hint',
                                    defaultMessage: 'Shows how the board works again, tip by tip, on the board that is open, and brings back the hints that have been put away.',
                                })}
                            </span>
                        </div>
                        <button
                            type='button'
                            class='AppPanel__actionLink'
                            disabled={!board()?.id}
                            onClick={showTourAgain}
                        >
                            {intl.formatMessage({id: 'Settings.tour-start', defaultMessage: 'Show'})}
                        </button>
                    </div>

                    {/* The one thing in the old corner that was about the
                        moment rather than about the app, and the last reason
                        that corner existed. It goes next to the guide because
                        the two are one question apart: what is written down,
                        and what to do when it does not match. */}
                    <div class='AppPanel__action'>
                        <div class='AppPanel__actionText'>
                            <span class='AppPanel__actionName'>
                                {intl.formatMessage({id: 'Settings.feedback', defaultMessage: 'Give feedback'})}
                            </span>
                            <span class='AppPanel__actionHint'>
                                {intl.formatMessage({
                                    id: 'Settings.feedback-hint',
                                    defaultMessage: 'Bugs and requests go by email, to {email}.',
                                }, {email: Constants.feedbackEmail})}
                            </span>
                        </div>
                        <a
                            class='AppPanel__actionLink'
                            href={Constants.feedbackUrl}
                        >
                            {intl.formatMessage({id: 'Settings.feedback-open', defaultMessage: 'Write'})}
                        </a>
                    </div>
                </section>
            </div>
        </div>
    )
}

export default AppPanel
