import {useIntl} from '../../intl'

import {CommentBlock} from '../../blocks/commentBlock'
import CompassIcon from '../../widgets/icons/compassIcon'

import CommentsList from './commentsList'

import './cardComments.scss'

// Comments beside the card rather than in it: the body is what a person wrote.
// Shares the terminal's panel — same question, two readers (docs/teamwork.md).

type Props = {
    cardId: string
    boardId: string
    comments: readonly CommentBlock[]
    readonly: boolean
    onClose: () => void
}

const CardComments = (props: Props) => {
    const intl = useIntl()

    return (
        <div class='CardComments'>
            <div class='CardComments__head'>
                <span class='CardComments__title'>
                    {intl.formatMessage({id: 'CardComments.title', defaultMessage: 'Comments'})}
                </span>
                <div class='CardComments__actions'>
                    <button
                        type='button'
                        class='CardComments__button'
                        title={intl.formatMessage({id: 'CardComments.close', defaultMessage: 'Close the panel'})}
                        aria-label={intl.formatMessage({id: 'CardComments.close', defaultMessage: 'Close the panel'})}
                        onClick={() => props.onClose()}
                    >
                        <CompassIcon icon='close'/>
                    </button>
                </div>
            </div>

            <div class='CardComments__body'>
                <CommentsList
                    comments={props.comments}
                    boardId={props.boardId}
                    cardId={props.cardId}
                    readonly={props.readonly}
                />
            </div>
        </div>
    )
}

export default CardComments
