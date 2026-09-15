import {createReadStream, existsSync, mkdirSync, readdirSync, rmSync} from 'fs'
import {cp} from 'fs/promises'
import path from 'path'
import {fileURLToPath} from 'url'

import formatjs from '@formatjs/unplugin/vite'
import autoprefixer from 'autoprefixer'
import {defineConfig} from 'vite'
import type {Connect, Plugin, ViteDevServer} from 'vite'
import solid from 'vite-plugin-solid'

const root = path.dirname(fileURLToPath(import.meta.url))
const staticDir = path.join(root, 'static')
const outDir = path.join(root, 'pack')

// Where `npm run dev` proxies the API: the standalone server's default port
// from config.json, i.e. whatever `make watch` is running in another terminal.
const devServerURL = 'http://localhost:8000'

const mimeTypes: Record<string, string> = {
    '.gif': 'image/gif',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.png': 'image/png',
    '.svg': 'image/svg+xml',
}

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

// `static/` is served from `/static` in dev and copied to `pack/static` on build
// — the same contract the Go server expects (it mounts pack/static at /static).
// Assets imported from TypeScript are emitted into pack/static by Vite itself.
function appStatic(): Plugin {
    return {
        name: 'xciii-static',
        configureServer(server: ViteDevServer) {
            server.middlewares.use('/static', ((req, res, next) => {
                const rel = decodeURIComponent((req.url || '').split('?')[0])
                const file = path.join(staticDir, rel)
                if (!file.startsWith(staticDir) || !existsSync(file)) {
                    next()
                    return
                }
                const type = mimeTypes[path.extname(file).toLowerCase()]
                if (type) {
                    res.setHeader('Content-Type', type)
                }
                createReadStream(file).pipe(res)
            }) as Connect.NextHandleFunction)
        },
        async closeBundle() {
            await cp(staticDir, path.join(outDir, 'static'), {recursive: true})
        },
    }
}

// Empties pack/ before a build, around its .gitkeep — Vite's own emptyOutDir,
// minus the one file it would take with it (see build.emptyOutDir below).
// Clearing is what keeps a renamed asset from surviving a rebuild, and it is
// the build's job rather than the Taskfile's: the Taskfile did it with
// rm/mkdir/touch, which Task's shell does not have on Windows.
//
// Once per process, as emptyOutDir is: a watch rebuild overwrites in place,
// and emptying pack/ on every one would leave a dev build, which serves it off
// disk, with nothing to serve for the length of each rebuild.
function emptyPack(): Plugin {
    let emptied = false
    return {
        name: 'xciii-empty-pack',
        apply: 'build',
        buildStart() {
            if (emptied) {
                return
            }
            emptied = true
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

        // Handled by appStatic: Vite would copy publicDir to the output
        // root, but these files have to land in pack/static.
        publicDir: false,

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
            solid(),
            appHtml(),
            appStatic(),
            emptyPack(),
        ],

        build: {
            outDir,
            assetsDir: 'static',

            // pack/ holds a committed .gitkeep that keeps the directory
            // embeddable for `go mod tidy`, which resolves `go:embed all:`
            // under every tag and runs in parallel with this build. Vite's own
            // emptying would delete it and reopen that race, so emptyPack()
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

        // `npm run dev` — browser-only dev server with HMR, run alongside
        // `make watch`. The desktop app does not use it: `make dev-wails` keeps
        // serving the page from the Go server and rebuilds pack/ on the side.
        server: {
            port: 5173,
            strictPort: true,
            proxy: Object.fromEntries(
                ['/api', '/plugins', '/files', '/login', '/register', '/logout'].
                    map((route) => [route, {target: devServerURL, changeOrigin: true}]),
            ),
        },
    }
})
