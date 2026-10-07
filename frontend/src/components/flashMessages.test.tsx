import {render, screen} from '@solidjs/testing-library'
import {createMemo, flush, refresh} from 'solid-js'

import '@testing-library/jest-dom'

import userEvent from '@testing-library/user-event'

import {wrapIntl} from '../testUtils'

import {FlashMessages, sendFlashMessage} from './flashMessages'

vi.mock('../mutator')

beforeEach(() => {
    vi.useFakeTimers()
})

afterEach(() => {
    vi.clearAllTimers()
})

describe('components/flashMessages', () => {
    test('mounting does not replay a request sent without a renderer', () => {
        sendFlashMessage({content: 'Before mount', severity: 'normal'})
        flush()

        render(() => wrapIntl(() => <FlashMessages milliseconds={2000}/>))
        expect(screen.queryByText('Before mount')).toBeNull()

        sendFlashMessage({content: 'After mount', severity: 'normal'})
        flush()
        expect(screen.getByText('After mount')).toBeVisible()
    })

    test('sending the same message object again restarts its lifetime', () => {
        render(() => wrapIntl(() => <FlashMessages milliseconds={2000}/>))
        const message = {content: 'Again', severity: 'normal' as const}

        sendFlashMessage(message)
        flush()
        vi.advanceTimersByTime(1000)

        sendFlashMessage(message)
        flush()
        vi.advanceTimersByTime(1100)
        expect(screen.getByText('Again')).toBeVisible()

        vi.advanceTimersByTime(900)
        expect(screen.queryByText('Again')).toBeNull()
    })

    test('a new message interrupts the old fade and starts fully visible', () => {
        render(() => wrapIntl(() => <FlashMessages milliseconds={2000}/>))
        sendFlashMessage({content: 'First', severity: 'normal'})
        flush()
        vi.advanceTimersByTime(1800)
        expect(screen.getByText('First')).toHaveClass('flashOut')

        sendFlashMessage({content: 'Second', severity: 'high'})
        flush()
        expect(screen.queryByText('First')).toBeNull()
        expect(screen.getByText('Second')).toHaveClass('flashIn')

        vi.advanceTimersByTime(200)
        expect(screen.getByText('Second')).toBeVisible()
        vi.advanceTimersByTime(1800)
        expect(screen.queryByText('Second')).toBeNull()
    })

    test('disposing one renderer leaves the other subscribed', () => {
        const first = render(() => wrapIntl(() => <FlashMessages milliseconds={2000}/>))
        render(() => wrapIntl(() => <FlashMessages milliseconds={2000}/>))
        sendFlashMessage({content: 'Together', severity: 'normal'})
        flush()
        expect(screen.getAllByText('Together')).toHaveLength(2)

        first.unmount()
        sendFlashMessage({content: 'Still mounted', severity: 'normal'})
        flush()
        expect(screen.getByText('Still mounted')).toBeVisible()

        vi.advanceTimersByTime(2000)
        expect(screen.queryByText('Still mounted')).toBeNull()
    })

    test('unmounting clears the active timer and stops receiving messages', () => {
        const {unmount} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={2000}/>),
        )

        sendFlashMessage({content: 'First', severity: 'normal'})
        flush()
        expect(vi.getTimerCount()).toBe(1)

        unmount()
        expect(vi.getTimerCount()).toBe(0)

        sendFlashMessage({content: 'After unmount', severity: 'normal'})
        flush()
        expect(vi.getTimerCount()).toBe(0)
    })

    // Something on the page being asked again is the ordinary state of a
    // board, and a message running out meanwhile once halted the whole page:
    // Solid 2 ran the class binding once more with the message already null.
    // The async memo stands in for whatever is being asked again.
    test('a message running out while something else is being asked again does not stop the page', async () => {
        vi.useRealTimers()
        const tick = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))
        const errors = vi.spyOn(console, 'error').mockImplementation(() => undefined)
        let calls = 0
        let asked!: () => number
        render(() => {
            asked = createMemo(async () => {
                calls++
                await tick(calls === 1 ? 1 : 400)
                return calls
            }, {loadingValue: 0})
            return wrapIntl(() => (
                <div>
                    <span>{String(asked())}</span>
                    <FlashMessages milliseconds={250}/>
                </div>
            ))
        })
        await tick(20)

        sendFlashMessage({content: 'First', severity: 'normal'})
        await tick(10)
        expect(screen.getByText('First')).toBeInTheDocument()

        refresh(asked)
        await tick(350)
        expect(screen.queryByText('First')).toBeNull()

        sendFlashMessage({content: 'Second', severity: 'high'})
        await tick(10)
        expect(screen.getByText('Second')).toBeInTheDocument()
        expect(errors.mock.calls.flat().join(' ')).not.toContain('REACTIVITY_HALTED')
        errors.mockRestore()
    })

    test('renders a flash message with high severity', () => {
        const {container} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        /**
         * Check for high severity
         */

        sendFlashMessage({content: 'Mock Content', severity: 'high'})

        flush()

        expect(container).toMatchSnapshot()

        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Mock Content')).toBeNull()
    })

    test('renders a flash message with normal severity', () => {
        const {container} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Mock Content', severity: 'normal'})

        flush()

        expect(screen.getByText('Mock Content')).toHaveClass('normal')

        expect(container).toMatchSnapshot()

        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Mock Content')).toBeNull()
    })

    test('renders a flash message with low severity', () => {
        const {container} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Mock Content', severity: 'low'})

        flush()

        expect(screen.getByText('Mock Content')).toHaveClass('low')

        expect(container).toMatchSnapshot()

        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Mock Content')).toBeNull()
    })

    test('renders a flash message with low severity and custom HTML in flash message', () => {
        const {container} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: <div data-testid='mock-test-id'>{'Mock Content'}</div>, severity: 'low'})

        flush()

        expect(screen.getByTestId('mock-test-id')).toBeVisible()

        expect(container).toMatchSnapshot()

        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Mock Content')).toBeNull()
    })

    // A notice outlives the mount-wide default: the wizard's parting note has a
    // three-menu path in it, and 2 seconds is not enough to read one.
    test('a notice stays up for its own time, not the default', () => {
        render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Come back any time', severity: 'normal', notice: true, milliseconds: 5000})

        flush()

        vi.advanceTimersByTime(1000)
        expect(screen.getByText('Come back any time')).toBeVisible()

        vi.advanceTimersByTime(4000)
        expect(screen.queryByText('Come back any time')).toBeNull()
    })

    // The × is for whoever has already read it — the message must not make
    // them wait the five seconds out.
    test('a notice carries a close button that dismisses it', () => {
        render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Come back any time', severity: 'normal', notice: true, milliseconds: 5000})

        flush()

        userEvent.click(screen.getByRole('button', {name: 'Close'}))
        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Come back any time')).toBeNull()
    })

    // The quick confirmations stay what they were: no close button on them.
    test('a plain flash has no close button', () => {
        render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Copied!', severity: 'normal'})

        flush()

        expect(screen.queryByRole('button', {name: 'Close'})).toBeNull()
    })

    test('renders a flash message with low severity and check onClick on flash works', () => {
        const {container} = render(() =>
            wrapIntl(() => <FlashMessages milliseconds={200}/>),
        )

        sendFlashMessage({content: 'Mock Content', severity: 'low'})

        flush()

        userEvent.click(screen.getByText('Mock Content'))

        expect(container).toMatchSnapshot()

        vi.advanceTimersByTime(200)

        expect(screen.queryByText('Mock Content')).toBeNull()
    })
})
