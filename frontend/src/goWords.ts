// The eslint new-cap rule takes the Go bindings' PascalCase for constructors.
/* eslint-disable new-cap */
import {createIntl, IntlShape} from './intl'
import {getMessages} from './i18n'
import {goApp} from './goBridge'
import {goSentences} from './goMessages'

// The words the Go side draws on this page's behalf: the menu bar, the OS
// notification and the titles of the windows it opens (gowords.go). They are
// ours like every other string, and handed over because Go draws them when
// there may be no page to ask — the board's window closed, the app started by a
// share. Sent at boot and again when the language changes.

// How many counts get a plural category of their own; past it Go says
// "other". The menu names six waiting agents and counts the rest.
const pluralCounts = 100

type PluralWords = {
    forms: Record<string, string>
    counts: string[]
}

export type GoWords = {
    agentAsking: string
    anAgentAsking: string
    openToAnswer: string
    open: string
    quit: string
    diffWindow: string
    shareWindow: string
    moreWaits: PluralWords
    messages: Record<string, string>
}

// pluralWords turns a message with a {count} into what Go can use without a
// plural rule of its own: one sentence per CLDR category, with {count} left in
// it, and the category of each count. The rule is Intl's, the same one the
// message itself is formatted by.
function pluralWords(intl: IntlShape, render: (count: number) => string): PluralWords {
    const rules = new Intl.PluralRules(intl.locale)
    const counts: string[] = []
    const forms: Record<string, string> = {}
    for (let n = 0; n < pluralCounts; n++) {
        const category = rules.select(n)
        counts.push(category)
        if (!(category in forms)) {
            forms[category] = render(n).replace(intl.formatNumber(n), '{count}')
        }
    }
    if (!('other' in forms)) {
        forms.other = render(pluralCounts).replace(intl.formatNumber(pluralCounts), '{count}')
    }
    return {forms, counts}
}

export function goWords(intl: IntlShape): GoWords {
    return {
        agentAsking: intl.formatMessage({id: 'Attention.asking-agent', defaultMessage: '{agent} is asking'}, {agent: '{agent}'}),
        anAgentAsking: intl.formatMessage({id: 'Attention.asking', defaultMessage: 'The agent is asking'}),
        openToAnswer: intl.formatMessage({id: 'Tray.open-to-answer', defaultMessage: 'Open the board to answer.'}),
        open: intl.formatMessage({id: 'Tray.open', defaultMessage: 'Open'}),
        quit: intl.formatMessage({id: 'Tray.quit', defaultMessage: 'Quit'}),
        diffWindow: intl.formatMessage({id: 'CardDiff.window-title', defaultMessage: 'Changes · {title}'}, {title: '{title}'}),
        shareWindow: intl.formatMessage({id: 'Share.title', defaultMessage: 'Save to a board'}),
        moreWaits: pluralWords(intl, (count) => intl.formatMessage({id: 'Tray.more', defaultMessage: '…and {count} more'}, {count})),
        messages: goSentences(intl),
    }
}

// reportWords hands the words over in a language. Nothing to do without a Go
// side, and a refusal leaves Go on the words it had.
export async function reportWords(lang: string): Promise<void> {
    const bindings = goApp()
    if (!bindings?.UseWords) {
        return
    }
    const intl = createIntl({locale: lang.split(/[_]/)[0], defaultLocale: 'en', messages: getMessages(lang)})
    try {
        await bindings.UseWords(JSON.stringify(goWords(intl)))
    } catch {
        // An older Go side, or a binding that threw.
    }
}
