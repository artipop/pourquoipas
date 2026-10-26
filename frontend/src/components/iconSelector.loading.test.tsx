import {fireEvent, render, screen} from '@solidjs/testing-library'

import {wrapIntl} from '../testUtils'
import {ensureEmojiData} from '../widgets/emojiData'
import {loadEmojiLocale} from '../widgets/emojiLocale'

import IconSelector from './iconSelector'

vi.mock('../widgets/emojiData', () => ({ensureEmojiData: vi.fn()}))
vi.mock('../widgets/emojiLocale', () => ({loadEmojiLocale: vi.fn()}))
vi.mock('../widgets/emojiPicker', () => ({default: () => <button>{'Ready picker'}</button>}))

describe('emoji readiness', () => {
    test('starts data and translation together and waits for both before drawing the picker', async () => {
        let dataReady!: () => void
        let localeReady!: () => void
        vi.mocked(ensureEmojiData).mockReturnValueOnce(new Promise<void>((resolve) => {
            dataReady = resolve
        }))
        vi.mocked(loadEmojiLocale).mockReturnValueOnce(new Promise<void>((resolve) => {
            localeReady = resolve
        }))
        const {container} = render(() => wrapIntl(() => <IconSelector
            iconElement={<span>{'Icon'}</span>}
            onAddRandomIcon={() => undefined}
            onRemoveIcon={() => undefined}
            onSelectEmoji={() => undefined}
        />))
        expect(ensureEmojiData).not.toHaveBeenCalled()
        fireEvent.click(screen.getByRole('button', {name: 'menuwrapper'}))
        fireEvent.click(container.querySelector('#pick')!)
        expect(ensureEmojiData).toHaveBeenCalledTimes(1)
        expect(loadEmojiLocale).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('status')).toHaveTextContent('Loading…')
        dataReady()
        await Promise.resolve()
        expect(screen.queryByRole('button', {name: 'Ready picker'})).toBeNull()
        localeReady()
        expect(await screen.findByRole('button', {name: 'Ready picker'})).toBeVisible()
        expect(screen.queryByRole('status')).toBeNull()
    })
})
