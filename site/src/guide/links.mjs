import {existsSync} from 'node:fs'
import {dirname, relative, resolve} from 'node:path'
import {fileURLToPath} from 'node:url'

// The guide's markdown lives in docs/guide/ and links page to page the way it
// reads on GitHub: relative, with `.md` (`../inbox/index.md`). This file turns
// such a path into the page's address, and is the one place that knows how a
// file under docs/guide/ maps to one.

export const guideDir = resolve(dirname(fileURLToPath(import.meta.url)), '../../../docs/guide')

// `board.md` → `docs/board`, `automation/index.md` → `docs/automation`,
// `en/board.md` → `en/docs/board`: English is a locale, and Starlight reads the
// locale off the first segment.
export function guideId(file) {
    const parts = file.replace(/\.mdx?$/, '').split('/')
    if (parts.at(-1) === 'index') parts.pop()
    const locale = parts[0] === 'en' ? [parts.shift()] : []
    return [...locale, 'docs', ...parts].join('/')
}

// A remark plugin. `base` is the site's path (astro.config.mjs). A link to a
// file that is not there fails the build: a guide with a dead link is a guide
// nobody noticed was wrong.
export function guideLinks({base}) {
    return (tree, file) => {
        if (!file.path?.startsWith(guideDir)) return
        walk(tree, (node) => {
            if (node.type !== 'link' && node.type !== 'definition') return
            const match = /^([^:#?]+\.mdx?)(#.*)?$/.exec(node.url)
            if (!match) return
            const target = resolve(dirname(file.path), match[1])
            if (!existsSync(target)) {
                throw new Error(`${relative(guideDir, file.path)}: the link ${node.url} leads to no page`)
            }
            node.url = `${base}${guideId(relative(guideDir, target))}/${match[2] ?? ''}`
        })
    }
}

function walk(node, visit) {
    visit(node)
    for (const child of node.children ?? []) walk(child, visit)
}
