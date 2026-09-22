import {Board} from '../../blocks/board'

import './sidebarBoardItem.scss'

type Props = {
    board: Board
    isActive: boolean
    onClick: (boardId: string) => void
}

// The inbox, pinned above every category. It is the one place everything
// arrives, so it is where the list starts whatever a person has done to their
// categories — and it is not dragged, moved or deleted from here, which is why
// it is a row of its own rather than a SidebarBoardItem: that one is a sortable
// member of a category and carries the board menu.
const SidebarInboxItem = (props: Props) => (
    <div
        class={`SidebarBoardItem SidebarInboxItem subitem ${props.isActive ? 'active' : ''}`}
        onClick={() => props.onClick(props.board.id)}
    >
        <div class='octo-sidebar-icon'>
            {props.board.icon}
        </div>
        <div
            class='octo-sidebar-title'
            title={props.board.title}
        >
            {props.board.title}
        </div>
    </div>
)

export default SidebarInboxItem
