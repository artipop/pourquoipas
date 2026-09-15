import {reconcile, storePath} from 'solid-js'

import {Subscription} from '../wsclient'
import {ErrorId} from '../errors'
import {forgetSession} from '../session'
import {Board} from '../blocks/board'
import {parseUserProps} from '../user'

import {viewsFromBlocks} from './views'
import {cardsFromBlocks} from './cards'
import {contentsFromBlocks} from './contents'
import {commentsFromBlocks} from './comments'
import {attachmentsFromBlocks} from './attachments'

import type {StoreContext} from './context'

import type {RootState} from './index'

// The loaders that used to be thunks with a fan-out of extraReducers: one
// fetch, many domains. Each applies its whole result in a single batch so the
// UI never sees a half-loaded board.
export const createInitialLoadActions = (ctx: StoreContext) => {
    const {setState, deps} = ctx

    // Full board-content rebuild shared by initialReadOnlyLoad and
    // loadBoardData. reconcile keeps identity of unchanged blocks, so a reload
    // does not redraw every card.
    const applyBlocks = (blocks: Parameters<typeof viewsFromBlocks>[0]) => {
        const {cards, templates} = cardsFromBlocks(blocks)
        setState((draft) => {
            reconcile(viewsFromBlocks(blocks))(draft.views.views)
        })
        setState((draft) => {
            reconcile(cards)(draft.cards.cards)
        })
        setState((draft) => {
            reconcile(templates)(draft.cards.templates)
        })
        setState((draft) => {
            reconcile(contentsFromBlocks(blocks))(draft.contents)
        })
        setState((draft) => {
            reconcile(commentsFromBlocks(blocks))(draft.comments)
        })
        setState((draft) => {
            reconcile(attachmentsFromBlocks(blocks))(draft.attachments)
        })
    }

    return {
        async initialLoad() {
            try {
                const [me, myConfig, team, teams, boards, boardsMemberships, boardTemplates] = await Promise.all([
                    deps.client.getMe(),
                    deps.client.getMyConfig(),
                    deps.client.getTeam(),
                    deps.client.getTeams(),
                    deps.client.getBoards(),
                    deps.client.getMyBoardMemberships(),
                    deps.client.getTeamTemplates(),
                ])

                // if no me, normally user not logged in
                if (!me) {
                    // And the session that was being offered is gone, so it is
                    // thrown away rather than kept and retried. The board's
                    // socket reopens every three seconds and re-runs this load
                    // each time — with a token the server no longer knows, that
                    // is a loop that never ends and never logs in. Forgetting it
                    // is what turns the next attempt into the login page.
                    forgetSession()
                    throw new Error(ErrorId.NotLoggedIn)
                }

                // if no team, either bad id, or user doesn't have access
                if (!team) {
                    throw new Error(ErrorId.TeamUndefined)
                }

                setState(storePath('teams', 'current', team))
                setState(storePath('teams', 'allTeams', [...teams].sort((a, b) => (a.title < b.title ? -1 : 1))))

                setState(storePath('boards', 'boards', boards.reduce((acc: {[key: string]: Board}, b: Board) => {
                    acc[b.id] = b
                    return acc
                }, {})))
                setState(storePath('boards', 'templates', boardTemplates.reduce((acc: {[key: string]: Board}, b: Board) => {
                    acc[b.id] = b
                    return acc
                }, {})))
                setState(storePath('boards', 'myBoardMemberships', boardsMemberships.reduce((acc: {[key: string]: typeof boardsMemberships[0]}, m) => {
                    acc[m.boardId] = m
                    return acc
                }, {})))

                if (myConfig) {
                    setState(storePath('users', 'myConfig', parseUserProps(myConfig)))
                }

                return {team, teams, boards, boardsMemberships, boardTemplates, myConfig}
            } catch (e) {
                setState(storePath('globalError', 'value', (e as Error).message || ''))
                throw e
            }
        },

        async initialReadOnlyLoad(boardId: string) {
            try {
                const [board, blocks] = await Promise.all([
                    deps.client.getBoard(boardId),
                    deps.client.getAllBlocks(boardId),
                ])

                // if no board, read_token invalid
                if (!board) {
                    throw new Error(ErrorId.InvalidReadOnlyBoard)
                }

                const boards: {[key: string]: Board} = {}
                const templates: {[key: string]: Board} = {}
                if (board.isTemplate) {
                    templates[board.id] = board
                } else {
                    boards[board.id] = board
                }
                setState((draft) => {
                    reconcile(boards)(draft.boards.boards)
                })
                setState((draft) => {
                    reconcile(templates)(draft.boards.templates)
                })
                applyBlocks(blocks)

                return {board, blocks}
            } catch (e) {
                setState(storePath('globalError', 'value', (e as Error).message || ''))
                throw e
            }
        },

        // A fetch and an apply rather than one action for the board page, which
        // asks through an async memo so that the answer for a board somebody has
        // already left is never applied over the one they opened.
        fetchBoardBlocks(boardID: string) {
            return deps.client.getAllBlocks(boardID)
        },

        applyBoardBlocks(blocks: Parameters<typeof viewsFromBlocks>[0]) {
            applyBlocks(blocks)
        },

        async loadBoardData(boardID: string) {
            const blocks = await deps.client.getAllBlocks(boardID)
            applyBlocks(blocks)
            return {blocks}
        },

        async loadBoards() {
            const boards = await deps.client.getBoards()
            setState((draft) => {
                reconcile(boards.reduce((acc: {[key: string]: Board}, b: Board) => {
                    acc[b.id] = b
                    return acc
                }, {}))(draft.boards.boards)
            })
            return {boards}
        },

        async loadMyBoardsMemberships() {
            const boardsMemberships = await deps.client.getMyBoardMemberships()
            setState((draft) => {
                reconcile(boardsMemberships.reduce((acc: {[key: string]: typeof boardsMemberships[0]}, m) => {
                    acc[m.boardId] = m
                    return acc
                }, {}))(draft.boards.myBoardMemberships)
            })
            return {boardsMemberships}
        },
    }
}

export const getUserBlockSubscriptions = (state: RootState): Subscription[] => state.users.blockSubscriptions

export const getUserBlockSubscriptionList = (state: RootState): Subscription[] => getUserBlockSubscriptions(state)
