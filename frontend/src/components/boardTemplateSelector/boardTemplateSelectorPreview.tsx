import {Loading, Match, Show, Switch, createEffect, createMemo, createSignal, lazy} from 'solid-js'

import {Board} from '../../blocks/board'
import {Card} from '../../blocks/card'
import {BoardView} from '../../blocks/boardView'
import octoClient from '../../octoClient'
import {getVisibleAndHiddenGroups, withoutEmptyGroup} from '../../boardUtils'
import {oldestView} from '../../store/views'
import {sortCards} from '../../store/cards'

import ViewTitle from '../viewTitle'
import Kanban from '../kanban/kanban'
import Table from '../table/table'

import Gallery from '../gallery/gallery'

import './boardTemplateSelectorPreview.scss'

// FullCalendar is half a megabyte of source and the calendar view is the one
// nobody opens first, so it arrives when a calendar is actually drawn.
const CalendarFullView = lazy(() => import('../calendar/fullCalendar'))

type Props = {
    activeTemplate: Board|null
}

const BoardTemplateSelectorPreview = (props: Props) => {
    const [activeView, setActiveView] = createSignal<BoardView|null>(null)
    const [activeTemplateCards, setActiveTemplateCards] = createSignal<Card[]>([])

    createEffect(() => props.activeTemplate, (activeTemplate) => {
        let isSubscribed = true
        if (activeTemplate) {
            setActiveTemplateCards([])
            setActiveView(null)
            octoClient.getAllBlocks(activeTemplate.id).then((blocks) => {
                if (isSubscribed) {
                    const cards = blocks.filter((b) => b.type === 'card')

                    // The view the template was made with, which is the one a
                    // board made from it opens on — not the one whose title
                    // sorts first. Templates used to carry an inbox view that
                    // sorted before their own, and the preview of every one of
                    // them was an empty inbox that said nothing about it.
                    const views = blocks.filter((b) => b.type === 'view') as BoardView[]
                    if (views.length > 0) {
                        setActiveView(oldestView(views))
                    }
                    if (cards.length > 0) {
                        // In the order the board made from it shows them.
                        setActiveTemplateCards(sortCards([...cards] as Card[], {}, activeTemplate, oldestView(views), {}))
                    }
                }
            })
        }
        return () => {
            isSubscribed = false
        }
    })

    const dateDisplayProperty = createMemo(() => {
        return props.activeTemplate?.cardProperties.find((o) => o.id === activeView()?.fields.dateDisplayPropertyId)
    })

    const groupByProperty = createMemo(() => {
        return props.activeTemplate?.cardProperties.find((o) => o.id === activeView()?.fields.groupById) || props.activeTemplate?.cardProperties[0]
    })

    const groups = createMemo(() => {
        const view = activeView()
        if (!view) {
            return {visible: [], hidden: []}
        }
        const all = getVisibleAndHiddenGroups(activeTemplateCards(), view.fields.visibleOptionIds, view.fields.hiddenOptionIds, groupByProperty())
        return {visible: withoutEmptyGroup(all.visible), hidden: []}
    })

    return (
        <Show when={props.activeTemplate}>
            <div class='BoardTemplateSelectorPreview'>
                {/* A picture of the board a person is about to get, and
                    nothing to press: the title and the columns with their
                    cards, without the filter, the totals and the add buttons
                    that the board itself has. */}
                <Show when={activeView()}>
                    <div class='top-head'>
                        <ViewTitle
                            board={props.activeTemplate!}
                            readonly={true}
                        />
                    </div>
                </Show>

                <Switch>
                    <Match when={activeView()?.fields.viewType === 'board'}>
                        <Kanban
                            board={props.activeTemplate!}
                            activeView={activeView()!}
                            cards={activeTemplateCards()}
                            groupByProperty={groupByProperty()}
                            visibleGroups={groups().visible}
                            hiddenGroups={groups().hidden}
                            selectedCardIds={[]}
                            readonly={true}
                            onCardClicked={() => null}
                            addCard={() => Promise.resolve()}
                            addCardFromTemplate={() => Promise.resolve()}
                            showCard={() => null}
                        />
                    </Match>
                    <Match when={activeView()?.fields.viewType === 'table'}>
                        <Table
                            board={props.activeTemplate!}
                            activeView={activeView()!}
                            cards={activeTemplateCards()}
                            groupByProperty={groupByProperty()}
                            views={[activeView()!]}
                            visibleGroups={groups().visible}
                            selectedCardIds={[]}
                            readonly={true}
                            cardIdToFocusOnRender={''}
                            onCardClicked={() => null}
                            addCard={() => Promise.resolve()}
                            showCard={() => null}
                        />
                    </Match>
                    <Match when={activeView()?.fields.viewType === 'gallery'}>
                        <Gallery
                            board={props.activeTemplate!}
                            cards={activeTemplateCards()}
                            activeView={activeView()!}
                            readonly={true}
                            selectedCardIds={[]}
                            onCardClicked={() => null}
                            addCard={() => Promise.resolve()}
                        />
                    </Match>
                    <Match when={activeView()?.fields.viewType === 'calendar'}>
                        <Loading fallback={null}>
                            <CalendarFullView
                                board={props.activeTemplate!}
                                cards={activeTemplateCards()}
                                activeView={activeView()!}
                                readonly={true}
                                dateDisplayProperty={dateDisplayProperty()}
                                showCard={() => null}
                                addCard={() => Promise.resolve()}
                            />
                        </Loading>
                    </Match>
                </Switch>
            </div>
        </Show>
    )
}

export default BoardTemplateSelectorPreview
