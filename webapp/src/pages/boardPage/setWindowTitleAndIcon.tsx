import {createEffect} from 'solid-js'

import {Utils} from '../../utils'
import {getCurrentBoard} from '../../store/boards'
import {getCurrentView} from '../../store/views'
import {useAppSelector} from '../../store/hooks'

const SetWindowTitleAndIcon = (): null => {
    const board = useAppSelector(getCurrentBoard)
    const activeView = useAppSelector(getCurrentView)

    createEffect(() => board()?.icon, (icon) => Utils.setFavicon(icon))

    createEffect(() => [board(), activeView()?.title] as const, ([currentBoard]) => {
        if (currentBoard) {
            let title = `${currentBoard.title}`
            if (activeView()?.title) {
                title += ` | ${activeView().title}`
            }
            document.title = title
        } else {
            document.title = 'XCIII'
        }
    })

    return null
}

export default SetWindowTitleAndIcon
