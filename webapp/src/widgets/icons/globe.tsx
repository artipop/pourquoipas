import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function Globe(): JSX.Element {
    return (
        <CompassIcon
            icon='globe'
            class='GlobeIcon'
        />
    )
}
