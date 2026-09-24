import {createIntl} from './intl'
import {TestBlockFactory} from './test/testBlockFactory'
import {createCheckboxBlock} from './blocks/checkboxBlock'
import {cardAsText} from './cardText'

const intl = createIntl({locale: 'en'})

describe('cardAsText', () => {
    // Dropped into a terminal, this is typed into an agent's prompt: the task
    // has to read as a task, with the field values a person chose rather than
    // the option ids the card stores.
    test('a card reads as its title, its set fields and its description', () => {
        const board = TestBlockFactory.createBoard()
        board.cardProperties = [
            {id: 'status', name: 'Status', type: 'select', options: [{id: 'opt-1', value: 'In progress', color: ''}]},
            {id: 'note', name: 'Note', type: 'text', options: []},
            {id: 'created', name: 'Created', type: 'createdTime', options: []},
        ]
        const card = TestBlockFactory.createCard(board)
        card.title = 'Fix the login'
        card.fields.properties = {status: 'opt-1', note: ''}

        const text = TestBlockFactory.createText(card)
        text.title = 'The form forgets the password.'
        const checkbox = createCheckboxBlock()
        checkbox.title = 'Reproduce it'
        checkbox.fields.value = true

        expect(cardAsText(card, board, [text, [checkbox]], intl)).toBe(
            '# Fix the login\n\nStatus: In progress\n\nThe form forgets the password.\n\n- [x] Reproduce it',
        )
    })

    test('a card with nothing but a title is only its heading', () => {
        const board = TestBlockFactory.createBoard()
        board.cardProperties = []
        const card = TestBlockFactory.createCard(board)
        card.title = ''

        expect(cardAsText(card, board, [], intl)).toBe('# Untitled')
    })
})
