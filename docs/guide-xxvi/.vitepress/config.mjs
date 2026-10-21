import {defineConfig} from 'vitepress'

// The XXVI guide. A second site rather than a second section of the Pourquoi Pas? one:
// the two are different products, and a reader who came for one should not have
// to step over the other's sidebar. Everything else — VitePress, the theme, the
// place in the deploy — is deliberately the same as `docs/guide/`, so there is
// one set of habits for both.
//
// It is short on purpose. The product is at the beginning of its way, and a
// guide that describes screens which are still moving would be a promise the
// application has not made yet. English will follow when the Russian pages stop
// changing week to week.

export default defineConfig({
    title: 'XXVI',
    lang: 'ru-RU',
    titleTemplate: ':title — руководство XXVI',
    description: 'Руководство пользователя XXVI: входящие, флоу, лента и агенты.',

    // The product is a screen, so the screen theme is the one a reader arrives
    // in — the same decision the landing page and the Pourquoi Pas? guide make.
    appearance: 'dark',
    lastUpdated: true,

    // Links keep their `.html`, for the same reason as in the Pourquoi Pas? guide: a
    // guide that 404s depending on where it was uploaded is worse than one with
    // an extension in the address.
    cleanUrls: false,

    // Published beside the Pourquoi Pas? guide, inside the site's own dist, so that a
    // deploy stays one directory. `build:all` in site/ runs the landing first
    // (it empties dist/), then the Pourquoi Pas? guide (it empties dist/docs), then
    // this one — the order is the whole reason it is a single script.
    base: '/docs/xxvi/',
    outDir: '../../site/dist/docs/xxvi',

    head: [
        ['link', {rel: 'icon', href: '/docs/xxvi/favicon.svg'}],
    ],

    markdown: {
        theme: {light: 'github-light', dark: 'github-dark'},
    },

    themeConfig: {
        siteTitle: 'XXVI · Руководство',

        nav: [
            {text: 'Начало', link: '/start'},
            {text: 'Флоу', link: '/flow'},
            {text: 'Лента', link: '/ribbon'},
            {text: 'Агенты', link: '/agents'},
        ],

        sidebar: [
            {
                text: 'Руководство',
                items: [
                    {text: 'Первый запуск', link: '/start'},
                    {text: 'Флоу и стадии', link: '/flow'},
                    {text: 'Лента', link: '/ribbon'},
                    {text: 'Агенты и вопросы', link: '/agents'},
                ],
            },
        ],

        // Everything a reader sees is Russian, including the theme's own
        // furniture — the defaults are English.
        outline: {level: [2, 3], label: 'На этой странице'},
        docFooter: {prev: 'Назад', next: 'Дальше'},
        returnToTopLabel: 'Наверх',
        sidebarMenuLabel: 'Разделы',
        darkModeSwitchLabel: 'Оформление',
        lightModeSwitchTitle: 'Светлая тема',
        darkModeSwitchTitle: 'Тёмная тема',
        lastUpdatedText: 'Обновлено',

        footer: {
            message: 'Руководство пользователя XXVI',
            copyright: '© 2026 deffun',
        },

        notFound: {
            title: 'Такой страницы нет',
            quote: 'Возможно, раздел переехал — он должен быть в списке слева.',
            linkText: 'К началу руководства',
        },

        search: {
            provider: 'local',
            options: {
                translations: {
                    button: {
                        buttonText: 'Поиск',
                        buttonAriaLabel: 'Поиск по руководству',
                    },
                    modal: {
                        displayDetails: 'Показать подробности',
                        resetButtonTitle: 'Очистить',
                        backButtonTitle: 'Закрыть',
                        noResultsText: 'Ничего не нашлось',
                        footer: {
                            selectText: 'открыть',
                            navigateText: 'листать',
                            closeText: 'закрыть',
                        },
                    },
                },
            },
        },
    },
})
