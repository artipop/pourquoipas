// Every theme this build ships, in the order the dropdown draws them.
//
// They are JSON files rather than objects in this module because a theme is a
// config: the same text a person pastes into the settings is the text that is
// committed here, so "our theme" and "a theme somebody found" are the same
// kind of thing and one can be made from the other by copying.
//
// The three groups are what a person is choosing between — the product's own
// look, a look borrowed from somewhere else, and a joke — and the titles they
// are drawn under live in the panel, where the rest of the catalogue is.

import acidBlueYellowDark from './themes/acid-blue-yellow-dark.json'
import acidBlueYellowLight from './themes/acid-blue-yellow-light.json'
import acidTechnoGreenDark from './themes/acid-techno-green-dark.json'
import acidTechnoGreenLight from './themes/acid-techno-green-light.json'
import darkTheme from './themes/dark-theme.json'
import discordDark from './themes/discord-dark.json'
import githubLight from './themes/github-light.json'
import gruvboxDark from './themes/gruvbox-dark.json'
import lightTheme from './themes/light-theme.json'
import monokaiLight from './themes/monokai-light.json'
import nightOwlDark from './themes/night-owl-dark.json'
import oneDark from './themes/one-dark.json'
import solarizedDark from './themes/solarized-dark.json'
import windowsDark from './themes/windows-dark.json'

import {ThemeConfig} from './config'

export type ThemeGroupId = 'xciii' | 'ported' | 'acid'

export type ThemeGroup = {
    id: ThemeGroupId
    themes: ThemeConfig[]
}

// The two the product was designed in. Their ids are the names the setting has
// stored since before themes were configs, so an install that picked one keeps
// it without a migration.
export const THEME_GROUPS: ThemeGroup[] = [
    {
        id: 'xciii',
        themes: [lightTheme, darkTheme],
    },
    {

        // Mattermost's own community list, run through the derivation and kept
        // where it came out looking like this product rather than like a
        // recolouring of another one. What each of them had to be told is in
        // `docs/themes.md`.
        id: 'ported',
        themes: [
            windowsDark,
            oneDark,
            gruvboxDark,
            nightOwlDark,
            discordDark,
            solarizedDark,
            githubLight,
            monokaiLight,
        ],
    },
    {
        id: 'acid',
        themes: [
            acidBlueYellowLight,
            acidBlueYellowDark,
            acidTechnoGreenLight,
            acidTechnoGreenDark,
        ],
    },
]

export const SHIPPED_THEMES: ThemeConfig[] = THEME_GROUPS.flatMap((group) => group.themes)

export function shippedTheme(id: string): ThemeConfig | undefined {
    return SHIPPED_THEMES.find((theme) => theme.id === id)
}
