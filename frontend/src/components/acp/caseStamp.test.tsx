import {render, screen, waitFor} from '@solidjs/testing-library'
import '@testing-library/jest-dom'

import {fakeGoSide} from '../../test/goSide'

import CaseStamp from './caseStamp'

function cardBindings(state: any) {
    return {GetCardAgent: vi.fn().mockResolvedValue(JSON.stringify(state))}
}

describe('components/acp/caseStamp', () => {
    beforeEach(() => {
        vi.clearAllMocks()
        fakeGoSide()
    })

    // A card nobody has run an agent on is not a case yet, and a line of empty
    // labels under its title would be noise on every card on the board.
    it('says nothing about a card no agent has touched', async () => {
        fakeGoSide(cardBindings({}))

        const {container} = render(() => <CaseStamp cardId='card-1'/>)

        await waitFor(() => expect(container.querySelector('.CaseStamp')).toBeNull())
    })

    // Where the work lives is the thing a person opening a card wants first —
    // and it is one place, not two. A copy is the branch checked out
    // elsewhere, and stamping «branch» and «worktree» side by side read as the
    // app having created two things: the complaint this screen started from.
    it('stamps one line for where the work lives, plus the session', async () => {
        fakeGoSide(cardBindings({
            session: {
                status: 'finished',
                branch: 'pourquoipas/card-1',
                worktree: '/Users/someone/work/pourquoipas-card-1',
            },
        }))

        render(() => <CaseStamp cardId='card-2'/>)

        expect(await screen.findByText('pourquoipas/card-1')).toBeInTheDocument()
        expect(await screen.findByText('finished')).toBeInTheDocument()

        // The copy's path rides in the line's tooltip; it is not a second
        // line, because it is not a second thing.
        expect(screen.getByTitle('/Users/someone/work/pourquoipas-card-1')).toBeInTheDocument()
        expect(screen.queryByText('pourquoipas-card-1')).toBeNull()
    })

    // The line is named after the folder's setting. A person who chose
    // a copy of its own and reads BRANCH off the card concludes the setting
    // did not take — which is exactly what happened.
    it('names the line worktree when the workspace is a copy, and says the base', async () => {
        fakeGoSide(cardBindings({
            work: {mode: 'worktree', base: 'main', started: true, here: true},
            session: {
                status: 'finished',
                branch: 'b-41aa6e-cabgjc3c',
                worktree: '/data/worktrees/groql-backend-cabgjc3c',
            },
        }))

        render(() => <CaseStamp cardId='card-5'/>)

        expect(await screen.findByText('worktree')).toBeInTheDocument()
        expect(screen.queryByText('branch')).toBeNull()

        // One line still: the branch is the value (the handle git commands
        // take), the copy's path is the tooltip.
        expect(screen.getByText('b-41aa6e-cabgjc3c')).toBeInTheDocument()
        expect(screen.getByTitle('/data/worktrees/groql-backend-cabgjc3c')).toBeInTheDocument()

        // What it was cut from — and therefore where it merges.
        expect(screen.getByText('from')).toBeInTheDocument()
        expect(screen.getByText('main')).toBeInTheDocument()
    })

    // A resumable terminal is a case still open, and it names its branch even
    // though no session is running right now.
    it('falls back to what a resumable terminal knows', async () => {
        fakeGoSide(cardBindings({
            resume: {available: true, branch: 'pourquoipas/card-9', cwd: '/tmp/wt/card-9'},
        }))

        render(() => <CaseStamp cardId='card-3'/>)

        expect(await screen.findByText('pourquoipas/card-9')).toBeInTheDocument()
        expect(screen.queryByText('card-9')).toBeNull()
    })

    // A folder with no branch — an ordinary folder, or the board's drafts —
    // is the one case the directory itself is the fact worth stamping.
    it('stamps the folder when there is no branch', async () => {
        fakeGoSide(cardBindings({
            resume: {available: true, cwd: '/Users/someone/notes'},
        }))

        render(() => <CaseStamp cardId='card-4'/>)

        expect(await screen.findByText('notes')).toBeInTheDocument()
    })
})
