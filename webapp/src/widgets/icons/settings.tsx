import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './settings.scss'

export default function SettingsIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='settings-outline'
            class='SettingsIcon'
        />
    )
}
