import {getCurrentLanguage, storeLanguage as i18nStoreLanguage} from '../i18n'
import {reportWords} from '../goWords'
import {loadAppWords} from '../appWords'

import type {StoreContext} from './context'

import type {RootState} from './index'

// rereads counts the language picks that changed the words the app's own
// boards arrive translated with (appWords.ts): the board page reads them again
// on each.
export type LanguageState = {value: string, rereads: number}

export const initialLanguageState = (): LanguageState => ({value: 'en', rereads: 0})

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
        loadAppWords(lang).then((changed) => {
            if (changed) {
                setState((draft) => {
                    draft.language.rereads += 1
                })
            }
        })
    },
})

export function getLanguage(state: RootState): string {
    return state.language.value
}

export function getLanguageRereads(state: RootState): number {
    return state.language.rereads
}
