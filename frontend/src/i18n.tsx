import {createSignal} from 'solid-js'

import en from '../i18n/en.json'

import {UserSettings} from './userSettings'
import {languages} from './languages'
import type {Messages} from './intl'

const supportedLanguages: string[] = languages.map((l) => l.code)

// Each catalogue is a chunk of its own, fetched when its language is asked
// for: bundling all of them put every language into the main chunk whatever
// the page was read in. English is the defaults and is always there.
const catalogues = import.meta.glob<Messages>(['../i18n/*.json', '!../i18n/en.json', '!../i18n/languages.json'], {import: 'default'})

const loaded = new Map<string, Messages>([['en', en]])

// A catalogue arriving has to reach a page already drawn in English, so what
// getMessages returns is tracked.
const [arrivals, setArrivals] = createSignal(0)

function fileOf(lang: string): string {
    return languages.find((l) => l.code === lang)?.file ?? 'en'
}

// loadMessages fetches a language's catalogue once. Awaited before the first
// render, so the page never paints in English first.
export async function loadMessages(lang: string): Promise<void> {
    const file = fileOf(lang)
    const load = catalogues[`../i18n/${file}.json`]
    if (loaded.has(file) || !load) {
        return
    }
    loaded.set(file, await load())
    setArrivals((n) => n + 1)
}

// getMessages is the catalogue for a language, or English until it has
// arrived.
export function getMessages(lang: string): Messages {
    arrivals()
    return loaded.get(fileOf(lang)) ?? en
}

// getCurrentLanguage reads what the person picked — read before the first
// render (settingsRead.ts) — else the OS language the webview reports, else
// English.
export function getCurrentLanguage(): string {
    let lang = UserSettings.language
    if (!lang) {
        if (supportedLanguages.includes(navigator.language)) {
            lang = navigator.language
        } else if (supportedLanguages.includes(navigator.language.split(/[-_]/)[0])) {
            lang = navigator.language.split(/[-_]/)[0]
        } else {
            lang = 'en'
        }
    }
    return lang
}

// storeLanguage goes through UserSettings, which keeps it with the person.
export function storeLanguage(lang: string): void {
    UserSettings.language = lang
    loadMessages(lang)
}
