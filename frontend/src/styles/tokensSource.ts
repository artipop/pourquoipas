// `_tokens.scss` read as data, for the two suites that check it.
//
// It is a module rather than a copy inside each test because the alternative
// is two parsers of one file, and the second one goes wrong — which is the
// same bug the palette itself was collapsed into one file to avoid.
//
// Nothing in the running product imports this: it reaches for `fs`, and it is
// here because the file it parses is here.

import {readFileSync} from 'fs'
import {join} from 'path'

export type Vars = Record<string, string>

export const tokensPath = join(__dirname, '_tokens.scss')

export function tokensSource(): string {
    return readFileSync(tokensPath, 'utf8')
}

// The file is three `:root` blocks: light, dark, and the derived layer both
// share. Parsed in order, later declarations winning, which is what the
// cascade does with them too.
export function blocks(source = tokensSource()): Array<{selector: string, vars: Vars}> {
    const withoutComments = source.replace(/\/\/.*$/gm, '')
    const out: Array<{selector: string, vars: Vars}> = []
    const re = (/([^{}]+)\{([^{}]*)\}/g)
    let match = re.exec(withoutComments)
    while (match) {
        const vars: Vars = {}
        for (const line of match[2].split(';')) {
            const decl = (/^\s*(--[\w-]+)\s*:\s*(.+?)\s*$/).exec(line)
            if (decl) {
                vars[decl[1]] = decl[2]
            }
        }
        out.push({selector: match[1].trim(), vars})
        match = re.exec(withoutComments)
    }
    return out
}

export function themeVars(dark: boolean, source = tokensSource()): Vars {
    const vars: Vars = {}
    for (const block of blocks(source)) {
        const isDarkBlock = block.selector.includes('data-theme')
        if (isDarkBlock && !dark) {
            continue
        }
        Object.assign(vars, block.vars)
    }
    return vars
}

// Just the raw block of one theme — what a config has to reproduce. The
// derived layer is everything the two theme blocks do not declare, and it is
// written in terms of them, so it is not a config's business.
export function rawThemeVars(dark: boolean, source = tokensSource()): Vars {
    const wanted = dark ? "data-theme='dark'" : null
    for (const block of blocks(source)) {
        if (wanted ? block.selector.includes(wanted) : block.selector === ':root' && block.vars['--canvas-rgb']) {
            return block.vars
        }
    }
    throw new Error(`no ${dark ? 'dark' : 'light'} theme block in _tokens.scss`)
}
