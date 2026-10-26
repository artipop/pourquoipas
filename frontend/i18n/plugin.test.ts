import {execFile} from 'child_process'
import path from 'path'
import {fileURLToPath, pathToFileURL} from 'url'
import {promisify} from 'util'
import {runInNewContext} from 'vm'

const directory = path.dirname(fileURLToPath(import.meta.url))
const root = path.dirname(directory)
const entry = path.join(directory, 'compiled-fixture.ts')
const exec = promisify(execFile)

// Build in a separate Node process, as production does, then execute its
// output to verify the parser-free runtime as well as the module graph.
async function buildFixture(input: string, code: string): Promise<{code: string, modules: string[]}> {
    const script = `
import formatjs from '@formatjs/unplugin/vite'
import {build} from 'vite'
import {noParserAlias, precompileCatalogues} from ${JSON.stringify(pathToFileURL(path.join(directory, 'plugin.ts')).href)}
const entry = ${JSON.stringify(input)}
const source = ${JSON.stringify(code)}
const output = await build({
    configFile: false,
    root: ${JSON.stringify(root)},
    logLevel: 'silent',
    plugins: [
        formatjs({ast: true}),
        precompileCatalogues(${JSON.stringify(directory)}),
        {
            name: 'message-fixture',
            resolveId(id) { return id === entry ? entry : null },
            load(id) { return id === entry ? source : null },
        },
    ],
    resolve: {alias: [noParserAlias]},
    build: {
        write: false,
        minify: false,
        rollupOptions: {input: entry, preserveEntrySignatures: 'strict', output: {format: 'iife', name: 'messages'}},
    },
})
const chunk = output.output.find(file => file.type === 'chunk')
process.stdout.write(JSON.stringify({code: chunk.code, modules: Object.keys(chunk.modules)}))
`
    const {stdout} = await exec(process.execPath, ['--input-type=module', '-e', script], {cwd: root, maxBuffer: 10 * 1024 * 1024})
    return JSON.parse(stdout)
}

// Run the browser build pipeline, then execute the emitted code. Ordinary
// component tests use raw catalogues and cannot catch missing precompilation.
const source = `
import {createIntl, IntlErrorCode} from '@formatjs/intl'
import languages from './languages.json'
export {languages}
export const catalogues = import.meta.glob(['./*.json', '!./languages.json'], {eager: true, import: 'default'})
const onError = (error) => { if (error.code !== IntlErrorCode.MISSING_TRANSLATION) throw error }
export function russian(count) {
    return createIntl({locale: 'ru', messages: catalogues['./ru.json'], onError}).formatMessage({id: 'GroupBy.showHiddenGroups'}, {count})
}
export function richText() {
    return createIntl({locale: 'en', messages: catalogues['./en.json'], onError}).formatMessage(
        {id: 'FindBoardsDialog.SubTitle'}, {b: (chunks) => '[' + chunks.join('') + ']'},
    )
}
export function fallback(count) {
    return createIntl({locale: 'ru', defaultLocale: 'en', messages: {}, onError}).formatMessage(
        {id: 'fixture.fallback', defaultMessage: '{count, plural, one {# item} other {# items}}'}, {count},
    )
}
`

describe('precompiled message builds', () => {
    test('formats catalogues and source defaults without a runtime parser', async () => {
        const {code, modules} = await buildFixture(entry, source)
        expect(modules.some((id) => id.endsWith('/icu-messageformat-parser/no-parser.js'))).toBe(true)
        expect(modules.some((id) => id.endsWith('/icu-messageformat-parser/index.js'))).toBe(false)
        expect(modules.some((id) => id.includes('/icu-skeleton-parser/'))).toBe(false)

        const compiled = runInNewContext(`${code}\nmessages`)
        expect(Object.keys(compiled.catalogues)).toHaveLength(17)
        for (const catalogue of Object.values(compiled.catalogues) as Array<Record<string, unknown>>) {
            expect(Object.values(catalogue).every(Array.isArray)).toBe(true)
        }
        expect(compiled.languages[0].code).toBe('en')
        expect([1, 2, 5, 11, 21].map(compiled.russian)).toEqual([
            'Показать 1 скрытую группу',
            'Показать 2 скрытые группы',
            'Показать 5 скрытых групп',
            'Показать 11 скрытых групп',
            'Показать 21 скрытую группу',
        ])
        expect(compiled.richText()).toBe('Type to find a board. Use [UP/DOWN] to browse. [ENTER] to select, [ESC] to dismiss')
        expect(compiled.fallback(1)).toBe('1 item')
        expect(compiled.fallback(5)).toBe('5 items')
    })

    test('reports the catalogue and message id when ICU syntax is invalid', async () => {
        const broken = path.join(directory, 'broken.json')
        await expect(buildFixture(broken, JSON.stringify({'broken.message': '{count, plural,'}))).rejects.toThrow('broken.json: broken.message:')
    })
})
