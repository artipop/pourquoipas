import {createIntl} from './intl'
import {getMessages} from './i18n'
import {goWords, reportWords} from './goWords'
import {fakeGoSide, goSide} from './test/goSide'

const intlIn = (lang: string) => createIntl({locale: lang, defaultLocale: 'en', messages: getMessages(lang)})

describe('goWords', () => {
    afterEach(() => {
        fakeGoSide()
    })

    // The menu bar and the notifications are drawn by Go, and a person reading
    // the app in Russian has to read them in Russian too.
    test('are the catalogue\'s words in the page\'s language', () => {
        const words = goWords(intlIn('ru'))
        expect(words.quit).toBe('Выход')
        expect(words.agentAsking).toBe('{agent} спрашивает')
        expect(words.diffWindow).toBe('Изменения · {title}')
        expect(goWords(intlIn('en')).quit).toBe('Quit')
    })

    // Go has no plural rule of its own, so the page sends the category of each
    // count along with one sentence per category.
    test('carry the plural as forms and the category of each count', () => {
        const {moreWaits} = goWords(intlIn('ru'))
        expect(moreWaits.counts[1]).toBe('one')
        expect(moreWaits.counts[3]).toBe('few')
        expect(moreWaits.counts[5]).toBe('many')
        expect(moreWaits.counts[21]).toBe('one')
        expect(moreWaits.forms.many).toBe('…и ещё {count}')
        expect(moreWaits.forms.other).toBeTruthy()
    })

    test('are handed to Go as JSON', async () => {
        fakeGoSide({UseWords: vi.fn().mockResolvedValue(undefined)})
        await reportWords('ru')
        const sent = JSON.parse(vi.mocked(goSide.UseWords).mock.calls[0][0] as string)
        expect(sent.open).toBe('Открыть')
    })
})
