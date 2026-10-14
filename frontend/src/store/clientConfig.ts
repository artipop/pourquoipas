import {ClientConfig} from '../config/clientConfig'

import {ShowUsername} from '../utils'

import type {StoreContext} from './context'

import type {RootState} from './index'

export type ClientConfigState = {value: ClientConfig}

const defaultClientConfig = (): ClientConfig => ({enablePublicSharedBoards: false, teammateNameDisplay: ShowUsername, featureFlags: {}, maxFileSize: 0, teamMode: false, registrationOpen: false})

export const initialClientConfigState = (): ClientConfigState => ({value: defaultClientConfig()})

export const createClientConfigActions = ({setState, deps}: StoreContext) => ({
    setClientConfig(config: ClientConfig) {
        setState((draft) => {
            draft.clientConfig.value = config
        })
    },
    async fetchClientConfig(): Promise<void> {
        const config = await deps.client.getClientConfig()
        setState((draft) => {
            draft.clientConfig.value = config || defaultClientConfig()
        })
    },
})

export function getClientConfig(state: RootState): ClientConfig {
    return state.clientConfig.value
}

// Whether there is a second person to say something to. What hangs off it is
// the comments — a conversation between people, switched off while there is one
// (docs/teamwork.md) — so it is asked as its own question rather than by
// reading a config field at every call site.
export function getTeamMode(state: RootState): boolean {
    return Boolean(state.clientConfig?.value?.teamMode)
}

// Whether the way in is registration rather than a login. False everywhere
// except a hosted server nobody has claimed, so a page that reads it before the
// config arrives sends nobody anywhere by accident.
export function getRegistrationOpen(state: RootState): boolean {
    return Boolean(state.clientConfig?.value?.registrationOpen)
}
