import {Match, Show, Switch, createMemo, lazy} from 'solid-js'

import {Board} from '../../blocks/board'
import {Card} from '../../blocks/card'
import {BoardView} from '../../blocks/boardView'
import octoClient from '../../octoClient'
import {getVisibleAndHiddenGroups, withoutEmptyGroup} from '../../boardUtils'
import {oldestView} from '../../store/views'
import {createLoader} from '../../hooks/loader'
import {useIntl} from '../../intl'

import {AsyncContent, LoadError, LoadingMessage} from '../asyncContent'
import {loadCalendar} from '../calendar/calendarLoader'
import {sortCards} from '../../store/cards'

import ViewTitle from '../viewTitle'
import Kanban from '../kanban/kanban'
import Table from '../table/table'

import Gallery from '../gallery/gallery'

import './boardTemplateSelectorPreview.scss'

type Props = {
    activeTemplate: Board|null
}

const BoardTemplateSelectorPreview = (props: Props) => {
    const intl = useIntl()
    const CalendarFullView = lazy(() => loadCalendar(intl.locale))
    const [preview, reload, loading, error] = createLoader(async (id: string | undefined) => ({
        id,
        blocks: id ? await octoClient.getAllBlocks(id) : [],
    }), undefined as {id: string | undefined, blocks: Awaited<ReturnType<typeof octoClient.getAllBlocks>>} | undefined,
    () => props.activeTemplate?.id)

    // The loader keeps its last answer during a refresh. Only a matching
    // template may draw that answer; switching templates clears the preview.
    const blocks = () => (preview()?.id === props.activeTemplate?.id ? preview()?.blocks || [] : [])
    const activeView = createMemo(() => {
        const views = blocks().filter((block) => block.type === 'view') as BoardView[]
        return views.length ? oldestView(views) : null
    })
    const activeTemplateCards = createMemo(() => {
        const template = props.activeTemplate
        const view = activeView()
        const cards = blocks().filter((block) => block.type === 'card') as Card[]
        return template && view ? sortCards(cards, {}, template, view, {}) : cards
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
                <Show when={error()}>
                    <LoadError onRetry={reload}/>
                </Show>
                <Show when={loading()}>
                    <LoadingMessage/>
                </Show>
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
                        <AsyncContent>
                            <CalendarFullView
                                board={props.activeTemplate!}
                                cards={activeTemplateCards()}
                                activeView={activeView()!}
                                readonly={true}
                                dateDisplayProperty={dateDisplayProperty()}
                                showCard={() => null}
                                addCard={() => Promise.resolve()}
                            />
                        </AsyncContent>
                    </Match>
                </Switch>
            </div>
        </Show>
    )
}

export default BoardTemplateSelectorPreview
