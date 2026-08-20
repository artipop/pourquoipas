import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './help.scss'

export default function HelpIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='help-circle-outline'
            class='HelpIcon'
        />
    )
}
