// The catalogues go out to a translator and come back, and a spreadsheet is
// what a translator works in. This carries them both ways:
//
//   node scripts/i18nTable.mjs export   → i18n/translations.csv
//   node scripts/i18nTable.mjs import   ← i18n/translations.csv
//
// Only the languages the app can actually switch to (i18n/languages.json) get a column: a
// translation for a language the app cannot select is work nobody will see.
// English and Russian are the reference columns and are never written back —
// en.json is generated from the source by `npm run i18n-extract`, and Russian
// is maintained here.

import fs from 'fs'
import path from 'path'
import {fileURLToPath} from 'url'

const here = path.dirname(fileURLToPath(import.meta.url))
const i18nDir = path.join(here, '..', 'i18n')
const csvPath = path.join(i18nDir, 'translations.csv')

// The files in i18n/ the language menu offers, with what it calls them, so a
// translator gets a column heading rather than a file name. English and
// Russian are the reference pair and are left out here.
const languages = JSON.parse(fs.readFileSync(path.join(i18nDir, 'languages.json'), 'utf8')).
    filter((l) => l.file !== 'en' && l.file !== 'ru').
    map((l) => [l.file, l.displayName])

function read(name) {
    const file = path.join(i18nDir, `${name}.json`)
    if (!fs.existsSync(file)) {
        return {}
    }
    const text = fs.readFileSync(file, 'utf8').trim()
    return text ? JSON.parse(text) : {}
}

function write(name, messages) {
    const sorted = {}
    for (const key of Object.keys(messages).sort()) {
        sorted[key] = messages[key]
    }
    fs.writeFileSync(path.join(i18nDir, `${name}.json`), `${JSON.stringify(sorted, null, 2)}\n`)
}

function csvCell(value) {
    return `"${String(value).replace(/"/g, '""')}"`
}

// A hand-rolled reader because the file comes back from a spreadsheet, where
// a translated string may well hold a comma, a quote or a newline of its own.
function parseCsv(text) {
    const rows = []
    let row = []
    let cell = ''
    let quoted = false

    for (let i = 0; i < text.length; i++) {
        const c = text[i]
        if (quoted) {
            if (c === '"') {
                if (text[i + 1] === '"') {
                    cell += '"'
                    i++
                } else {
                    quoted = false
                }
            } else {
                cell += c
            }
        } else if (c === '"') {
            quoted = true
        } else if (c === ',') {
            row.push(cell)
            cell = ''
        } else if (c === '\n') {
            row.push(cell)
            rows.push(row)
            row = []
            cell = ''
        } else if (c !== '\r') {
            cell += c
        }
    }
    if (cell !== '' || row.length > 0) {
        row.push(cell)
        rows.push(row)
    }
    return rows
}

function doExport() {
    const en = read('en')
    const ru = read('ru')
    const catalogues = languages.map(([name]) => read(name))

    const header = ['key', 'en — English', 'ru — Русский', ...languages.map(([name, label]) => `${name} — ${label}`)]
    const lines = [header.map(csvCell).join(',')]

    let gaps = 0
    for (const key of Object.keys(en).sort()) {
        const cells = [key, en[key], ru[key] ?? '']
        for (const catalogue of catalogues) {
            const value = catalogue[key]
            if (value === undefined) {
                gaps++
            }
            cells.push(value ?? '')
        }
        lines.push(cells.map(csvCell).join(','))
    }

    // The BOM is what makes a spreadsheet open this as UTF-8 rather than as
    // whatever the machine's locale happens to be, which is the difference
    // between Русский and Ð ÑƒÑ ÑÐºÐ¸Ð¹.
    fs.writeFileSync(csvPath, `﻿${lines.join('\n')}\n`)
    console.log(`${csvPath}: ${Object.keys(en).length} keys × ${languages.length} languages, ${gaps} blanks`)
}

function doImport() {
    const en = read('en')
    const rows = parseCsv(fs.readFileSync(csvPath, 'utf8').replace(/^﻿/, ''))
    const header = rows.shift()
    if (!header || header[0] !== 'key') {
        throw new Error('translations.csv does not start with a key column')
    }

    // Columns are found by the language name in the heading rather than by
    // position: a spreadsheet comes back with columns moved, and silently
    // filing Catalan under German is not a mistake anyone would catch.
    const columns = languages.map(([name]) => ({
        name,
        index: header.findIndex((cell) => cell.split('—')[0].trim() === name),
    }))

    for (const {name, index} of columns) {
        if (index < 0) {
            console.log(`${name}: no column, left alone`)
            continue
        }
        const catalogue = read(name)
        let added = 0
        let changed = 0
        for (const row of rows) {
            const key = row[0]
            const value = (row[index] ?? '').trim()
            if (!key || !value || !(key in en)) {
                continue
            }
            if (!(key in catalogue)) {
                added++
            } else if (catalogue[key] !== value) {
                changed++
            }
            catalogue[key] = value
        }

        // An id the source no longer uses is a line nobody can see, and the
        // translator was never shown it.
        for (const key of Object.keys(catalogue)) {
            if (!(key in en)) {
                delete catalogue[key]
            }
        }
        write(name, catalogue)
        console.log(`${name}: ${Object.keys(catalogue).length} keys (+${added} new, ${changed} changed)`)
    }
}

const command = process.argv[2]
if (command === 'export') {
    doExport()
} else if (command === 'import') {
    doImport()
} else {
    console.error('usage: node scripts/i18nTable.mjs export|import')
    process.exit(1)
}
