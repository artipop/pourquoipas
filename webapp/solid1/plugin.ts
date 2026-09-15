import path from 'path'
import {fileURLToPath} from 'url'

import type {Plugin} from 'vite'

const here = path.dirname(fileURLToPath(import.meta.url))

// Which packages still speak Solid 1. `@dschz/solid-flow` came off the list
// with its 1.0 line, which is built for Solid 2 and brings Solid 2 builds of
// the `@solid-primitives/*` it depends on.
const solid1Packages = [
    '/node_modules/@dnd-kit/',
]

const shims: Record<string, string> = {
    'solid-js': path.join(here, 'core.js'),
    'solid-js/web': path.join(here, 'web.js'),
    'solid-js/store': path.join(here, 'store.js'),
}

// Scoped by importer on purpose: `@solidjs/web` imports `solid-js` too, and a
// global alias would build the Solid 2 renderer on Solid 1 effect semantics.
export const solid1Compat = (): Plugin => ({
    name: 'solid1-compat',
    enforce: 'pre',
    resolveId(source, importer) {
        if (!importer || !solid1Packages.some((pkg) => importer.includes(pkg))) {
            return null
        }
        return shims[source] ?? null
    },
})
