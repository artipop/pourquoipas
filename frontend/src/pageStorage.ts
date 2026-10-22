import type {IAppWindow} from './types'

type PageStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

// A desktop window keeps its data out of the browser: wails:// storage outlives
// the app's database, and pointed a fresh install at boards it does not have.
// What outlives a launch is the install's (ui-settings.json); the session is
// handed over by the bootstrap on each load.
let memory: Map<string, string> | undefined

const inMemory: PageStorage = {
    getItem: (key) => memoryStore().get(key) ?? null,
    setItem: (key, value) => {
        memoryStore().set(key, String(value))
    },
    removeItem: (key) => {
        memoryStore().delete(key)
    },
}

function memoryStore(): Map<string, string> {
    if (!memory) {
        const seeded = (window as unknown as IAppWindow).seededSession
        memory = new Map(seeded ? [['boardSessionId', seeded]] : [])
    }
    return memory
}

export function pageStorage(): PageStorage {
    return (window as unknown as IAppWindow).pageOnWails ? inMemory : localStorage
}
