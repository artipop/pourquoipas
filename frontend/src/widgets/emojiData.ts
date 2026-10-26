// The emoji data set, fetched the first time something needs emoji.
//
// emoji-mart 5 no longer bundles its data and has to be handed it once
// (`init`). That used to happen in main.tsx, which put 424 KB of JSON and the
// library itself in front of the first paint — for a picker most people never
// open. Both of its users ask here instead: the picker alongside its chunk, and
// the `:` autocomplete before its first search.
//
// The promise is kept, so the second ask is free and two asks at once make one
// request.
let ready: Promise<void> | undefined

export function ensureEmojiData(): Promise<void> {
    if (!ready) {
        ready = Promise.all([
            import('emoji-mart'),
            import('@emoji-mart/data'),
        ]).then(([{init}, data]) => init({data: data.default})).catch((error) => {
            ready = undefined
            throw error
        })
    }
    return ready
}
