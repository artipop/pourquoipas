import {createRoot, createSignal, flush} from 'solid-js'
import {waitFor} from '@solidjs/testing-library'

import octoClient from '../octoClient'
import type {FileInfo} from '../blocks/block'

import {createFileLoader} from './fileLoader'

vi.mock('../octoClient', () => ({default: {getFileAsDataUrl: vi.fn()}}))

describe('file loader', () => {
    test('clears an old file and discards a slow answer after the file changes', async () => {
        const answers: Record<string, (info: FileInfo) => void> = {}
        vi.mocked(octoClient.getFileAsDataUrl).mockImplementation((_board, id) => new Promise((resolve) => {
            answers[id] = resolve
        }))
        const [file, setFile] = createSignal('first')
        let loader!: ReturnType<typeof createFileLoader>
        const dispose = createRoot((cleanup) => {
            loader = createFileLoader(() => 'board', file)
            return cleanup
        })
        flush()
        answers.first({url: 'first.png'})
        await waitFor(() => expect(loader[0]().url).toBe('first.png'))

        setFile('slow')
        flush()
        expect(loader[0]().url).toBeUndefined()
        setFile('last')
        flush()
        answers.last({url: 'last.png', archived: true})
        await waitFor(() => expect(loader[0]().url).toBe('last.png'))
        answers.slow({url: 'slow.png'})
        await Promise.resolve()
        flush()
        expect(loader[0]()).toEqual({url: 'last.png', archived: true})
        dispose()
    })

    test('reports a failed request and can retry without remounting', async () => {
        vi.mocked(octoClient.getFileAsDataUrl).mockRejectedValueOnce(new Error('Unavailable')).mockResolvedValueOnce({url: 'retried.png'})
        let loader!: ReturnType<typeof createFileLoader>
        const dispose = createRoot((cleanup) => {
            loader = createFileLoader(() => 'board', () => 'file')
            return cleanup
        })
        flush()
        await waitFor(() => expect(loader[3]()).toBeInstanceOf(Error))
        expect(loader[2]()).toBe(false)
        loader[1]()
        flush()
        expect(loader[3]()).toBeUndefined()
        await waitFor(() => expect(loader[0]().url).toBe('retried.png'))
        dispose()
    })

    test('does not fetch an upload File or a missing file id', async () => {
        vi.mocked(octoClient.getFileAsDataUrl).mockClear()
        const dispose = createRoot((cleanup) => {
            const file = createFileLoader(() => 'board', () => new File(['data'], 'upload.png'))
            expect(file[0]()).toEqual({})
            expect(file[2]()).toBe(false)
            return cleanup
        })
        flush()
        expect(octoClient.getFileAsDataUrl).not.toHaveBeenCalled()
        dispose()
    })
})
