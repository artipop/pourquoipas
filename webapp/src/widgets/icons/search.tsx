import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function Search(): JSX.Element {
    return (
        <CompassIcon
            icon='magnify'
            class='MagnifyIcon'
        />
    )
}
