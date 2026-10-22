import {createSignal} from 'solid-js'

import {UserSettings} from './userSettings'

// The one switch for everything the app does with agents. Off, Pourquoi Pas? is a
// kanban board: no agent settings, no routes, no waits — nothing
// on screen that mentions an agent. The Go side reads the same key and stops
// starting stages (aiEnabled in machinesettings.go).
//
// Read through to the setting on every call rather than held in a signal of
// its own: this module is imported before main.tsx reads the settings, so a value taken at import would be a guess. The signal is only a
// clock that tells tracked readers to ask again.
const [changed, setChanged] = createSignal(0)

export function aiEnabled(): boolean {
    changed()
    return UserSettings.aiEnabled
}

export function setAIEnabled(on: boolean): void {
    UserSettings.aiEnabled = on
    setChanged((n) => n + 1)
}
