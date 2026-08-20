// The Solid 1 `solid-js/web` surface, on Solid 2. See README.md.
//
// The three added here are what the Solid 1 compiler emitted for `use:`,
// `on:` and `classList:` — a bundle compiled before Solid 2 still calls them
// even though nothing in this repository writes those forms any more.

export * from '@solidjs/web'

export const use = (fn, element, arg) => fn(element, arg ?? (() => true))

export const addEventListener = (node, name, handler, delegate) => {
    if (delegate) {
        node[`$$${name}`] = handler
        return
    }
    if (Array.isArray(handler)) {
        node.addEventListener(name, (e) => handler[0](handler[1], e))
        return
    }
    node.addEventListener(name, handler)
}

export const classList = (node, value = {}, prev = {}) => {
    for (const key in prev) {
        if (!value[key]) {
            node.classList.toggle(key, false)
        }
    }
    for (const key in value) {
        node.classList.toggle(key, Boolean(value[key]))
    }
    return value
}
