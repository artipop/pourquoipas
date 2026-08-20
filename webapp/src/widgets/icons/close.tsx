import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function CloseIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='close'
            class='CloseIcon'
        />
    )
}
