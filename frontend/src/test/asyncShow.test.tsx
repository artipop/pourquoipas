import {render, screen, waitFor} from '@solidjs/testing-library'
import {Show, createMemo, createSignal, flush, refresh} from 'solid-js'

// rc.8 re-ran a binding in a closing Show while an unrelated async read was
// pending. Both the direct read and Show's narrowed accessor halted the page.
describe('conditional content during async revalidation', () => {
    test.each(['refresh', 'source'])('disposes conditional bindings safely during %s', async (trigger) => {
        let finish!: (value: number) => void
        const [key, setKey] = createSignal(0)
        const [message, setMessage] = createSignal<{text: string} | null>({text: 'First'})
        const [count, setCount] = createSignal(0)
        let calls = 0
        let asked!: () => number
        render(() => {
            asked = createMemo(() => {
                key()
                return ++calls === 1 ? Promise.resolve(1) : new Promise<number>((resolve) => {
                    finish = resolve
                })
            }, {loadingValue: 0})
            return (
                <>
                    <span data-testid='answer'>{asked()}</span>
                    <span data-testid='counter'>{count()}</span>
                    <Show when={message()}>
                        <strong>{message()!.text}</strong>
                    </Show>
                    <Show when={message()}>
                        {(value) => <em>{value().text}</em>}
                    </Show>
                </>
            )
        })
        await waitFor(() => expect(screen.getByTestId('answer')).toHaveTextContent('1'))
        if (trigger === 'refresh') {
            void refresh(asked)
        } else {
            setKey(1)
        }
        flush()
        setMessage(null)
        setCount(1)
        flush()
        expect(screen.queryByText('First')).toBeNull()
        expect(screen.getByTestId('counter')).toHaveTextContent('1')
        finish(2)
        await waitFor(() => expect(screen.getByTestId('answer')).toHaveTextContent('2'))
        setMessage({text: 'Second'})
        setCount(2)
        flush()
        expect(screen.getAllByText('Second')).toHaveLength(2)
        expect(screen.getByTestId('counter')).toHaveTextContent('2')
    })
})
