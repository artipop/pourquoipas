import css from '@mattermost/compass-icons/css/compass-icons.css?raw'

// An icon name the font does not have renders as nothing: the `<i>` is still
// there, still takes its width and line-height, so the button beside it sits
// visibly crooked with no glyph in it. Nothing else says so — no error, no
// warning, and in a test the element is present and the name is on it.
//
// So the names are checked against the font itself, the way the palette is
// checked against `_tokens.scss`: read the dependency, not a copy of it.
// Three names were wrong when this was written, all in the diff viewer, and
// what found them was somebody looking at the screen.

function fontNames(): Set<string> {
    const names = new Set<string>()
    for (const m of css.matchAll(/^\.icon-([a-z0-9-]+):before/gm)) {
        names.add(m[1])
    }
    return names
}

// The page's own code, as text, keyed by its path; tests are not callers.
const sources = import.meta.glob<string>(['/src/**/*.{ts,tsx}', '!/src/**/*.test.{ts,tsx}'], {query: '?raw', import: 'default', eager: true})

describe('widgets/icons/compassIcon', () => {
    it('is never asked for a glyph the font has not got', () => {
        const known = fontNames()
        expect(known.size).toBeGreaterThan(100)

        // Only the names written as literals. A name computed at run time is
        // the caller's to get right, and there is nothing here to check it
        // against.
        const wrong: string[] = []
        for (const [path, source] of Object.entries(sources)) {
            for (const m of source.matchAll(/icon=['"]([a-z0-9-]+)['"]/g)) {
                if (!known.has(m[1])) {
                    wrong.push(`${m[1]} (${path.slice(1)})`)
                }
            }
        }
        expect(wrong).toEqual([])
    })
})
