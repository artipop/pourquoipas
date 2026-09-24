import {For, Show} from 'solid-js'

import {useIntl} from '../../intl'

import {CardHistoryEntry} from '../../blocks/cardHistory'
import octoClient from '../../octoClient'
import {createLoader} from '../../hooks/loader'
import {useAppSelector} from '../../store/hooks'
import {getBoardUsers} from '../../store/users'
import {IUser} from '../../user'
import {Utils} from '../../utils'
import CompassIcon from '../../widgets/icons/compassIcon'
import IconButton from '../../widgets/buttons/iconButton'

import './cardHistory.scss'

// Who did what to this card.
//
// The board has always kept it — every write leaves a copy of the block with
// the id of whoever made it — and until this there was nowhere to read it. It
// answers the question a person actually asks of a card they did not watch:
// not "what does it say now" but "who changed it, and to what".
//
// Nothing is computed here. The server turns the copies into the difference
// between them (`server/app/cardhistory.go`), because naming a property or an
// option needs the board's schema.

type Props = {
    cardId: string
    onClose: () => void
}

const CardHistory = (props: Props) => {
    const intl = useIntl()
    const boardUsers = useAppSelector<{[key: string]: IUser}>(getBoardUsers)

    const [entries] = createLoader(
        (cardId: string) => octoClient.getCardHistory(cardId).catch(() => [] as CardHistoryEntry[]),
        [] as CardHistoryEntry[],
        () => props.cardId,
    )

    // A board account id is what an entry carries, and the board already knows
    // the name behind it — an agent's account included, which is the whole
    // point of asking who. An id nobody knows is drawn as the app itself: on a
    // desktop every write a person makes lands under the one account there is.
    const who = (userId: string): string => {
        const user = userId ? boardUsers()[userId] : undefined
        if (user) {
            return user.username
        }
        return intl.formatMessage({id: 'CardHistory.someone', defaultMessage: 'You'})
    }

    const value = (text?: string): string => (
        text && text.trim() ? text : intl.formatMessage({id: 'CardHistory.empty', defaultMessage: 'empty'})
    )

    const said = (entry: CardHistoryEntry): string => {
        switch (entry.kind) {
        case 'created':
            return intl.formatMessage({id: 'CardHistory.created', defaultMessage: 'made the card'})
        case 'title':
            return intl.formatMessage(
                {id: 'CardHistory.title', defaultMessage: 'renamed it: “{to}”'},
                {to: value(entry.to)},
            )
        case 'property':
            return intl.formatMessage(
                {id: 'CardHistory.property', defaultMessage: '“{property}”: {from} → {to}'},
                {property: entry.property || '', from: value(entry.from), to: value(entry.to)},
            )
        case 'comment':
            return intl.formatMessage({id: 'CardHistory.comment', defaultMessage: 'wrote a comment'})
        default:
            return intl.formatMessage({id: 'CardHistory.deleted', defaultMessage: 'deleted it'})
        }
    }

    return (
        <div class='CardHistory'>
            <div class='CardHistory__header'>
                <span class='CardHistory__title'>
                    {intl.formatMessage({id: 'CardHistory.title-panel', defaultMessage: 'History'})}
                </span>
                <IconButton
                    onClick={props.onClose}
                    icon={<CompassIcon icon='close'/>}
                    title={intl.formatMessage({id: 'CardHistory.close', defaultMessage: 'Close the panel'})}
                    size='small'
                />
            </div>

            <Show
                when={entries().length > 0}
                fallback={
                    <div class='CardHistory__empty'>
                        {intl.formatMessage({id: 'CardHistory.none', defaultMessage: 'Nothing has happened on this card yet.'})}
                    </div>
                }
            >
                <div class='CardHistory__list'>
                    <For each={entries()}>
                        {(entry) => (
                            <div class='CardHistory__entry'>
                                <div class='CardHistory__line'>
                                    <span class='CardHistory__who'>{who(entry.userId)}</span>
                                    <span class='CardHistory__what'>{said(entry)}</span>
                                </div>
                                <div class='CardHistory__when'>{Utils.displayDateTime(new Date(entry.at), intl)}</div>

                                {/* The comment itself, because "wrote a
                                    comment" is not what somebody came to read.
                                    Trimmed by the panel's own width rather than
                                    by us: a long one is still the thing that
                                    happened. */}
                                <Show when={entry.kind === 'comment' && entry.text}>
                                    <div class='CardHistory__text'>{entry.text}</div>
                                </Show>
                            </div>
                        )}
                    </For>
                </div>
            </Show>
        </div>
    )
}

export default CardHistory
