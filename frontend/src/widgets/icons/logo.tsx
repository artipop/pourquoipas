import type {JSX} from '@solidjs/web'

import './logo.scss'
import CompassIcon from './compassIcon'

export default function LogoIcon(): JSX.Element {
    return (
        <CompassIcon
            icon='product-boards'
            class='boards-rhs-icon'
        />
    )
}
