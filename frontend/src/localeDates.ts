// The dates this page says out loud, in the reader's language: "2 hours ago",
// "3 days", and the short format a person types a date in. This is what moment
// did, on Intl instead — no library, no locale bundle to fetch before a date
// stops being English.
//
// Formatters are kept per locale because building one is the expensive half;
// everything else here is arithmetic.

const SECOND = 1000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const relativeFormatters = new Map<string, Intl.RelativeTimeFormat>()
const shortFormatters = new Map<string, Intl.DateTimeFormat>()
const unitFormatters = new Map<string, Intl.NumberFormat>()

function relativeFormatter(locale: string): Intl.RelativeTimeFormat {
    let formatter = relativeFormatters.get(locale)
    if (!formatter) {
        // 'auto' is what gives "yesterday" instead of "1 day ago" where the
        // language has a word for it.
        formatter = new Intl.RelativeTimeFormat(locale, {numeric: 'auto'})
        relativeFormatters.set(locale, formatter)
    }
    return formatter
}

// The short date, spelled the way moment's 'L' was: the parts in the locale's
// own order and a four-digit year. `dateStyle: 'short'` is not the same thing —
// it gives en-US a two-digit year, which is not what anybody typed into this
// field before.
function shortFormatter(locale: string): Intl.DateTimeFormat {
    let formatter = shortFormatters.get(locale)
    if (!formatter) {
        formatter = new Intl.DateTimeFormat(locale, {year: 'numeric', month: '2-digit', day: '2-digit'})
        shortFormatters.set(locale, formatter)
    }
    return formatter
}

function unitFormatter(locale: string, unit: string): Intl.NumberFormat {
    const key = `${locale}|${unit}`
    let formatter = unitFormatters.get(key)
    if (!formatter) {
        formatter = new Intl.NumberFormat(locale, {style: 'unit', unit, unitDisplay: 'long', maximumFractionDigits: 0})
        unitFormatters.set(key, formatter)
    }
    return formatter
}

// The unit a span is said in, largest first, with the thresholds moment used:
// under 45 seconds is still seconds, 45 minutes rounds up to an hour.
const SPANS: Array<{unit: Intl.RelativeTimeFormatUnit, ms: number, upTo: number}> = [
    {unit: 'year', ms: 365 * DAY, upTo: Number.POSITIVE_INFINITY},
    {unit: 'month', ms: 30 * DAY, upTo: 320 * DAY},
    {unit: 'day', ms: DAY, upTo: 26 * DAY},
    {unit: 'hour', ms: HOUR, upTo: 22 * HOUR},
    {unit: 'minute', ms: MINUTE, upTo: 45 * MINUTE},
    {unit: 'second', ms: SECOND, upTo: 45 * SECOND},
]

function spanFor(ms: number): {unit: Intl.RelativeTimeFormatUnit, ms: number} {
    for (let i = SPANS.length - 1; i >= 0; i--) {
        if (ms < SPANS[i].upTo) {
            return SPANS[i]
        }
    }
    return SPANS[0]
}

// relativeTime says how long ago something happened — moment's fromNow().
export function relativeTime(date: Date, locale: string, now: number = Date.now()): string {
    const difference = date.getTime() - now
    const span = spanFor(Math.abs(difference))
    return relativeFormatter(locale).format(Math.round(difference / span.ms), span.unit)
}

// humanizeDuration says how long something lasted, with no "ago" about it —
// moment's duration().humanize(). Intl.DurationFormat would be the direct
// answer and is too new for the browsers this page still supports, so the
// number and its unit are formatted as one quantity instead.
export function humanizeDuration(ms: number, locale: string): string {
    const span = spanFor(Math.abs(ms))
    return unitFormatter(locale, span.unit).format(Math.round(Math.abs(ms) / span.ms))
}

type DatePart = 'day' | 'month' | 'year'

// The order of day, month and year in this locale's short date, with the
// separators that go between them.
function shortDateShape(locale: string): {order: DatePart[], parts: Intl.DateTimeFormatPart[]} {
    // A date whose three numbers cannot be mistaken for one another.
    const sample = new Date(Date.UTC(2033, 10, 22))
    const parts = shortFormatter(locale).formatToParts(sample)
    const order = parts.
        map((part) => part.type).
        filter((type): type is DatePart => type === 'day' || type === 'month' || type === 'year')
    return {order, parts}
}

// shortDatePattern is the hint under an empty date field: the format itself,
// written in letters — moment's longDateFormat('L').
export function shortDatePattern(locale: string): string {
    const {parts} = shortDateShape(locale)
    return parts.map((part) => {
        switch (part.type) {
        case 'day':
            return 'DD'
        case 'month':
            return 'MM'
        case 'year':
            return 'YYYY'
        default:
            return part.value
        }
    }).join('')
}

// parseShortDate reads a date typed in that same format, and refuses anything
// else — moment(value, 'L', locale, true). Strict on purpose: a half-typed date
// must leave the field alone rather than jump to some other day.
export function parseShortDate(value: string, locale: string): Date | undefined {
    const numbers = value.split((/\D+/)).filter((piece) => piece !== '')
    const {order} = shortDateShape(locale)
    if (numbers.length !== order.length) {
        return undefined
    }

    const read: Record<DatePart, number> = {day: 0, month: 0, year: 0}
    for (let i = 0; i < order.length; i++) {
        const digits = numbers[i]
        if (!(/^\d+$/).test(digits)) {
            return undefined
        }
        read[order[i]] = Number(digits)
    }
    if (read.year < 1000 || read.month < 1 || read.month > 12 || read.day < 1 || read.day > 31) {
        return undefined
    }

    const date = new Date(read.year, read.month - 1, read.day)

    // Rolling over — the 31st of a 30-day month — is what strict meant.
    if (date.getFullYear() !== read.year || date.getMonth() !== read.month - 1 || date.getDate() !== read.day) {
        return undefined
    }
    return date
}

// Which day a week starts on, counted the way the calendar widget counts:
// 0 is Sunday. Intl says 1 for Monday through 7 for Sunday, hence the modulo.
//
// getWeekInfo is not everywhere yet (Firefox has none, Safari got it in 17), so
// the fallback is the rule the missing ones follow anyway: Monday, except in
// the places that answer Sunday or Saturday.
const SUNDAY_FIRST = new Set(['en-us', 'en-ca', 'en-au', 'en-nz', 'en-za', 'en-ph', 'en-il', 'ja', 'ko', 'zh', 'zh-hans', 'zh-hant', 'he', 'pt-br', 'es-mx', 'ar-sa'])
const SATURDAY_FIRST = new Set(['ar', 'ar-eg', 'fa'])

export function firstDayOfWeekIn(locale: string): number {
    const tag = locale.toLowerCase()
    type WeekInfo = {getWeekInfo?: () => {firstDay: number}}
    try {
        const info = (new Intl.Locale(tag) as unknown as WeekInfo).getWeekInfo?.()
        if (info) {
            return info.firstDay % 7
        }
    } catch {
        // An unusable tag falls through to the table, like any unknown one.
    }
    if (SUNDAY_FIRST.has(tag) || SUNDAY_FIRST.has(tag.split('-')[0])) {
        return 0
    }
    if (SATURDAY_FIRST.has(tag) || SATURDAY_FIRST.has(tag.split('-')[0])) {
        return 6
    }
    return 1
}
