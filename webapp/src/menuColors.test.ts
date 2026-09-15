import ru from '../i18n/ru.json'
import en from '../i18n/en.json'

import {createIntl} from './intl'

import {Constants} from './constants'
import {menuColorLabel} from './menuColors'

describe('menuColors', () => {
    // A colour the constant offers and the dictionary does not know would go
    // back on screen as the constant's English value — the bug this fixes.
    test('every colour the menus offer is translated in both catalogues', () => {
        const intl = createIntl({locale: 'ru', messages: ru as Record<string, string>})
        for (const key of Object.keys(Constants.menuColors)) {
            const label = menuColorLabel(intl, key)
            expect(label).not.toBe(Constants.menuColors[key])
            expect(label.length).toBeGreaterThan(0)

            // The English catalogue carries the same ids, so the language
            // switcher never lands on a missing name.
            const id = `MenuColors.${key.replace('propColor', '').toLowerCase()}`
            expect((en as Record<string, string>)[id]).toBeTruthy()
        }
    })
})
