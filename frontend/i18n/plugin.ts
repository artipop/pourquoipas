import path from 'path'

import {parse} from '@formatjs/icu-messageformat-parser'
import {normalizePath} from 'vite'
import type {Plugin} from 'vite'

// Only the app's catalogues are ICU messages. languages.json is metadata;
// calendar and emoji JSON belongs to those libraries and stays untouched.
export function precompileCatalogues(directory: string): Plugin {
    const catalogueDir = normalizePath(directory)
    return {
        name: 'xciii-i18n-catalogues',
        apply: 'build',
        enforce: 'pre',
        transform(code, id) {
            if (normalizePath(path.dirname(id)) !== catalogueDir || !id.endsWith('.json') || path.basename(id) === 'languages.json') {
                return null
            }
            const messages = JSON.parse(code) as Record<string, string>
            const compiled = Object.fromEntries(Object.entries(messages).map(([key, message]) => {
                try {
                    return [key, parse(message)]
                } catch (error) {
                    return this.error(`${path.basename(id)}: ${key}: ${(error as Error).message}`)
                }
            }))

            // Vite's JSON plugin turns the transformed JSON into a JS module.
            // Source catalogues remain strings for extraction and translators.
            return {code: JSON.stringify(compiled), map: null}
        },
    }
}

// Match the package entry only, so its no-parser subpath is not rewritten.
export const noParserAlias = {
    find: /^@formatjs\/icu-messageformat-parser$/,
    replacement: '@formatjs/icu-messageformat-parser/no-parser.js',
}
