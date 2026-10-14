import {render, screen} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'

import {wrapDNDIntl, onAPhone} from '../testUtils'

import Modal from './modal'

describe('components/modal', () => {
    beforeEach(vi.clearAllMocks)
    test('should match snapshot', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <Modal
                onClose={vi.fn()}
            >
                <div id='test'/>
            </Modal>,
        ))
        expect(container).toMatchSnapshot()
    })
    test('return Modal and close', async () => {
        await onAPhone()
        const onMockedClose = vi.fn()
        render(() => wrapDNDIntl(() =>
            <Modal
                onClose={onMockedClose}
            >
                <div id='test'/>
            </Modal>,
        ))
        const buttonClose = screen.getByRole('button', {name: 'Close'})
        userEvent.click(buttonClose)
        expect(onMockedClose).toHaveBeenCalledTimes(1)
    })
    test('return Modal on position top', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <Modal
                position={'top'}
                onClose={vi.fn()}
            >
                <div id='test'/>
            </Modal>,
        ))
        expect(container).toMatchSnapshot()
    })

    test('return Modal on position bottom', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <Modal
                position={'bottom'}
                onClose={vi.fn()}
            >
                <div id='test'/>
            </Modal>,
        ))
        expect(container).toMatchSnapshot()
    })

    test('return Modal on position bottom-right', () => {
        const {container} = render(() => wrapDNDIntl(() =>
            <Modal
                position={'bottom-right'}
                onClose={vi.fn()}
            >
                <div id='test'/>
            </Modal>,
        ))
        expect(container).toMatchSnapshot()
    })
})
