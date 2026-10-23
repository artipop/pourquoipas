import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'
import {TestBlockFactory} from '../../test/testBlockFactory'

import {fakeGoSide} from '../../test/goSide'

import WorkdirsDialog from './workdirsDialog'
import {SHOW_GIT} from './gitShown'

vi.mock('../../mutator')

describe('components/acp/workdirsDialog', () => {
    const board = TestBlockFactory.createBoard()

    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // The choice is on the repository's own row: it is a fact about that
    // repository, so it holds on every board the folder is offered on.
    test.skipIf(!SHOW_GIT)('asks how a repository is worked in, on its own row', async () => {
        const bindings = {
            ListAgentWorkdirs: vi.fn().mockResolvedValue(JSON.stringify([
                {name: 'code', path: '/tmp/code', git: true, base: 'main', mode: 'worktree'},
            ])),
            SetAgentWorkdirMode: vi.fn().mockResolvedValue('{}'),
            PickDirectory: vi.fn(),
            AddAgentWorkdir: vi.fn(),
            RemoveAgentWorkdir: vi.fn(),
        }
        fakeGoSide(bindings)

        render(() => wrapIntl(() =>
            <WorkdirsDialog
                board={board}
                onClose={vi.fn()}
            />,
        ))

        expect(await screen.findByText('code')).toBeInTheDocument()

        userEvent.click(screen.getByRole('button', {name: 'in the folder itself'}))
        await waitFor(() => expect(bindings.SetAgentWorkdirMode).toHaveBeenCalledWith('code', board.id, 'branch'))
    })
})
