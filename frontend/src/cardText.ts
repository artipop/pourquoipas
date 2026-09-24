import {Board} from './blocks/board'
import {Card} from './blocks/card'
import {ContentBlock} from './blocks/contentBlock'
import {IntlShape} from './intl'
import propsRegistry from './properties'

// A card as Markdown, for handing it to another application -- mostly a
// terminal an agent runs in, where a drop is typed into the prompt. So it is
// what a person would paste to describe the task: the title, the fields that
// hold a value, and the description. Fields computed from the block (created,
// updated) are left out, because a card that sets nothing carries none of them.
export function cardAsText(card: Card, board: Board, contents: Array<ContentBlock | ContentBlock[]>, intl: IntlShape): string {
    const parts: string[] = []

    const title = card.title.trim() || intl.formatMessage({id: 'KanbanCard.untitled', defaultMessage: 'Untitled'})
    parts.push(`# ${title}`)

    const fields: string[] = []
    for (const template of board.cardProperties) {
        const value = card.fields.properties[template.id]
        if (value === undefined || value === '' || (Array.isArray(value) && value.length === 0)) {
            continue
        }
        const shown = propsRegistry.get(template.type).displayValue(value, card, template, intl)
        const text = Array.isArray(shown) ? shown.filter(Boolean).join(', ') : (shown || '')
        if (text) {
            fields.push(`${template.name}: ${text}`)
        }
    }
    if (fields.length > 0) {
        parts.push(fields.join('\n'))
    }

    const body = contents.flat().map(contentAsText).filter(Boolean)
    if (body.length > 0) {
        parts.push(body.join('\n\n'))
    }

    return parts.join('\n\n')
}

function contentAsText(block: ContentBlock): string {
    const text = block.title.trim()
    switch (block.type) {
    case 'text':
        return text
    case 'h1':
        return text && `## ${text}`
    case 'h2':
        return text && `### ${text}`
    case 'h3':
        return text && `#### ${text}`
    case 'list-item':
        return text && `- ${text}`
    case 'checkbox':
        return text && `- [${block.fields.value ? 'x' : ' '}] ${text}`
    case 'quote':
        return text && `> ${text}`
    case 'divider':
        return '---'
    default:
        // Images, videos and attachments are files on the board's server; a
        // file name alone gives the reader nothing to open.
        return ''
    }
}
