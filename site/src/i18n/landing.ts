// The landing page's text in both languages. English is the root (/), Russian
// lives under /ru/. Strings are HTML: they carry &nbsp; to keep short words off
// the end of a line, and nothing else. UI names quote frontend/i18n/{en,ru}.json.

export type Lang = 'en' | 'ru'

interface Pair {
    title: string
    text: string
}

interface Card {
    title: string
    who: string
    person?: boolean
    asking?: string
    question?: string
    answers?: [string, string]
    comment?: string
    working?: boolean
}

export interface LandingStrings {
    lang: Lang
    title: string
    description: string
    nav: {how: string; inbox: string; docs: string; price: string; language: string; theme: string}
    hero: {label: string; headline: string; lead: string; text: string; preorder: string; how: string; shipAlt: string}
    how: {label: string; title: string; columns: {name: string; cards: Card[]}[]; steps: Pair[]; more: string}
    inbox: {label: string; title: string; lead: string; tiles: Pair[]; moves: string; planTitle: string; planText: string}
    stepIn: {label: string; items: {title: string; text: string; muted: string}[]}
    facts: {label: string; title: string; rows: Pair[]}
    price: {label: string; badge: string; title: string; text: string; preorder: string}
    form: {email: string; name: string; os: string; submit: string; note: string; subject: string; bodyEmail: string; bodyName: string; bodyOs: string}
    status: {badEmail: string; mailOpened: string; sending: string; sent: string; failedTo: string; failed: string}
    footer: {tagline: string; docs: string; other: string}
}

const en: LandingStrings = {
    lang: 'en',
    title: 'Pourquoi Pas? — a board that AI agents work on',
    description: 'Pourquoi Pas? is a board that AI agents work on: an agent is assigned to a card and takes it up when the card is moved into a column. Everything is local, with your agents or your API key.',
    nav: {how: 'How it works', inbox: 'Inbox', docs: 'Guide', price: 'Price', language: 'Language', theme: 'Switch theme'},
    hero: {
        label: 'Board · AI agents · macOS, Windows, Linux',
        headline: 'Free time is somewhere near.',
        lead: 'A board where the work gets done: an agent is the assignee of a card, like a person, and takes it up when you move the card into a column.',
        text: 'The agent does the task and leaves the result in a comment on the card. Then it moves the card itself, into the column whose name says how it went.',
        preorder: 'Tell me when it’s out',
        how: 'How it works',
        shipAlt: 'The liner «Pourquoi Pas?»',
    },
    how: {
        label: 'How it works',
        title: 'Assignee, column, description',
        columns: [
            {name: 'To do', cards: [
                {title: 'Update the frontend dependencies', who: 'Claude Code'},
                {title: 'Reply to a customer about export', who: 'Anna', person: true},
            ]},
            {name: 'In progress', cards: [
                {title: 'Move the settings to the new format', who: 'Codex', asking: 'The agent is asking', question: 'Delete the old file or keep it alongside?', answers: ['Delete', 'Keep']},
                {title: 'Tests for archive import', who: 'Claude Code', working: true},
            ]},
            {name: 'Done', cards: [
                {title: 'Why the Linux build fails', who: 'Claude Code', comment: 'The CI image was missing libwebkit2gtk. Added it, the build passes.'},
            ]},
            {name: 'Didn’t work', cards: []},
        ],
        steps: [
            {title: 'Assignee', text: 'An agent is made the assignee of a card, like a person. Any of your agents fits any board; there is no need to add it to the board.'},
            {title: 'Column', text: 'Moving a card into a column starts the agent. It knows the board’s columns and, when done, moves the card into the one that says how it ended. Columns need no setting up.'},
            {title: 'Task', text: 'The agent reads its task from the card’s description. The board adds its own instruction, shared by all of its agents.'},
        ],
        more: 'Step by step&nbsp;— in the <a href="{docs}automation/">guide</a>.',
    },
    inbox: {
        label: 'Inbox',
        title: 'Cards arrive on their own',
        lead: 'Everything that comes from outside lands on one board, the Inbox&nbsp;— a column per source. From there a card is moved to the board it belongs to.',
        tiles: [
            {title: 'Kaiten', text: 'Tasks assigned to you in Kaiten appear in the Inbox by themselves. One task is one card.'},
            {title: '«Share»', text: 'On a Mac a link or text from any app goes to the Inbox or to a board through the system «Share» menu.'},
            {title: 'HTTP', text: 'Every source has an address and a token. A card can be created by a script, a webhook or a Shortcut on a phone.'},
        ],
        moves: 'A card moves to another board together with its comments.',
        planTitle: 'Or cut them with AI',
        planText: 'This is planning: «New» → «Talk it over with an agent» opens a conversation with an agent that reads the project folder. The cards you agree on, the agent creates on the board itself.',
    },
    stepIn: {
        label: 'When you need to step in',
        items: [
            {title: 'Card history', text: 'The «History» button shows who changed what on the card: a person or an agent, each under their own name.', muted: 'An agent’s work in a repository stays a separate branch, and your working copy does not change.'},
            {title: 'The agent’s questions', text: 'When an agent needs a decision or a permission, it stops and asks. The question with its answers arrives on the card and as a notification.', muted: 'The rest of the work goes on meanwhile.'},
        ],
    },
    facts: {
        label: 'What this program is',
        title: 'No cloud, and your own agents',
        rows: [
            {title: 'All on your computer', text: 'The board, the files and the agents run locally. We have no servers and no accounts.'},
            {title: 'Your AI', text: 'Pourquoi Pas? runs the agents you already have installed, under your own account: a Claude or Codex subscription is enough. Instead of a program you can connect a model with your own Anthropic or OpenAI API key. Both are connected in one place, in the settings.'},
            {title: 'Supported agents', text: 'Claude Code, Codex, GitHub Copilot CLI, Antigravity, Junie and any agent that speaks ACP.'},
            {title: 'Separate copies', text: 'An agent works in its own copy of the project and does not touch the one you have open. Several cards run at once.'},
            {title: 'Platforms', text: 'macOS, Windows and Linux.'},
        ],
    },
    price: {
        label: 'Price',
        badge: 'The app is free',
        title: 'Only AI through our subscription is paid',
        text: 'The board, the templates, agents on your computer and your own API key work without payment. The subscription will give AI without a key of your own and without an installed agent; it is not ready yet.',
        preorder: 'Leave your email&nbsp;— we will write when the build for your system is out.',
    },
    form: {
        email: 'Email',
        name: 'Name (optional)',
        os: 'System',
        submit: 'Tell me when it’s out',
        note: 'We will write once, when the build is out. No newsletters.',
        subject: 'Pourquoi Pas?: tell me when it’s out',
        bodyEmail: 'Email',
        bodyName: 'Name',
        bodyOs: 'System',
    },
    status: {
        badEmail: 'Check the email address.',
        mailOpened: 'Your mail app is open with the letter filled in; it only needs sending.',
        sending: 'Sending…',
        sent: 'Done. We will write when the build is out.',
        failedTo: 'It did not go through. Write to us at {email} and we will add you by hand.',
        failed: 'It did not go through. Please try again later.',
    },
    footer: {tagline: 'A board that AI agents work on.', docs: 'Guide', other: 'Русский'},
}

const ru: LandingStrings = {
    lang: 'ru',
    title: 'Pourquoi Pas? — доска, на которой работают ИИ-агенты',
    description: 'Pourquoi Pas? — доска, на которой работают ИИ-агенты: агент — исполнитель карточки и берётся за неё, когда её переносят в колонку. Всё локально, с вашими агентами или ключом API.',
    nav: {how: 'Как это работает', inbox: 'Входящие', docs: 'Руководство', price: 'Цена', language: 'Язык', theme: 'Переключить тему'},
    hero: {
        label: 'Доска · ИИ-агенты · macOS, Windows, Linux',
        headline: 'Свободное время где-то рядом.',
        lead: 'Доска, на&nbsp;которой работа делается: агент&nbsp;— исполнитель карточки, как человек, и&nbsp;берётся за&nbsp;неё, когда вы&nbsp;переносите её&nbsp;в&nbsp;колонку.',
        text: 'Агент делает задачу и&nbsp;оставляет результат в&nbsp;комментарии карточки. Потом сам переносит её&nbsp;в&nbsp;колонку, название которой говорит, что вышло.',
        preorder: 'Узнать о&nbsp;выходе',
        how: 'Как это работает',
        shipAlt: 'Лайнер «Pourquoi Pas?»',
    },
    how: {
        label: 'Как это работает',
        title: 'Исполнитель, колонка, описание',
        columns: [
            {name: 'Сделать', cards: [
                {title: 'Обновить зависимости фронтенда', who: 'Claude Code'},
                {title: 'Ответ клиенту про экспорт', who: 'Анна', person: true},
            ]},
            {name: 'В работе', cards: [
                {title: 'Перевести настройки на новый формат', who: 'Codex', asking: 'Агент спрашивает', question: 'Старый файл удалить или оставить рядом?', answers: ['Удалить', 'Оставить']},
                {title: 'Тесты для импорта архива', who: 'Claude Code', working: true},
            ]},
            {name: 'Готово', cards: [
                {title: 'Почему падает сборка на Linux', who: 'Claude Code', comment: 'Не хватало пакета libwebkit2gtk в образе CI. Добавил, сборка проходит.'},
            ]},
            {name: 'Не вышло', cards: []},
        ],
        steps: [
            {title: 'Исполнитель', text: 'Агента назначают исполнителем карточки, как человека. Любой ваш агент годится для&nbsp;любой доски; добавлять его на&nbsp;доску не&nbsp;нужно.'},
            {title: 'Колонка', text: 'Перенос карточки в&nbsp;колонку запускает агента. Он&nbsp;знает колонки доски и, закончив, сам переносит карточку в&nbsp;ту, что говорит, чем всё кончилось. Настраивать колонки не&nbsp;нужно.'},
            {title: 'Задание', text: 'Задание агент читает из&nbsp;описания карточки. К&nbsp;нему доска добавляет свою инструкцию&nbsp;— общую для&nbsp;всех её&nbsp;агентов.'},
        ],
        more: 'Подробно, по&nbsp;шагам&nbsp;— в&nbsp;<a href="{docs}automation/">руководстве</a>.',
    },
    inbox: {
        label: 'Входящие',
        title: 'Карточки приходят сами',
        lead: 'Всё, что пришло извне, складывается на&nbsp;одну доску Входящие&nbsp;— по&nbsp;колонке на&nbsp;каждый источник. Оттуда карточка переносится на&nbsp;доску, к&nbsp;которой относится.',
        tiles: [
            {title: 'Kaiten', text: 'Задачи, назначенные на&nbsp;вас в&nbsp;Kaiten, появляются во&nbsp;Входящих сами. Одна задача&nbsp;— одна карточка.'},
            {title: '«Поделиться»', text: 'На&nbsp;Mac ссылка или текст из&nbsp;любого приложения отправляется во&nbsp;Входящие или на&nbsp;доску через системное меню «Поделиться».'},
            {title: 'HTTP', text: 'У&nbsp;каждого источника есть адрес и&nbsp;токен. Карточку может создать скрипт, вебхук или «Быстрая команда» на&nbsp;телефоне.'},
        ],
        moves: 'Карточка переносится на&nbsp;другую доску вместе со&nbsp;своими комментариями.',
        planTitle: 'Или вы их нарежете с&nbsp;помощью&nbsp;ИИ',
        planText: 'Это режим планирования: «Создать» → «Обсудить с&nbsp;агентом» открывает разговор с&nbsp;агентом, который читает папку проекта. Карточки, о&nbsp;которых вы&nbsp;договорились, агент заводит на&nbsp;доске сам.',
    },
    stepIn: {
        label: 'Когда нужно вмешаться',
        items: [
            {title: 'История карточки', text: 'Кнопка «История» показывает, кто и&nbsp;что менял в&nbsp;карточке: человек или агент, каждый под своим именем.', muted: 'Работа агента в&nbsp;репозитории остаётся отдельной веткой, а&nbsp;ваша рабочая копия не&nbsp;меняется.'},
            {title: 'Вопросы агента', text: 'Если агенту нужно решение или разрешение, он&nbsp;останавливается и&nbsp;спрашивает. Вопрос с&nbsp;вариантами ответа приходит на&nbsp;карточку и&nbsp;уведомлением.', muted: 'Остальную работу агент в&nbsp;это время продолжает.'},
        ],
    },
    facts: {
        label: 'Что это за программа',
        title: 'Без облака и&nbsp;с&nbsp;вашими агентами',
        rows: [
            {title: 'Всё на&nbsp;вашем компьютере', text: 'Доска, файлы и&nbsp;агенты работают локально. Серверов и&nbsp;учётных записей у&nbsp;нас нет.'},
            {title: 'Ваш ИИ', text: 'Pourquoi Pas? запускает агентов, которые уже установлены у&nbsp;вас, под&nbsp;вашей учётной записью: подписки на&nbsp;Claude или Codex достаточно. Вместо программы можно подключить модель по&nbsp;своему ключу API Anthropic или OpenAI. И&nbsp;то, и&nbsp;другое подключается в&nbsp;одном месте, в&nbsp;настройках.'},
            {title: 'Поддерживаемые агенты', text: 'Claude Code, Codex, GitHub Copilot CLI, Antigravity, Junie и&nbsp;любой агент с&nbsp;поддержкой ACP.'},
            {title: 'Отдельные копии', text: 'Агент работает в&nbsp;отдельной копии проекта и&nbsp;не&nbsp;трогает ту, что открыта у&nbsp;вас. Несколько карточек идут одновременно.'},
            {title: 'Платформы', text: 'macOS, Windows и&nbsp;Linux.'},
        ],
    },
    price: {
        label: 'Цена',
        badge: 'Приложение бесплатное',
        title: 'Платно только ИИ по&nbsp;нашей подписке',
        text: 'Доска, шаблоны, агенты на&nbsp;вашем компьютере и&nbsp;ваш ключ API работают без&nbsp;оплаты. Подписка даст ИИ без&nbsp;своего ключа и&nbsp;без&nbsp;установленного агента; она ещё не&nbsp;готова.',
        preorder: 'Оставьте почту&nbsp;— мы&nbsp;напишем, когда выйдет сборка под&nbsp;вашу систему.',
    },
    form: {
        email: 'Почта',
        name: 'Имя (необязательно)',
        os: 'Система',
        submit: 'Написать мне о&nbsp;выходе',
        note: 'Мы&nbsp;напишем один раз, когда выйдет сборка. Рассылок не&nbsp;будет.',
        subject: 'Pourquoi Pas?: напишите о выходе',
        bodyEmail: 'Почта',
        bodyName: 'Имя',
        bodyOs: 'Система',
    },
    status: {
        badEmail: 'Проверьте адрес почты.',
        mailOpened: 'Открыли почтовую программу — письмо заполнено, осталось отправить.',
        sending: 'Отправляем…',
        sent: 'Записали. Напишем, когда выйдет сборка.',
        failedTo: 'Не отправилось. Напишите нам на {email}, мы запишем вас вручную.',
        failed: 'Не отправилось. Попробуйте позже.',
    },
    footer: {tagline: 'Доска, на&nbsp;которой работают ИИ-агенты.', docs: 'Руководство', other: 'English'},
}

export const landing = {en, ru}
