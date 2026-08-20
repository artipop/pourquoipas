import {createSignal, onSettled} from 'solid-js'
import type {Accessor} from 'solid-js'

export default function useElementAvailable(
    elementIds: string[],
): Accessor<boolean> {
    let checkAvailableInterval: ReturnType<typeof setInterval> | null = null
    const [available, setAvailable] = createSignal(false)

    onSettled(() => {
        checkAvailableInterval = setInterval(() => {
            if (elementIds.every((x) => document.querySelector(x))) {
                setAvailable(true)
                if (checkAvailableInterval) {
                    clearInterval(checkAvailableInterval)
                    checkAvailableInterval = null
                }
            }
        }, 500)
        return () => {
            if (checkAvailableInterval) {
                clearInterval(checkAvailableInterval)
                checkAvailableInterval = null
            }
        }
    })

    return available
}
