import {defineConfig} from 'vite'

// Two pages, so the entries are listed by hand: on its own Vite finds only the
// index.html at the root. `mpa` keeps the dev server from answering /xciii/
// with the home page.
export default defineConfig({
    appType: 'mpa',

    // On the deploy /docs/ is a folder inside dist/; in dev it is a second
    // server, so the link to the guide in the header would be a 404 exactly
    // where it is being written. The proxy makes the two look like one site
    // here too — `npm run dev` in docs/guide (pinned to 5174) answers it.
    server: {
        proxy: {
            // Three guides, three dev servers, and the more specific paths
            // first: the rules are tried in order, so '/docs' listed above
            // would take /docs/xxvi/ and /docs/vi/ to the XCIII guide.
            //
            // `ws` because a guide's own hot reload talks over a socket on
            // these paths too, and a proxy that forwards only HTTP leaves it
            // reconnecting for ever.
            //
            // The VI guide lives in the six repository next door
            // (../../six/docs/guide) rather than in this one: the browser's
            // documentation ships with the browser. `npm run dev` starts it
            // along with the rest, so nothing has to be remembered here.
            '/docs/xxvi': {target: 'http://localhost:5175', ws: true},
            '/docs/vi': {target: 'http://localhost:5176', ws: true},
            '/docs': {target: 'http://localhost:5174', ws: true},
        },
    },

    // `preview` has no proxy of its own: Vite falls its `proxy` back to
    // `server.proxy` unless it is told otherwise. Inherited, the rules above
    // send /docs, /docs/xxvi and /docs/vi to dev servers that are not running
    // during a preview — so the one thing preview exists to show, the built
    // guides sitting in dist/, is the one thing it would not serve. An empty
    // object is what turns the inheritance off; there is nothing to proxy here,
    // because after `build:all` every guide is a real folder under dist/docs.
    preview: {
        proxy: {},
    },

    build: {
        rollupOptions: {
            input: {
                index: new URL('index.html', import.meta.url).pathname,
                xciii: new URL('xciii/index.html', import.meta.url).pathname,
            },
        },
    },
})
