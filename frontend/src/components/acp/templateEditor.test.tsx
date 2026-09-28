import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import Mutator from '../../mutator'
import {wrapIntl} from '../../testUtils'
import {Board, createBoard} from '../../blocks/board'

import {fakeGoSide} from '../../test/goSide'

import TemplateEditor from './templateEditor'
import {BOARD_PROP_SETUP} from './automation'

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
        ListSetupSteps: vi.fn().mockResolvedValue(JSON.stringify([
            {kind: 'agent', registry: 'agents', optional: false},
            {kind: 'deploy', registry: 'deploys', optional: true},
            {kind: 'done', optional: false},
        ])),
        ListAgents: vi.fn().mockResolvedValue('[]'),
        ListDeployTargets: vi.fn().mockResolvedValue('[]'),
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

    test('saving writes the questions into the board', async () => {
        stubBindings()
        const onClose = vi.fn()
        render(() => wrapIntl(() => (
            <TemplateEditor
                board={template}
                onClose={onClose}
            />
        )))

        // Until the steps are named they are worked out from the automation, so
        // the property is not written at all — which is what Go reads as "this
        // template said nothing about it".
        userEvent.click(await screen.findByRole('button', {name: 'Name the steps'}))

        // One hint box per step that asks something; the first is the agent.
        const hints = await screen.findAllByPlaceholderText('A line of your own beside the question')
        userEvent.type(hints[0], 'Агент, который пишет рассылку')

        userEvent.click(screen.getByRole('button', {name: 'Save template'}))
        await waitFor(() => expect(mockedMutator.updateBoard).toHaveBeenCalled())

        const saved = mockedMutator.updateBoard.mock.calls[0][0] as Board
        const setup = JSON.parse(saved.properties[BOARD_PROP_SETUP] as string)
        expect(setup.steps.map((s: {kind: string}) => s.kind)).toEqual(['agent', 'deploy', 'done'])
        expect(setup.steps[0].hint).toBe('Агент, который пишет рассылку')
        await waitFor(() => expect(onClose).toHaveBeenCalled())
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
