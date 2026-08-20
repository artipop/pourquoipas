import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function AlertIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='alert-outline'
            class='AlertIcon'
        />
    )
}
