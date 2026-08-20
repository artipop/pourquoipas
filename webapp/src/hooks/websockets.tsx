import {createEffect} from 'solid-js'

import wsClient, {WSClient} from '../wsclient'

// teamId is an accessor now; the deps list is gone — the effect re-runs when
// whatever fn reads changes, which is what the list approximated.
export const useWebsockets = (teamId: () => string, fn: (wsClient: WSClient) => () => void): void => {
    createEffect(teamId, (team) => {
        if (!team) {
            return undefined
        }

        wsClient.subscribeToTeam(team)
        const teardown = fn(wsClient)

        return () => {
            teardown()
            wsClient.unsubscribeToTeam(team)
        }
    })
}
