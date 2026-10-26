import {fireEvent, render, screen} from '@solidjs/testing-library'

import type {ComboboxItem} from '../combobox'

import Combobox from './combobox'

describe('async combobox', () => {
    test('keeps focus and accepts only the latest query, with an empty list on failure', async () => {
        const pending: Record<string, {resolve: (items: Array<ComboboxItem<string>>) => void, reject: (error: Error) => void}> = {}
        const load = (text: string) => new Promise<Array<ComboboxItem<string>>>((resolve, reject) => {
            pending[text] = {resolve, reject}
        })
        render(() => <Combobox
            classNamePrefix='Test'
            ariaLabel='Search'
            loadOptions={load}
            onChange={() => undefined}
            menuIsOpen={true}
            noOptionsMessage='No results'
        />)
        const input = screen.getByRole('combobox') as HTMLInputElement
        input.focus()
        fireEvent.input(input, {target: {value: 'slow'}})
        fireEvent.input(input, {target: {value: 'latest'}})
        pending.latest.resolve([{id: 'latest', label: 'Latest result', data: 'latest'}])
        expect(await screen.findByRole('option', {name: 'Latest result'})).toBeVisible()
        pending.slow.resolve([{id: 'slow', label: 'Old result', data: 'slow'}])
        await Promise.resolve()
        expect(screen.queryByRole('option', {name: 'Old result'})).toBeNull()
        expect(input).toHaveFocus()
        fireEvent.input(input, {target: {value: 'failed'}})
        pending.failed.reject(new Error('Unavailable'))
        expect(await screen.findByText('No results')).toBeVisible()
        expect(screen.queryByRole('option')).toBeNull()
    })
})
