import {getCurrentLanguage, storeLanguage as i18nStoreLanguage} from '../i18n'
import {reportWords} from '../goWords'
import {reportLanguage} from '../userSettings'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type LanguageState = {value: string}

export const initialLanguageState = (): LanguageState => ({value: 'en'})

export const createLanguageActions = ({setState}: StoreContext) => ({

    // Synchronous on purpose: by the time the store exists, main.tsx has
    // already read the settings (settingsRead.ts), so this read is the right
    // answer, not a guess. Asked again after a login or a logout.
    fetchLanguage() {
        setState((draft) => {
            draft.language.value = getCurrentLanguage()
        })
    },
    storeLanguage(lang: string) {
        i18nStoreLanguage(lang)
        setState((draft) => {
            draft.language.value = lang
        })
        reportWords(lang)
        reportLanguage(lang)
    },
})

export function getLanguage(state: RootState): string {
    return state.language.value
}
