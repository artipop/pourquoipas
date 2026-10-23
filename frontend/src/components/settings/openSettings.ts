import {createSignal} from 'solid-js'

// A screen elsewhere asking for a section of the settings: an agent is
// connected in Settings → AI, and a dialog that needs one sends the person
// there rather than growing a second form.
const [requested, setRequested] = createSignal<string | undefined>(undefined)

export const requestedSettings = requested

export function openSettings(section: string): void {
    setRequested(section)
}

export function settingsOpened(): void {
    setRequested(undefined)
}
