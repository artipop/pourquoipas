import {createIntl} from './intl'
<<<<<<< Updated upstream
import {getMessages} from './i18n'
=======
import {getMessages} from './i18n'
>>>>>>> Stashed changes
import {goError, goMessage, goSentences} from './goMessages'

const intlIn = (lang: string) => createIntl({locale: lang, defaultLocale: 'en', messages: getMessages(lang)})

describe('goMessages', () => {
<<<<<<< Updated upstream
=======
>>>>>>> Stashed changes
    // Go says a code and its values; the sentence is ours, in the reader's
    // language.
    test('word a code in the page\'s language', () => {
        const said = {code: 'folders.dirty', args: {dir: '/work/web'}, text: 'The folder /work/web has unsaved changes'}
        expect(goMessage(intlIn('ru'), said)).toBe('В папке /work/web есть несохранённые изменения')
        expect(goMessage(intlIn('en'), said)).toBe('The folder /work/web has unsaved changes')
    })

    // A Go side newer than this page says codes the page has not heard of,
    // and a plugin's own words come with no code at all.
    test('fall back on the English Go sent', () => {
        expect(goMessage(intlIn('ru'), {code: 'no.such-code', text: 'Something new'})).toBe('Something new')
        expect(goMessage(intlIn('ru'), {text: 'the plugin said so'})).toBe('the plugin said so')
        expect(goMessage(intlIn('ru'), undefined)).toBe('')
    })

    // A binding that fails carries the code as the error's cause
    // (apperr.MarshalError).
    test('read a failed call\'s cause', () => {
        const e = Object.assign(new Error('Enter a username'), {cause: {code: 'attach.no-username', message: 'Enter a username'}})
        expect(goError(intlIn('ru'), e)).toBe('Укажите имя пользователя')
        expect(goError(intlIn('ru'), new Error('disk full'))).toBe('disk full')
    })

    // The permission question reaches the OS notification without the page, so
    // Go gets the sentence with its slots left in.
    test('hand Go the sentences it words itself, slots and all', () => {
        expect(goSentences(intlIn('ru'))['question.allow-tool-title']).toBe('Разрешить {tool}: {title}?')
    })
})
