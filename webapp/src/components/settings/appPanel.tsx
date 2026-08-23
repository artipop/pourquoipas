import {For, Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Constants} from '../../constants'
import {useAppStore} from '../../store/hooks'
import CheckIcon from '../../widgets/icons/check'
import {
    activeThemeConfig,
    customThemeName,
    getActiveThemeName,
    getCustomThemeSource,
    setCustomThemeSource,
    setTheme,
    systemThemeName,
    themeConfigText,
    ThemeConfigError,
    ThemeGroupId,
    ThemeName,
    THEME_GROUPS,
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
// The themes are a dropdown, and they are the one place that rule turned over.
// There were three of them and there are fourteen — the two the product was
// designed in, eight borrowed from Mattermost's own community list, four
// written as a joke — and a row of fourteen chips is a wall, not a choice. It
// is the platform's own `select` rather than this app's menu: a native list
// takes `color-scheme` from the theme being previewed, and it is drawn by the
// window rather than by the page, so the clipping a dialog's own menu has to
// be placed around does not arise.

const AppPanel = () => {
    const intl = useIntl()
    const {actions} = useAppStore()

    // The theme module writes the tokens onto the document element and is not
    // reactive, so which one is current is kept here, seeded with what startup
    // loaded.
    const [themeName, setThemeName] = createSignal(getActiveThemeName())
    const [source, setSource] = createSignal(getCustomThemeSource())
    const [error, setError] = createSignal('')

    // Spelled out rather than built as `Settings.theme-group-${id}`: a computed
    // id is invisible to `npm run i18n-extract`, so it would reach no catalogue
    // and read as a dead entry in every other one.
    const groupTitle = (id: ThemeGroupId): string => {
        switch (id) {
        case 'xciii':
            return intl.formatMessage({id: 'Settings.theme-group-xciii', defaultMessage: 'XCIII'})
        case 'ported':
            return intl.formatMessage({id: 'Settings.theme-group-ported', defaultMessage: 'Ported'})
        default:
            return intl.formatMessage({id: 'Settings.theme-group-acid', defaultMessage: 'Acid'})
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

    // Choosing the custom slot with nothing in it hands over a copy of the
    // theme on screen: a person writing one starts from what they are looking
    // at, and a config field that opens empty is a format to go and look up.
    //
    // The selection is recorded before the theme is applied, and stays put when
    // applying fails. A config that stopped parsing is exactly the case where
    // the field has to be on screen, and hanging it off "the theme applied"
    // made a broken one impossible to select and therefore impossible to fix.
    const pickTheme = (name: ThemeName) => {
        setError('')
        setThemeName(name)
        if (name === customThemeName && !source().trim()) {
            const seeded = themeConfigText({...activeThemeConfig(), id: customThemeName, name: undefined})
            setSource(seeded)
            setCustomThemeSource(seeded)
        }
        try {
            setTheme(name)
        } catch (e) {
            setError(reasonFor(e))
        }
    }

    const choiceClass = (current: boolean) => `AppPanel__choice${current ? ' AppPanel__choice--current' : ''}`

    const applyCustom = () => {
        setCustomThemeSource(source())
        setThemeName(customThemeName)
        try {
            setTheme(customThemeName)
            setError('')
        } catch (e) {
            setError(reasonFor(e))
        }
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
                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        <label for='xciii-theme'>
                            {intl.formatMessage({id: 'Settings.theme', defaultMessage: 'Theme'})}
                        </label>
                    </h4>
                    <select
                        id='xciii-theme'
                        class='AppPanel__select'
                        value={themeName()}
                        onChange={(event) => pickTheme(event.currentTarget.value)}
                    >
                        <option value={systemThemeName}>
                            {intl.formatMessage({id: 'Sidebar.system-theme', defaultMessage: 'System theme'})}
                        </option>
                        <For each={THEME_GROUPS}>
                            {(group) => (
                                <optgroup label={groupTitle(group.id)}>
                                    <For each={group.themes}>
                                        {(theme) => <option value={theme.id}>{theme.name}</option>}
                                    </For>
                                </optgroup>
                            )}
                        </For>
                        <option value={customThemeName}>
                            {intl.formatMessage({id: 'Settings.theme-custom', defaultMessage: 'Custom config'})}
                        </option>
                    </select>

                    <Show when={themeName() === customThemeName}>
                        <div class='AppPanel__custom'>
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
                            <textarea
                                class='AppPanel__code'
                                aria-label={intl.formatMessage({id: 'Settings.theme-custom', defaultMessage: 'Custom config'})}
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
                                    onClick={applyCustom}
                                >
                                    {intl.formatMessage({id: 'Settings.theme-custom-apply', defaultMessage: 'Apply'})}
                                </button>
                                <Show when={error()}>
                                    <span class='AppPanel__customError'>{error()}</span>
                                </Show>
                            </div>
                        </div>
                    </Show>
                </section>

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
                                    aria-pressed={intl.locale.toLowerCase() === language.code}
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
                                {intl.formatMessage({
                                    id: 'Settings.help-hint',
                                    defaultMessage: 'How the board, its columns and its routes work, screen by screen.',
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
