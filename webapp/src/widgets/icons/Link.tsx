import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './link.scss'

export default function LinkIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='link-variant'
            class='LinkIcon'
        />
    )
}
