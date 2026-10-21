import type {JSX} from '@solidjs/web'

import logo from '../../assets/appLogo.png'
import logoDark from '../../assets/appLogoDark.png'

import './appLogo.scss'

// The app icon itself, corners and all, written by tools/appicon from the same
// pictures. A picture does not take the sidebar's text colour, so unlike the
// other icons in this directory it is an image rather than an inline SVG. The
// night picture is chosen by the theme's half, not the system's: the two can
// differ (appPanel).
export default function AppLogoIcon(): JSX.Element {
    return (
        <picture class='AppLogoIcon Icon'>
            <img
                class='AppLogoIcon__day'
                src={logo}
                alt=''
            />
            <img
                class='AppLogoIcon__night'
                src={logoDark}
                alt=''
            />
        </picture>
    )
}
