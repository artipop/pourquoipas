// Separate from vite.config.ts rather than a `test` block inside it, because a
// test run wants a different pipeline than a build: no formatjs pre-compilation
// (@formatjs/intl compiles the same messages at runtime, which is what every
// snapshot in the suite was recorded against) and none of the build's output
// wiring. What the two must share is vite-plugin-solid, and one plugin instance
// is exactly what stops solid-js being loaded twice.

import path from 'path'
import {fileURLToPath} from 'url'

import {defineConfig} from 'vitest/config'
import solid from '@solidjs/vite-plugin'
import {playwright} from '@vitest/browser-playwright'

import {solid1Compat} from './solid1/plugin'
import {mouse} from './src/test/mouseCommand'

const root = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
    root,

    // The plugin also settles what jest needed customExportConditions for: in
    // test mode it resolves solid-js to its browser development build, without
    // which a component renders once and never reacts again.
    plugins: [solid1Compat(), solid()],

    resolve: {
        alias: [

            // Vite would resolve an imported asset to a URL that differs between
            // a dev server and a build; the suite wants one stable string, and
            // the snapshots already contain it.
            // The pattern has to match the whole specifier: a regex alias
            // rewrites what it matched, so anchoring it to the extension alone
            // would leave the directories in front of a replaced suffix.
            {
                find: /^.+\.(jpg|jpeg|png|gif|eot|otf|webp|svg|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga)$/,
                replacement: path.join(root, '__mocks__/fileMock.js'),
            },
        ],
    },

    // The page runs in a real browser, where Vite pre-bundles dependencies
    // with esbuild exactly as the dev server does, and for the same reason
    // (vite.config.ts) the Solid 1 package is left out of it.
    //
    // Left out, it imports @dnd-kit/state as it lies in node_modules, while
    // the bundled @dnd-kit/dom carries a copy of its own: two signal runtimes,
    // and an effect in one never hears a write in the other. A column then
    // never lights under a dragged card. Named here, the package is bundled
    // once and both sides import that.
    optimizeDeps: {
        exclude: ['@dnd-kit/solid'],
        include: ['@dnd-kit/state'],
    },

    test: {

        // The suite was written against jest's injected globals, so `describe`,
        // `expect` and `vi` are expected to exist without an import.
        globals: true,

        // Build integration uses Node APIs; UI tests keep their real browser.
        projects: [
            {test: {name: 'ui', include: ['src/**/*.test.{ts,tsx}']}},
            {
                extends: false,
                root,
                test: {name: 'build', globals: true, environment: 'node', include: ['i18n/*.test.ts']},
            },
        ],

        browser: {
            enabled: true,
            provider: playwright(),
            headless: true,
            instances: [{browser: 'chromium'}],

            // A desktop window, as jsdom's was and as the suite was written
            // for. Vitest's default is a phone, below the width at which the
            // sidebar takes itself off screen.
            viewport: {width: 1024, height: 768},

            // A failing test would leave a PNG beside it in the tree, and what
            // failed is in the assertion, not in a picture of the page.
            screenshotFailures: false,

            // A drag a hand could make, for the drag tests (src/test/drag.ts).
            commands: {mouse},
        },

        // The two mocks come first: in a browser a module is loaded once per
        // page, so a mock registered after something imported the module (as
        // flushAfterEvents imports solid-js) replaces nothing. The silent
        // socket goes before any module that may open one while loading.
        //
        // jest-dom is named here because vite-plugin-solid adds it only outside
        // browser mode, where vitest brings matchers of its own under the same
        // names, and its toHaveTextContent matches the whole text where
        // jest-dom's, which the suite was written against, matches a part.
        setupFiles: [
            path.join(root, 'src/test/stableIds.ts'),
            path.join(root, 'src/test/goSideMock.ts'),
            path.join(root, 'src/test/silentWebSocket.ts'),
            path.join(root, 'src/test/appStylesheets.ts'),
            path.join(root, 'src/test/flushAfterEvents.ts'),
            path.join(root, 'src/test/parkPointer.ts'),
            path.join(root, 'src/test/jestDom.ts'),
        ],

        // Half the cores, as under jest: the suite is wide enough that full
        // parallelism buys little beyond making the machine unusable.
        maxWorkers: '50%',

        // Off unless asked for (`npm run test:coverage`): the counters are paid
        // on every run, and nothing reads the report on an ordinary one.
        coverage: {

            // v8's own counters rather than an instrumenting babel pass, which
            // is what let babel out of the toolchain altogether.
            provider: 'v8',
            enabled: false,
            include: ['src/**/*.{ts,tsx,js,jsx}'],
            exclude: ['src/test/**'],
        },
    },
})
