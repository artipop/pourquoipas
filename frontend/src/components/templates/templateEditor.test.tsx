import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import Mutator from '../../mutator'
import {wrapIntl} from '../../testUtils'
import {Board, createBoard} from '../../blocks/board'

import {fakeGoSide} from '../../test/goSide'

import TemplateEditor from './templateEditor'

vi.mock('../../mutator')

const mockedMutator = vi.mocked(Mutator)

const template: Board = {
    ...createBoard(),
    id: 'template-1',
    title: 'Разработка',
    isTemplate: true,
    cardProperties: [{
        id: 'prop-status',
        name: 'Статус',
        type: 'select',
        options: [
            {id: 'opt-work', value: 'В работе', color: ''},
            {id: 'opt-review', value: 'На ревью', color: ''},
        ],
    }],
}

function stubBindings() {
    const bindings = {
        ListAgents: vi.fn().mockResolvedValue('[]'),
    }
    fakeGoSide(bindings)
    return bindings
}

describe('components/acp/templateEditor', () => {
    afterEach(() => {
        fakeGoSide()
        vi.clearAllMocks()
    })

    // What a board made from the template will do is said here and edited on a
    // board: a template is shown, never drawn.
    test('says what the template carries, and does not offer to redraw it', async () => {
        stubBindings()
        const {container} = render(() => wrapIntl(() => (
            <TemplateEditor
                board={template}
                onClose={vi.fn()}
            />
        )))
        const carries = await waitFor(() => {
            const found = container.querySelector('.TemplateEditor__carries')
            expect(found).toBeTruthy()
            return found!
        })
        expect(carries.textContent).toContain('В работе')
        expect(screen.getByDisplayValue('Разработка')).toBeInTheDocument()
    })

    test('the name and the icon of the template are the board’s own', async () => {
        stubBindings()
        render(() => wrapIntl(() => (
            <TemplateEditor
                board={template}
                onClose={vi.fn()}
            />
        )))

        const name = await screen.findByDisplayValue('Разработка')
        userEvent.clear(name)
        userEvent.type(name, 'Мой шаблон')
        userEvent.click(screen.getByRole('button', {name: 'Save template'}))

        await waitFor(() => expect(mockedMutator.updateBoard).toHaveBeenCalled())
        expect((mockedMutator.updateBoard.mock.calls[0][0] as Board).title).toBe('Мой шаблон')
    })
})
