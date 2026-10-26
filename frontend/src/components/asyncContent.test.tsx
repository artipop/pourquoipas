import {fireEvent, render, screen} from '@solidjs/testing-library'
import {lazy} from 'solid-js'

import {wrapIntl} from '../testUtils'

import {AsyncContent} from './asyncContent'

describe('async content', () => {
    test('keeps the shell visible while a chunk is loading, and retries a failed load', async () => {
        let reject!: (error: Error) => void
        let calls = 0
        const Page = lazy(() => {
            calls++
            if (calls === 1) {
                return new Promise<never>((_resolve, fail) => {
                    reject = fail
                })
            }
            return Promise.resolve({default: () => <div>{'Loaded page'}</div>})
        })
        render(() => wrapIntl(() => (
            <>
                <div>{'Navigation'}</div>
                <AsyncContent><Page/></AsyncContent>
            </>
        )))
        expect(screen.getByText('Navigation')).toBeVisible()
        expect(screen.getByRole('status')).toHaveTextContent('Loading…')
        reject(new Error('Chunk unavailable'))
        expect(await screen.findByRole('alert')).toHaveTextContent('Could not load this content.')
        fireEvent.click(screen.getByRole('button', {name: 'Try again'}))
        expect(await screen.findByText('Loaded page')).toBeVisible()
        expect(calls).toBe(2)
        expect(screen.queryByRole('alert')).toBeNull()
        expect(screen.getByText('Navigation')).toBeVisible()
    })

    test('keeps a supplied preview visible until the editor chunk arrives', async () => {
        let resolve!: (value: {default: () => string}) => void
        const Editor = lazy(() => new Promise<{default: () => string}>((done) => {
            resolve = done
        }))
        render(() => wrapIntl(() => (
            <AsyncContent fallback={<p>{'Existing text'}</p>}><Editor/></AsyncContent>
        )))
        expect(screen.getByText('Existing text')).toBeVisible()
        resolve({default: () => 'Editable text'})
        expect(await screen.findByText('Editable text')).toBeVisible()
        expect(screen.queryByText('Existing text')).toBeNull()
    })
})
