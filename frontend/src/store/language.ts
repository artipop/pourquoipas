import {storePath} from 'solid-js'

import {getCurrentLanguage, storeLanguage as i18nStoreLanguage} from '../i18n'
import {reportWords} from '../goWords'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type LanguageState = {value: string}

export const initialLanguageState = (): LanguageState => ({value: 'en'})

export const createLanguageActions = ({setState}: StoreContext) => ({

    // Synchronous on purpose: by the time the store exists, main.tsx has
    // already hydrated localStorage from the install's own memory
    // (ui-settings.json), so this read is the right answer, not a guess.
    fetchLanguage() {
        setState(storePath('language', 'value', getCurrentLanguage()))
    },
    storeLanguage(lang: string) {
        i18nStoreLanguage(lang)
        setState(storePath('language', 'value', lang))
        reportWords(lang)
    },
})

export function getLanguage(state: RootState): string {
    return state.language.value
}
