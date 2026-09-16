// Every theme this build ships, in the order the list draws them.
//
// A theme is a **pair**: the same design in light and in dark. That is what
// splits the one question the picker used to ask into the two it was really
// asking — which palette, and which side of the line — and it is why there is
// no «Windows Dark» in the list any more, only «Windows», with the dark half
// chosen by the switch beside it.
//
// The halves are JSON files rather than objects in this module because a
// variant is a config: the same text a person pastes into the settings is the
// text committed here, so "our theme" and "a theme somebody found" are the
// same kind of thing and one can be made from the other by copying.
//
// The list is flat and the order is all the grouping there is: the product's
// own theme, then the ones borrowed from somewhere else, then the jokes. It
// carried headings for those three for a while, and they were three lines of
// furniture over eleven entries a person reads by name anyway.

import acidBlueYellowDark from './themes/acid-blue-yellow-dark.json'
import acidBlueYellowLight from './themes/acid-blue-yellow-light.json'
import acidTechnoGreenDark from './themes/acid-techno-green-dark.json'
import acidTechnoGreenLight from './themes/acid-techno-green-light.json'
import darkTheme from './themes/dark-theme.json'
import discordDark from './themes/discord-dark.json'
import discordLight from './themes/discord-light.json'
import githubDark from './themes/github-dark.json'
import githubLight from './themes/github-light.json'
import gruvboxDark from './themes/gruvbox-dark.json'
import gruvboxLight from './themes/gruvbox-light.json'
import lightTheme from './themes/light-theme.json'
import monokaiDark from './themes/monokai-dark.json'
import monokaiLight from './themes/monokai-light.json'
import nightOwlDark from './themes/night-owl-dark.json'
import nightOwlLight from './themes/night-owl-light.json'
import oneDark from './themes/one-dark.json'
import oneLight from './themes/one-light.json'
import solarizedDark from './themes/solarized-dark.json'
import solarizedLight from './themes/solarized-light.json'
import windowsDark from './themes/windows-dark.json'
import windowsLight from './themes/windows-light.json'

import {ThemeConfig} from './config'

export type ThemeFamily = {
    id: string
    name: string
    light: ThemeConfig
    dark: ThemeConfig
}

// The name in the list is the family's, not the file's: a variant keeps its own
// («Gruvbox Light», «XCIII — бумага») because that is what it is, and because a
// config exported from here should say which half it is.
export const THEME_FAMILIES: ThemeFamily[] = [

    // The one the product was designed in. Its id is `xciii`, and the two
    // halves keep the names the setting stored before themes were configs, so
    // an install that picked one is read without a migration table.
    {id: 'xciii', name: 'XCIII', light: lightTheme, dark: darkTheme},

    // Mattermost's own community list. Each of those themes exists in one
    // polarity there and in both upstream, so the missing half is the real one
    // — One Light, Gruvbox Light, Light Owl, Solarized Light and the rest —
    // rather than an inversion of the half that was published for a chat
    // client. `docs/themes.md` says what each of them had to be told.
    {id: 'windows', name: 'Windows', light: windowsLight, dark: windowsDark},
    {id: 'one', name: 'One', light: oneLight, dark: oneDark},
    {id: 'gruvbox', name: 'Gruvbox', light: gruvboxLight, dark: gruvboxDark},
    {id: 'night-owl', name: 'Night Owl', light: nightOwlLight, dark: nightOwlDark},
    {id: 'discord', name: 'Discord', light: discordLight, dark: discordDark},
    {id: 'solarized', name: 'Solarized', light: solarizedLight, dark: solarizedDark},
    {id: 'github', name: 'GitHub', light: githubLight, dark: githubDark},
    {id: 'monokai', name: 'Monokai', light: monokaiLight, dark: monokaiDark},

    {
        id: 'acid-blue-yellow',
        name: 'Кислота: синий и жёлтый',
        light: acidBlueYellowLight,
        dark: acidBlueYellowDark,
    },
    {
        id: 'acid-techno-green',
        name: 'Эйсид-техно',
        light: acidTechnoGreenLight,
        dark: acidTechnoGreenDark,
    },
]

export const SHIPPED_VARIANTS: ThemeConfig[] = THEME_FAMILIES.flatMap((f) => [f.light, f.dark])

export function themeFamily(id: string): ThemeFamily | undefined {
    return THEME_FAMILIES.find((family) => family.id === id)
}

// Which family a *variant* belongs to, and which half it is. This is the whole
// of the migration: a setting written when the list held one entry per variant
// named that variant, and it still names a real theme — `windows-dark` is
// «Windows», dark.
export function familyOfVariant(variantId: string): {family: ThemeFamily, dark: boolean} | undefined {
    for (const family of THEME_FAMILIES) {
        if (family.light.id === variantId) {
            return {family, dark: false}
        }
        if (family.dark.id === variantId) {
            return {family, dark: true}
        }
    }
    return undefined
}
