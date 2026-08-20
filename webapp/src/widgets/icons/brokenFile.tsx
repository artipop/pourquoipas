import type {JSX} from '@solidjs/web'

import CompassIcon from './compassIcon'

export default function BrokenFile(): JSX.Element {
    return (
        <CompassIcon
            icon='file-image-broken-outline'
            class='BrokenFile'
        />
    )
}
