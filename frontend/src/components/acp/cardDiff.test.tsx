import {render, screen} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'

import {fakeGoSide} from '../../test/goSide'

import CardDiffPanel, {CardDiffView, patchLines} from './cardDiff'

const patch = `diff --git a/auth.go b/auth.go
index 1111111..2222222 100644
--- a/auth.go
+++ b/auth.go
@@ -1,3 +1,3 @@
 package auth
-func Login() {}
+func Login(token string) {}
`

function stubBindings(overrides: Record<string, unknown> = {}) {
    const bindings = {
        GetCardDiff: vi.fn().mockResolvedValue(JSON.stringify({
            cardId: 'card-1',
            branch: 'fix-login',
            base: 'main',
            committed: [{path: 'auth.go', added: 1, removed: 1, patch}],
            uncommitted: [],
        })),
        OpenCardDiff: vi.fn().mockResolvedValue('http://127.0.0.1:1/acp/diff/card-1'),
        ...overrides,
    }
    fakeGoSide(bindings)
    return bindings
}

describe('components/acp/cardDiff', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // A generated file is a real change and belongs in the list, but expanded it
    // buries the few lines somebody opened the panel to read. Folded by its own
    // size rather than by its name: "generated" has no spelling anybody agrees
    // on, and what makes a file unreadable here is how long it is.
    it('starts a very long file folded and a short one open', async () => {
        const long = ['diff --git a/package-lock.json b/package-lock.json', '@@ -1,400 +1,400 @@']
        for (let i = 0; i < 400; i++) {
            long.push('+  "line": ' + i)
        }
        stubBindings({
            GetCardDiff: vi.fn().mockResolvedValue(JSON.stringify({
                cardId: 'card-1',
                committed: [
                    {path: 'package-lock.json', added: 400, removed: 0, patch: long.join('\n')},
                    {path: 'auth.go', added: 1, removed: 1, patch},
                ],
                uncommitted: [],
            })),
        })
        render(() => wrapIntl(() => <CardDiffView cardId='card-1'/>))

        await screen.findByText('auth.go')
        expect(screen.getByText('func Login(token string) {}')).toBeInTheDocument()
        expect(screen.queryByText('  "line": 0')).toBeNull()
    })

    // "All of them" has to reach a file whose own chevron somebody has already
    // clicked, so the two buttons are events rather than a shared open flag.
    it('folds and unfolds every file at once', async () => {
        stubBindings({
            GetCardDiff: vi.fn().mockResolvedValue(JSON.stringify({
                cardId: 'card-1',
                committed: [{path: 'auth.go', added: 1, removed: 1, patch}],
                uncommitted: [{path: 'other.go', added: 1, removed: 0, patch}],
            })),
        })
        render(() => wrapIntl(() => <CardDiffView cardId='card-1'/>))

        await screen.findByText('auth.go')
        await userEvent.click(screen.getByLabelText('Collapse every file'))
        expect(screen.queryByText('func Login(token string) {}')).toBeNull()

        await userEvent.click(screen.getByLabelText('Expand every file'))
        expect(screen.getAllByText('func Login(token string) {}').length).toBeGreaterThan(0)
    })

    // The name of the file is the row above the patch, so repeating the header
    // git puts in front of every hunk is noise the reader has to skip past.
    it('drops git’s own file header and keeps the changed lines', () => {
        const lines = patchLines(patch)
        expect(lines.map((l) => l.kind)).toEqual(['hunk', 'ctx', 'del', 'add'])
        expect(lines[3].text).toBe('func Login(token string) {}')
    })

    it('draws the branch and the files it changed', async () => {
        stubBindings()
        render(() => wrapIntl(() => <CardDiffView cardId='card-1'/>))

        expect(await screen.findByText('fix-login')).toBeInTheDocument()
        expect(screen.getByText('auth.go')).toBeInTheDocument()
        expect(screen.getByText('func Login(token string) {}')).toBeInTheDocument()
    })

    // A card nobody has worked on is an ordinary card, not a failure: the panel
    // says which of the "nothing here" cases this is, in words a person can act
    // on, and never in red.
    it('says why there is nothing to read, without calling it an error', async () => {
        stubBindings({
            GetCardDiff: vi.fn().mockResolvedValue(JSON.stringify({
                cardId: 'card-1',
                committed: [],
                uncommitted: [],
                reason: {code: 'carddiff.not-started', text: 'Nobody has worked on this card yet'},
            })),
        })
        render(() => wrapIntl(() => <CardDiffView cardId='card-1'/>))

        const note = await screen.findByText(/Nobody has worked on this card yet: there is no branch/)
        expect(note).toHaveClass('CardDiff__empty')
    })

    // The panel is the narrowest place in the app to read code in, so the way
    // out of it is the point of having it here at all.
    it('hands the reading to a window of its own', async () => {
        const bindings = stubBindings()
        render(() => wrapIntl(() => <CardDiffPanel cardId='card-1' onClose={vi.fn()}/>))

        await userEvent.click(await screen.findByLabelText('Open in a separate window'))
        expect(bindings.OpenCardDiff).toHaveBeenCalledWith('card-1')
    })
})
