import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function MessageIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='message-text-outline'
            class='MessageIcon'
        />
    )
}
