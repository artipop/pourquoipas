import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function Update(): JSX.Element {
    return (
        <CompassIcon
            icon='update'
            class='UpdateIcon'
        />
    )
}
