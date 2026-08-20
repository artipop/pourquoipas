import {Component, Show, createEffect} from 'solid-js'
import {useNavigate, useParams} from '@solidjs/router'
import type {Params, RouteDefinition} from '@solidjs/router'

import {Utils} from './utils'
import {getLoggedIn} from './store/users'
import {useAppSelector} from './store/hooks'

type RouteProps = {
    path: string|string[]
    component: Component
    getOriginalPath?: (params: Params) => string
    loginRequired?: boolean
}

// Solid Router 2 has no <Navigate>: a redirect is a navigate call, and it runs
// in the effect half so it is a side effect rather than something a render
// does.
const Redirect: Component<{href: string}> = (props) => {
    const navigate = useNavigate()
    createEffect(() => props.href, (href) => navigate(href, {replace: true}))
    return null
}

// The login guard react-router carried as a Route wrapper: render the page
// unless the session is known to be absent, then bounce to /error with the
// original path in `r` so login can come back.
//
// A route definition rather than a component now — Solid Router 2 takes the
// route tree as data, not as JSX children — so this returns what createRouter
// wants and the guard stays where it was.
function fbRoute(props: RouteProps): RouteDefinition {
    const guarded: Component = () => {
        const loggedIn = useAppSelector(getLoggedIn)
        const params = useParams()

        const loginUrl = () => {
            if (props.getOriginalPath) {
                let redirectUrl = '/' + Utils.buildURL(props.getOriginalPath(params))
                if (redirectUrl.indexOf('//') === 0) {
                    redirectUrl = redirectUrl.slice(1)
                }
                return `/error?id=not-logged-in&r=${encodeURIComponent(redirectUrl)}`
            }
            return '/error?id=not-logged-in'
        }

        return (
            <Show
                when={!(props.loginRequired && loggedIn() === false)}
                fallback={<Redirect href={loginUrl()}/>}
            >
                {props.component({})}
            </Show>
        )
    }

    return {
        path: props.path,
        component: props.loginRequired ? guarded : props.component,
    }
}

export {Redirect}

export default fbRoute
