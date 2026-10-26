const loaded = new Map<string, Promise<unknown>>()

// Cache readiness as well as bytes: the lazy picker waits for its translation
// before creating the custom element. A rejected request can be tried again.
export function loadEmojiLocale(locale: string): Promise<unknown> {
    const language = locale.toLowerCase().split('-')[0]
    const cached = loaded.get(language)
    if (cached) {
        return cached
    }
    const request = importLocale(language).catch((error) => {
        loaded.delete(language)
        throw error
    })
    loaded.set(language, request)
    return request
}

async function importLocale(language: string): Promise<unknown> {
    switch (language) {
    case 'de':
        return (await import('@emoji-mart/data/i18n/de.json')).default
    case 'es':
        return (await import('@emoji-mart/data/i18n/es.json')).default
    case 'fr':
        return (await import('@emoji-mart/data/i18n/fr.json')).default
    case 'it':
        return (await import('@emoji-mart/data/i18n/it.json')).default
    case 'ja':
        return (await import('@emoji-mart/data/i18n/ja.json')).default
    case 'nl':
        return (await import('@emoji-mart/data/i18n/nl.json')).default
    case 'pt':
        return (await import('@emoji-mart/data/i18n/pt.json')).default
    case 'ru':
        return (await import('@emoji-mart/data/i18n/ru.json')).default
    case 'tr':
        return (await import('@emoji-mart/data/i18n/tr.json')).default
    case 'zh':
        return (await import('@emoji-mart/data/i18n/zh.json')).default
    default:
        return undefined
    }
}
