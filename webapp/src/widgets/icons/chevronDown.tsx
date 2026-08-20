import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function ChevronDown(): JSX.Element {
    return (
        <CompassIcon
            icon='chevron-down'
            class='ChevronDownIcon'
        />
    )
}
