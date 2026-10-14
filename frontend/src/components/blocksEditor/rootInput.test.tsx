import {render, screen, fireEvent} from '@solidjs/testing-library'

import {wrapIntl} from '../../testUtils'

import RootInput from './rootInput'

describe('components/blocksEditor/rootInput', () => {
    test('shows the current value in the combobox', () => {
        render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value='test-value'
                onChangeType={vi.fn()}
                onSave={vi.fn()}
            />,
        ))
        expect(screen.getByRole('combobox')).toHaveValue('test-value')
        expect(screen.queryByText('Type your text, or a slash command')).toBeNull()
        expect(screen.queryByRole('listbox')).toBeNull()
    })

    test('shows slash commands when the menu opens', async () => {
        render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value=''
                onChangeType={vi.fn()}
                onSave={vi.fn()}
            />,
        ))
        const input = screen.getByDisplayValue('')
        fireEvent.input(input, {target: {value: '/'}})
        expect(await screen.findByRole('listbox')).toBeInTheDocument()
        expect(screen.getByText('/title Creates a new Title block.')).toBeInTheDocument()
        expect(screen.getByText('/checkbox Creates a new Checkbox block.')).toBeInTheDocument()
    })

    test('links the combobox to its menu and active option', async () => {
        const {container} = render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value=''
                onChangeType={vi.fn()}
                onSave={vi.fn()}
            />,
        ))
        const input = screen.getByRole('combobox')
        fireEvent.input(input, {target: {value: '/'}})

        const listbox = await screen.findByRole('listbox')
        expect(input.getAttribute('aria-controls')).toBe(listbox.id)
        const activeId = input.getAttribute('aria-activedescendant')
        expect(activeId).toBeTruthy()
        expect(container.querySelector(`[id="${activeId}"]`)?.getAttribute('role')).toBe('option')
    })

    test('should emit onChange event', async () => {
        const onChange = vi.fn()
        render(() => wrapIntl(() =>
            <RootInput
                onChange={onChange}
                value='test-value'
                onChangeType={vi.fn()}
                onSave={vi.fn()}
            />,
        ))

        expect(onChange).not.toHaveBeenCalled()

        const input = screen.getByDisplayValue('test-value')
        fireEvent.input(input, {target: {value: 'test-value-'}})
        expect(onChange).toHaveBeenCalled()
    })

    test('should not emit onChangeType event when value is not empty and hit backspace', async () => {
        const onChangeType = vi.fn()
        render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value='test-value'
                onChangeType={onChangeType}
                onSave={vi.fn()}
            />,
        ))

        expect(onChangeType).not.toHaveBeenCalled()
        const input = screen.getByDisplayValue('test-value')
        fireEvent.keyDown(input, {key: 'Backspace'})
        expect(onChangeType).not.toHaveBeenCalled()
    })

    test('should emit onSave event hit enter', async () => {
        const onSave = vi.fn()
        render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value='test-value'
                onChangeType={vi.fn()}
                onSave={onSave}
            />,
        ))

        expect(onSave).not.toHaveBeenCalled()
        const input = screen.getByDisplayValue('test-value')
        fireEvent.keyDown(input, {key: 'Enter'})
        expect(onSave).toHaveBeenCalled()
    })

    test('should emit onChangeType event on menu option selected', async () => {
        const onChangeType = vi.fn()
        render(() => wrapIntl(() =>
            <RootInput
                onChange={vi.fn()}
                value=''
                onChangeType={onChangeType}
                onSave={vi.fn()}
            />,
        ))

        const input = screen.getByDisplayValue('')
        fireEvent.input(input, {target: {value: '/'}})

        const option = screen.getByText('/title Creates a new Title block.')
        fireEvent.click(option)

        expect(onChangeType).toHaveBeenCalledWith(expect.objectContaining({name: 'h1'}))
    })
})
