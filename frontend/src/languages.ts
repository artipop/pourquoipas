import list from '../i18n/languages.json'

// The languages the app can be switched to, in the order the menu offers
// them: the code the language setting stores, the catalogue in i18n/ that
// answers for it, and what the menu calls it. A JSON file because the
// translators' table (scripts/i18nTable.mjs) reads the same list.
export const languages: ReadonlyArray<{code: string, file: string, displayName: string}> = list
