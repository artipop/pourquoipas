import {Block, createBlock} from './block'

type CardFields = {
    icon?: string
    isTemplate?: boolean

    // The card a board made from a template opens with, the one that says how
    // the board works («Where to start»). A mark rather than a title, because
    // nothing is found by its name; the onboarding points at it.
    readFirst?: boolean
    properties: Record<string, string | string[]>
    contentOrder: Array<string | string[]>
}

type Card = Block & {
    fields: CardFields
}

function createCard(block?: Block): Card {
    const contentOrder: Array<string|string[]> = []
    const contentIds = block?.fields?.contentOrder?.filter((id: any) => id !== null)

    if (contentIds?.length > 0) {
        for (const contentId of contentIds) {
            if (typeof contentId === 'string') {
                contentOrder.push(contentId)
            } else {
                contentOrder.push(contentId.slice())
            }
        }
    }
    return {
        ...createBlock(block),
        type: 'card',
        fields: {
            icon: block?.fields.icon || '',
            properties: {...(block?.fields.properties || {})},
            contentOrder,
            isTemplate: block?.fields.isTemplate || false,
            ...(block?.fields.readFirst ? {readFirst: true} : {}),
        },
    }
}

export {type Card, createCard}
