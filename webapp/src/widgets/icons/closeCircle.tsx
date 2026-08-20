import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './add.scss'

export default function CloseCircle(): JSX.Element {
    return (
        <CompassIcon
            icon='close-circle'
            class='CloseCircle'
        />
    )
}
