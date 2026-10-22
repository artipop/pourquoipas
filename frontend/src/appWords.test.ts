import {afterEach, describe, expect, test} from 'vitest'

import {createBoard} from './blocks/board'
import {createBlock} from './blocks/block'
import {fakeGoSide} from './test/goSide'
import {loadAppWords, resetAppWords, translateBlocks, translateBoard} from './appWords'

const ru = {
    fields: ['title', 'name', 'value'],
    words: {Development: 'Разработка', Status: 'Статус', 'In progress': 'В работе', Inbox: 'Входящие', Author: 'Автор', Draft: 'Черновик'},
}

const template = () => createBoard({
    id: 'tpl',
    isTemplate: true,
    createdBy: 'system',
    title: 'Development',
    cardProperties: [{id: 'status', name: 'Status', type: 'select', options: [{id: 'o1', value: 'In progress', color: ''}]}],
})

const inbox = () => createBoard({
    id: 'inbox',
    title: 'Inbox',
    properties: {inbox: true} as unknown as Record<string, string>,
    cardProperties: [{id: 'a', name: 'Author', type: 'createdBy', options: []}],
})

describe('appWords', () => {
    afterEach(() => {
        resetAppWords()
        fakeGoSide()
    })

    // The install's own templates are stored in English and read in each
    // person's language; on a server two people see one board in two.
    test('a template the install shipped is shown in the language of the page', async () => {
        fakeGoSide({TemplateWords: async () => ru})
        await loadAppWords('ru')

        const board = translateBoard(template())
        expect(board.title).toBe('Разработка')
        expect(board.cardProperties[0].name).toBe('Статус')
        expect(board.cardProperties[0].options[0].value).toBe('В работе')
        expect(board.cardProperties[0].options[0].id).toBe('o1')

        const [card] = translateBlocks([createBlock({boardId: 'tpl', type: 'card', title: 'Draft'})])
        expect(card.title).toBe('Черновик')
    })

    // A board made from a template, or a person's own template, is their data:
    // it was translated once, as it was copied, and is never touched again.
    test('a board a person made is shown as it is', async () => {
        fakeGoSide({TemplateWords: async () => ru})
        await loadAppWords('ru')

        const mine = createBoard({...template(), id: 'mine', isTemplate: false, createdBy: 'me'})
        expect(translateBoard(mine).title).toBe('Development')
        const [card] = translateBlocks([createBlock({boardId: 'mine', type: 'card', title: 'Draft'})])
        expect(card.title).toBe('Draft')
    })

    // The inbox is the app's board but a person's cards: only the names the
    // app gave it are translated.
    test('the inbox has its own names translated and nothing that was written on it', async () => {
        fakeGoSide({TemplateWords: async () => ru})
        await loadAppWords('ru')

        const board = translateBoard(inbox())
        expect(board.title).toBe('Входящие')
        expect(board.cardProperties[0].name).toBe('Автор')

        const [view, card] = translateBlocks([
            createBlock({boardId: 'inbox', type: 'view', title: 'Inbox'}),
            createBlock({boardId: 'inbox', type: 'card', title: 'Draft'}),
        ])
        expect(view.title).toBe('Входящие')
        expect(card.title).toBe('Draft')
    })

    // A language the templates are not translated into, or no Go side at all,
    // shows the English the boards are stored in.
    test('with no words the boards are shown in English', async () => {
        await loadAppWords('kk')
        expect(translateBoard(template()).title).toBe('Development')
    })

    // The page reads the app's boards again on a switch, and only then.
    test('loading says whether the language changed from the one boards were read in', async () => {
        fakeGoSide({TemplateWords: async () => ru})
        expect(await loadAppWords('ru')).toBe(false)
        expect(await loadAppWords('ru')).toBe(false)
        expect(await loadAppWords('en')).toBe(true)
    })

    // On a server the bindings answer only after the login, and the page asks
    // for the words both before it and after it: the first failure must not
    // stand for the language.
    // The board page's first language can be the store's default before the
    // person's is read; the answer to the later question is the one that stays.
    test('an answer overtaken by a later language is dropped', async () => {
        let release: (v: typeof ru) => void = () => {}
        const english = new Promise<typeof ru>((resolve) => {
            release = resolve
        })
        fakeGoSide({TemplateWords: (lang: string) => (lang === 'en' ? english : Promise.resolve(ru))})
        const first = loadAppWords('en')
        await loadAppWords('ru')
        release({fields: ru.fields, words: {}})
        await first
        expect(translateBoard(template()).title).toBe('Разработка')
    })

    test('a load that found no Go side is asked again', async () => {
        await loadAppWords('ru')
        fakeGoSide({TemplateWords: async () => ru})
        await loadAppWords('ru')
        expect(translateBoard(template()).title).toBe('Разработка')
    })
})
