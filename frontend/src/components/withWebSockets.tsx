import {ParentComponent, createEffect, onSettled} from 'solid-js'

import {sessionToken} from '../session'
import wsClient from '../wsclient'

type Props = {
    userId?: string
}

// WithWebSockets component initialises the websocket connection if
// it's not yet running and subscribes to the current team
const WithWebSockets: ParentComponent<Props> = (props) => {
    const queryString = new URLSearchParams(window.location.search)

    onSettled(() => {
        // if the websocket client was already connected, do nothing
        if (wsClient.state !== 'init') {
            return
        }

        const token = sessionToken() || queryString.get('r') || ''
        if (token) {
            wsClient.authenticate(token)
        }
        wsClient.open()
    })

    createEffect(() => props.userId, (userId) => {
        if (!userId) {
            return
        }

        const token = sessionToken() || queryString.get('r') || ''
        if (wsClient.token !== token) {
            wsClient.authenticate(token)
        }
    })

    return <>{props.children}</>
}

export default WithWebSockets
