import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

// TODO use this icon instead of check.tsx
export default function Check(): JSX.Element {
    return (
        <CompassIcon
            icon='check'
            class='CheckIconCompass'
        />
    )
}
