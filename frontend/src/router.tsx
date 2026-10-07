import {Component, Loading, createEffect, lazy, onSettled} from 'solid-js'
import {createRouter, useLocation, useNavigate, useParams} from '@solidjs/router'
import type {JSX} from '@solidjs/web'

import ChangePasswordPage from './pages/changePasswordPage'
import ErrorPage from './pages/errorPage'
import LoginPage from './pages/loginPage'
import RegisterPage from './pages/registerPage'
import {Utils} from './utils'
import octoClient from './octoClient'
import {getGlobalError} from './store/globalError'
import {useAppSelector, useAppStore} from './store/hooks'
import fbRoute from './route'

// Auth and auxiliary windows do not need the board's workspace UI.
const BoardPage = lazy(() => import('./pages/boardPage/boardPage'))

const BoardRoute: Component<{readonly?: boolean}> = (props) => (
    <Loading fallback={null}>
        <BoardPage readonly={props.readonly}/>
    </Loading>
)

// A card's changes in a window of their own: its own route, so the window is
// just a URL, and lazily loaded because the board's own window never draws it.
const DiffPage = lazy(() => import('./components/acp/diffPage'))

// The board on a phone: what is waiting for a person, and the cards it is
// about. Lazy for the same reason — a desktop window never opens these, and a
// browser build should not carry them.
const MobilePage = lazy(() => import('./pages/mobile/mobilePage'))

// The dialog the system's the share dialog opens: a link, and the one question of
// which board it goes on. Lazy like the other two — the board itself never
// shows it, and it is opened by the app in a window of its own.
const SharePage = lazy(() => import('./pages/share/sharePage'))

const UUID_REGEX = new RegExp(/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/)

// The pre-teams URL scheme survives in bookmarks: resolve the board, then
// swap /workspace/:workspaceId for /team/:teamId in place.
const workspaceToTeamRedirect = (shared: boolean): Component => () => {
    const params = useParams()
    const location = useLocation()
    const navigate = useNavigate()
    onSettled(() => {
        if (!params.boardId) {
            return
        }
        octoClient.getBoard(params.boardId).then((board) => {
            if (board) {
                const parts = [`/team/${board.teamId}`]
                if (shared) {
                    parts.push('/shared')
                }
                for (const segment of [board.id, params.viewId, params.cardId]) {
                    if (segment) {
                        parts.push(`/${segment}`)
                    }
                }
                navigate(parts.join('') + location.search, {replace: true})
            }
        })
    })
    return null
}

const GlobalErrorRedirect: Component = () => {
    const {actions} = useAppStore()
    const globalError = useAppSelector<string>(getGlobalError)
    const navigate = useNavigate()

    createEffect(globalError, (error) => {
        if (error) {
            actions.globalError.setGlobalError('')
            navigate(`/error?id=${error}`, {replace: true})
        }
    })

    return null
}

// Every route wrapped in one that draws the error redirect above them: Solid
// Router 2 has no `root` prop, and the render-prop child of the instance is
// where a layout goes instead.
const routes = [
    fbRoute({path: '/error', component: ErrorPage}),
    fbRoute({path: '/login', component: LoginPage}),
    fbRoute({path: '/register', component: RegisterPage}),
    fbRoute({path: '/change_password', component: ChangePasswordPage}),

    fbRoute({
        path: '/acp/diff/:cardId',
        component: () => (
            <Loading fallback={null}>
                <DiffPage/>
            </Loading>
        ),
    }),

    fbRoute({
        path: '/m',
        component: () => (
            <Loading fallback={null}>
                <MobilePage/>
            </Loading>
        ),
    }),
    fbRoute({
        path: '/share',
        component: () => (
            <Loading fallback={null}>
                <SharePage/>
            </Loading>
        ),
    }),

    fbRoute({
        path: ['/team/:teamId/shared/:boardId?/:viewId?/:cardId?', '/shared/:boardId?/:viewId?/:cardId?'],
        component: () => <BoardRoute readonly={true}/>,
    }),

    fbRoute({
        loginRequired: true,
        path: '/board/:boardId?/:viewId?/:cardId?',
        getOriginalPath: ({boardId, viewId, cardId}) => `/board/${Utils.buildOriginalPath('', boardId, viewId, cardId)}`,
        component: BoardRoute,
    }),
    fbRoute({
        path: '/workspace/:workspaceId/shared/:boardId?/:viewId?/:cardId?',
        component: workspaceToTeamRedirect(true),
    }),
    fbRoute({
        path: '/workspace/:workspaceId/:boardId?/:viewId?/:cardId?',
        component: workspaceToTeamRedirect(false),
    }),
    fbRoute({
        loginRequired: true,
        path: '/team/:teamId/:boardId?/:viewId?/:cardId?',
        getOriginalPath: ({teamId, boardId, viewId, cardId}) => `/team/${Utils.buildOriginalPath(teamId, boardId, viewId, cardId)}`,
        component: BoardRoute,
    }),

    fbRoute({
        path: '/:boardId?/:viewId?/:cardId?',
        loginRequired: true,
        getOriginalPath: ({boardId, viewId, cardId}) => {
            const boardIdIsValidUUIDV4 = UUID_REGEX.test(boardId || '')
            if (boardIdIsValidUUIDV4) {
                return `/${Utils.buildOriginalPath('', boardId, viewId, cardId)}`
            }
            return ''
        },
        component: BoardRoute,
    }),
]

// The base URL is read once, at module scope, because the router instance is
// built rather than rendered: createRouter takes the config, and the component
// it hands back is what goes in the tree.
const Instance = createRouter({
    routes,
    base: Utils.getBaseURL(),
})

const AppRouter: Component = () => (
    <Instance>
        {(props: {children?: JSX.Element}) => (
            <>
                <GlobalErrorRedirect/>
                {props.children}
            </>
        )}
    </Instance>
)

export default AppRouter
