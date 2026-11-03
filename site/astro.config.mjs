import {unified} from '@astrojs/markdown-remark'
import starlight from '@astrojs/starlight'
import {defineConfig} from 'astro/config'

import {guideLinks} from './src/guide/links.mjs'

// One site: the landing page at the root and the user guide under /docs/,
// written by Starlight. The guide's pages stay in docs/guide/ — the repository's
// rule is that a feature is not finished until that directory says how — and
// src/content.config.ts reads them from there.
//
// The guide is Russian at /docs/ and English at /en/docs/: Starlight takes a
// locale from the first segment of a page's address, so /docs/en/ is not
// something it can serve. /en/ is then also where an English landing page
// would go.
// GitHub Pages serves a project's site under /<repository>/; the workflow
// passes that path (.github/workflows/pages.yml). Anywhere else it is the root.
const base = (process.env.BASE_PATH ?? '/').replace(/\/?$/, '/')

export default defineConfig({
    site: process.env.SITE_ORIGIN,
    base,
    integrations: [
        starlight({
            title: {ru: 'Руководство Pourquoi Pas?', en: 'Pourquoi Pas? guide'},
            description: 'Руководство пользователя Pourquoi Pas?: доска, агенты, Входящие и настройки.',
            logo: {src: './src/assets/icon.png', alt: 'Pourquoi Pas?'},
            favicon: '/favicon.png',
            head: [
                {tag: 'link', attrs: {rel: 'icon', href: `${base}favicon-dark.png`, media: '(prefers-color-scheme: dark)'}},
            ],
            defaultLocale: 'root',
            locales: {
                root: {label: 'Русский', lang: 'ru'},
                en: {label: 'English', lang: 'en'},
            },
            lastUpdated: true,
            customCss: ['./src/styles/fonts.css', './src/styles/guide.css'],
            expressiveCode: {
                themes: ['github-dark', 'github-light'],
                styleOverrides: {borderRadius: '2px'},
            },
            components: {
                SiteTitle: './src/guide/SiteTitle.astro',
            },
            sidebar: [
                item('docs', 'Обзор', 'Overview'),
                {
                    label: 'Доска',
                    translations: {en: 'The board'},
                    items: [
                        item('docs/start', 'Первый запуск', 'First run'),
                        item('docs/board', 'Название, описание, меню', 'Title, description, menu'),
                        item('docs/agent', 'Агент на карточке', 'The agent on a card'),
                        item('docs/folders', 'Папки и ветки', 'Folders and branches'),
                    ],
                },
                {
                    label: 'Автоматизация',
                    translations: {en: 'Automation'},
                    items: [
                        item('docs/automation', 'Как доска запускает агента', 'How the board starts an agent'),
                        item('docs/automation/prompt', 'Системный промпт доски', 'The board’s system prompt'),
                    ],
                },
                {
                    label: 'Входящие',
                    translations: {en: 'Inbox'},
                    items: [
                        item('docs/inbox', 'Обзор', 'Overview'),
                        item('docs/inbox/sources', 'Источники', 'Sources'),
                        item('docs/inbox/sorting', 'Разбор входящих', 'Sorting the inbox'),
                    ],
                },
                {
                    label: 'Настройки',
                    translations: {en: 'Settings'},
                    items: [
                        item('docs/settings', 'Обзор', 'Overview'),
                        item('docs/settings/themes', 'Светлая и тёмная', 'Light and dark'),
                        item('docs/settings/ai', 'Подключение агентов', 'Connecting agents'),
                        item('docs/settings/agents', 'Программы-агенты, npx и Node.js', 'Program agents, npx and Node.js'),
                        item('docs/settings/team', 'Свой сервер', 'Your own server'),
                        item('docs/modes', 'Компьютер и сервер: что где работает', 'Computer and server: what works where'),
                        item('docs/settings/updates', 'Обновления', 'Updates'),
                    ],
                },
                {
                    label: 'Перенос',
                    translations: {en: 'Moving'},
                    items: [
                        item('docs/transfer', 'Перенос доски', 'Moving a board'),
                    ],
                },
            ],
        }),
    ],
    markdown: {
        processor: unified({remarkPlugins: [[guideLinks, {base}]]}),
    },
})

// A sidebar line names the Russian page; Starlight finds the English one under
// /en/ by itself and takes the label from `translations`.
function item(slug, ru, en) {
    return {slug, label: ru, translations: {en}}
}
