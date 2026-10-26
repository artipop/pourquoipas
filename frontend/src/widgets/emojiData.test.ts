import {init} from 'emoji-mart'

import {ensureEmojiData} from './emojiData'

vi.mock('emoji-mart', () => ({init: vi.fn()}))
vi.mock('@emoji-mart/data', () => ({default: {}}))

test('shares emoji initialization and retries after a failed attempt', async () => {
    vi.mocked(init).mockRejectedValueOnce(new Error('Unavailable')).mockResolvedValue(undefined)
    const first = ensureEmojiData()
    expect(ensureEmojiData()).toBe(first)
    await expect(first).rejects.toThrow('Unavailable')
    const retry = ensureEmojiData()
    expect(retry).not.toBe(first)
    await retry
    expect(ensureEmojiData()).toBe(retry)
    expect(init).toHaveBeenCalledTimes(2)
})
