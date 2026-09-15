import {For, Show} from 'solid-js'

import {FormattedMessage, useIntl} from '../../intl'

import {BoardView} from '../../blocks/boardView'
import mutator from '../../mutator'
import Button from '../../widgets/buttons/button'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import CheckIcon from '../../widgets/icons/check'
import {Flow} from '../acp/automation'

type Props = {
    activeView: BoardView
    flows: Flow[]
}

// Which route the board is drawn by. A board carries several — not every task
// is deployed — and drawing all of them at once is what makes a kanban of eight
// columns out of a route of three. The picker is a view control beside "group
// by" because that is what it is: the same cards, narrowed.
//
// Offered wherever the board has a route at all. Not only where there are two
// to choose between: narrowing a board of nine columns to the three its one
// route goes through is the same thing this is for.
const ViewHeaderFlowMenu = (props: Props) => {
    const intl = useIntl()

    const allRoutes = () => intl.formatMessage({id: 'ViewHeader.all-routes', defaultMessage: 'All routes'})
    const selected = () => props.flows.find((f) => f.id === props.activeView.fields.flowId)

    const pick = (id: string) => {
        if ((props.activeView.fields.flowId || '') === id) {
            return
        }
        mutator.changeViewFlowId(props.activeView.boardId, props.activeView.id, props.activeView.fields.flowId, id)
    }

    return (
        <Show when={props.flows.length > 0}>
            <MenuWrapper
                menu={
                    <Menu>
                        <Menu.Text
                            id={''}
                            name={allRoutes()}
                            rightIcon={selected() ? undefined : <CheckIcon/>}
                            onClick={pick}
                        />
                        <Menu.Separator/>
                        <For each={props.flows}>
                            {(flow) => (
                                <Menu.Text
                                    id={flow.id || ''}
                                    name={flow.name}
                                    rightIcon={selected()?.id === flow.id ? <CheckIcon/> : undefined}
                                    onClick={pick}
                                />
                            )}
                        </For>
                    </Menu>
                }
            >
                <Button>
                    <FormattedMessage
                        id='ViewHeader.route'
                        defaultMessage='Route: {route}'
                        values={{
                            route: (
                                <span
                                    style={{color: 'rgb(var(--center-channel-color-rgb))'}}
                                    id='routeLabel'
                                >
                                    {selected()?.name || allRoutes()}
                                </span>
                            ),
                        }}
                    />
                </Button>
            </MenuWrapper>
        </Show>
    )
}

export default ViewHeaderFlowMenu
