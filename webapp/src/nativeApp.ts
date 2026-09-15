import {IAppWindow} from './types'
import {exportUserSettingsBlob, importUserSettingsBlob} from './userSettings'

declare interface INativeApp {
    settingsBlob: string | null
}

declare const NativeApp: INativeApp
declare let window: IAppWindow

export function importNativeAppSettings(): void {
    if (typeof NativeApp === 'undefined' || !NativeApp.settingsBlob) {
        return
    }
    const importedKeys = importUserSettingsBlob(NativeApp.settingsBlob)
    const messageType = importedKeys.length ? 'didImportUserSettings' : 'didNotImportUserSettings'
    postWebKitMessage({type: messageType, settingsBlob: exportUserSettingsBlob(), keys: importedKeys})
    NativeApp.settingsBlob = null
}

// A settings write happens in the middle of whatever a person is doing —
// typing a theme's name, ticking a switch — and this runs inside it, before it
// is even known whether a native host is listening. So nothing here may take
// that down: the blob has thrown once already, `btoa` over a name written in
// Russian, and what it broke was the field it was called from rather than the
// host it was for.
export function notifySettingsChanged(key: string): void {
    try {
        postWebKitMessage({type: 'didChangeUserSettings', settingsBlob: exportUserSettingsBlob(), key})
    } catch (e) {
        // A host that cannot be told goes without.
    }
}

function postWebKitMessage<T>(message: T) {
    window.webkit?.messageHandlers.nativeApp?.postMessage(message)
}
