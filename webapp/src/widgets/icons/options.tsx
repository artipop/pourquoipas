import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './options.scss'

export default function OptionsIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='dots-horizontal'
            class='OptionsIcon'
        />
    )
}
