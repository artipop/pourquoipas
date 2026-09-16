import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

import './duplicate.scss'

export default function DuplicateIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='content-copy'
            class='content-copy'
        />
    )
}
