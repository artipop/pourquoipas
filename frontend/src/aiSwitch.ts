import {createSignal} from 'solid-js'

import {UserSettings} from './userSettings'

// The one switch for everything the app does with agents. Off, XCIII is a
// kanban board: no agent settings, no routes, no terminals, no waits — nothing
// on screen that mentions an agent. The Go side reads the same key and stops
// starting stages (aiEnabled in uisettings.go).
//
// Read through to the setting on every call rather than held in a signal of
// its own: this module is imported before main.tsx hydrates the install's
// settings into localStorage, so a value taken at import would be the last
// launch's origin's, which on the desktop is nobody's. The signal is only a
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
