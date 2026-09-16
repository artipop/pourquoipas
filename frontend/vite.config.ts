import {mkdirSync, readdirSync, rmSync} from 'fs'
import path from 'path'
import {fileURLToPath} from 'url'

import formatjs from '@formatjs/unplugin/vite'
import autoprefixer from 'autoprefixer'
import {defineConfig} from 'vite'
import type {Plugin} from 'vite'
import solid from '@solidjs/vite-plugin'

import {solid1Compat} from './solid1/plugin'

const root = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.join(root, 'dist')

// In a build the Go server resolves {{.BaseURL}} when it templates index.html;
// the dev server serves the file as-is, so the placeholder is dropped here.
function appHtml(): Plugin {
    return {
        name: 'xciii-html',
        apply: 'serve',
        transformIndexHtml: {
            order: 'pre',
            handler(html: string) {
                return html.replace(/\{\{\.BaseURL\}\}/g, '')
            },
        },
    }
}

// Empties dist/ before a build, around its .gitkeep — Vite's own emptyOutDir,
// minus the one file it would take with it (see build.emptyOutDir below).
// Clearing is what keeps a renamed asset from surviving a rebuild, and it is
// the build's job rather than the Taskfile's: the Taskfile did it with
// rm/mkdir/touch, which Task's shell does not have on Windows.
function emptyDist(): Plugin {
    return {
        name: 'xciii-empty-dist',
        apply: 'build',
        buildStart() {
            mkdirSync(outDir, {recursive: true})
            for (const name of readdirSync(outDir)) {
                if (name !== '.gitkeep') {
                    rmSync(path.join(outDir, name), {recursive: true, force: true})
                }
            }
        },
    }
}

export default defineConfig(({mode}) => {
    const isProduction = mode === 'production'

    return {
        root,
        base: '/',

        plugins: [

            // formatjs's own universal build plugin, standing in for the
            // @formatjs/ts-transformer that ran under ts-loader: pre-compile
            // messages to an ICU AST, and hash-generate an id only where the
            // source doesn't declare one. Every message in src/ declares an
            // explicit id, which is what keeps them matching i18n/*.json.
            formatjs({
                ast: true,
                idInterpolationPattern: '[sha512:contenthash:base64:6]',
            }),
            solid1Compat(),
            solid(),
            appHtml(),
            emptyDist(),
        ],

        build: {
            outDir,

            // dist/ holds a committed .gitkeep that keeps the directory
            // embeddable for `go mod tidy`, which resolves `go:embed all:`
            // under every tag and runs in parallel with this build. Vite's own
            // emptying would delete it and reopen that race, so emptyDist()
            // above clears around it instead.
            emptyOutDir: false,
            target: 'es2019',
            sourcemap: !isProduction,
            minify: isProduction,
        },

        experimental: {

            // The server rewrites {{.BaseURL}} in index.html, so HTML references
            // carry the placeholder while asset references inside JS/CSS stay
            // relative to the emitting chunk (webpack's `publicPath: 'auto'`).
            renderBuiltUrl(filename, {hostType}) {
                if (hostType === 'html') {
                    return `{{.BaseURL}}/${filename}`
                }
                return {relative: true}
            },
        },

        css: {

            // The desktop app is a WKWebView, and it does not know the
            // unprefixed `user-select` at all — measured, in the real engine:
            // CSS.supports('user-select','none') is false there and
            // -webkit-user-select is what it answers to. So every "do not
            // select this" in the product was inert in the app while working in
            // a browser, and a drag out of a socket on the route canvas painted
            // half the dialog blue.
            //
            // Prefixing by hand is what stylelint's property-no-vendor-prefix
            // forbids, and it is right to: the rule assumes this step exists.
            // It did not.
            postcss: {plugins: [autoprefixer()]},

            preprocessorOptions: {
                scss: {
                    api: 'modern-compiler',
                    silenceDeprecations: ['import', 'global-builtin'],
                },
            },
        },

        // `npm run dev` is what `wails3 dev` starts (`common:dev:frontend`,
        // with the port on the command line), and nobody opens it directly:
        // a development build's front door takes the page and its modules
        // from here and serves them under its own origin with the bootstrap
        // injected, while /api and /ws stay the board's (proxy.go). The HMR
        // client connects back to that origin, and the door passes the upgrade
        // through. So there is no proxy to configure here — the door is it.
        server: {},

        // The dev server pre-bundles dependencies with esbuild, which knows
        // nothing of solid1Compat: a Solid 1 package bundled there resolves
        // `solid-js/web` against Solid 2, which does not publish it, and Vite
        // dies at startup. Left out, the package is served module by module
        // through the plugins like the page's own code (vitest inlines it for
        // the same reason).
        optimizeDeps: {
            exclude: ['@dnd-kit/solid'],
        },
    }
})
