import {Show, createSignal, onSettled} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {FormattedMessage, useIntl} from '../intl'

import {Board} from '../blocks/board'
import {BoardView} from '../blocks/boardView'
import {Card} from '../blocks/card'
import {sendFlashMessage} from '../components/flashMessages'
import mutator from '../mutator'
import octoClient from '../octoClient'
import {getCardAttachments} from '../store/attachments'
import {getCard} from '../store/cards'
import {getCardComments} from '../store/comments'
import {getTeamMode} from '../store/clientConfig'
import {getCardContents} from '../store/contents'
import {useAppSelector, useAppStore} from '../store/hooks'
import {Utils} from '../utils'
import CompassIcon from '../widgets/icons/compassIcon'
import Menu from '../widgets/menu'

import ConfirmationDialogBox, {ConfirmationDialogBoxProps} from '../components/confirmationDialogBox'

import Button from '../widgets/buttons/button'

import {AttachmentBlock, createAttachmentBlock} from '../blocks/attachmentBlock'
import {Block, createBlock} from '../blocks/block'
import {Permission} from '../constants'
import {useHasCurrentBoardPermissions} from '../hooks/permissions'

import BoardPermissionGate from './permissions/boardPermissionGate'

import CardDiffPanel, {isCardDiffAvailable} from './acp/cardDiff'
import CardComments from './cardDetail/cardComments'
import CardHistory from './cardDetail/cardHistory'
import {cardAgentState, refreshCardAgent} from './acp/cardAgentState'
import {onAgentEvent} from './acp/agentEvents'

import CardDetail from './cardDetail/cardDetail'
import Dialog from './dialog'

import CardActionsMenu from './cardActionsMenu/cardActionsMenu'
import AddCommentTourStep from './onboardingTour/addComments/addComments'
import HintTip from './onboardingHints/hintTip'
import {HINT_CHANGES} from './onboardingHints/hints'
import './cardDialog.scss'

type Props = {
    board: Board
    activeView: BoardView
    views: BoardView[]
    cards: Card[]
    cardId: string
    onClose: () => void
    showCard: (cardId?: string) => void
    readonly: boolean
}

const CardDialog = (props: Props): JSX.Element => {
    const card = useAppSelector((state) => getCard(props.cardId)(state))
    const contents = useAppSelector((state) => getCardContents(props.cardId)(state))
    const comments = useAppSelector((state) => getCardComments(props.cardId)(state))
    const attachments = useAppSelector((state) => getCardAttachments(props.cardId)(state))
    const teamMode = useAppSelector<boolean>(getTeamMode)
    const canCommentBoardCards = useHasCurrentBoardPermissions([Permission.CommentBoardCards])
    const intl = useIntl()
    const {actions} = useAppStore()
    const isTemplate = () => card() && card()!.fields.isTemplate

    const [showConfirmationDialogBox, setShowConfirmationDialogBox] = createSignal<boolean>(false)

    // What the panel beside the card is showing, and '' for no panel. The
    // comments are what an agent reported and what people said back
    // (docs/teamwork.md); the changes are what the work amounts to. Both are
    // about the card without being part of it, which is why neither is in its
    // body.
    const [side, setSide] = createSignal<'' | 'comments' | 'diff' | 'history'>('')
    const toggleSide = (which: 'comments' | 'diff' | 'history') => setSide(side() === which ? '' : which)
    const makeTemplateClicked = async () => {
        const currentCard = card()
        if (!currentCard) {
            Utils.assertFailure('card')
            return
        }

        await mutator.duplicateCard(
            props.cardId,
            props.board.id,
            currentCard.fields.isTemplate,
            intl.formatMessage({id: 'Mutator.new-template-from-card', defaultMessage: 'new template from card'}),
            true,
            {},
            async (newCardId) => {
                props.showCard(newCardId)
            },
            async () => {
                props.showCard(undefined)
            },
        )
    }
    const handleDeleteCard = async () => {
        const currentCard = card()
        if (!currentCard) {
            Utils.assertFailure()
            return
        }
        await mutator.deleteBlock(currentCard, 'delete card')
        props.onClose()
    }

    const confirmDialogProps: ConfirmationDialogBoxProps = {
        heading: intl.formatMessage({id: 'CardDialog.delete-confirmation-dialog-heading', defaultMessage: 'Confirm card delete!'}),
        confirmButtonText: intl.formatMessage({id: 'CardDialog.delete-confirmation-dialog-button-text', defaultMessage: 'Delete'}),
        onConfirm: handleDeleteCard,
        onClose: () => {
            setShowConfirmationDialogBox(false)
        },
    }

    const handleDeleteButtonOnClick = () => {
        // use may be renaming a card title
        // and accidently delete the card
        // so adding des
        if (card()?.title === '' && card()?.fields.contentOrder.length === 0) {
            handleDeleteCard()
            return
        }

        setShowConfirmationDialogBox(true)
    }

    const menu = () => (
        <CardActionsMenu
            cardId={props.cardId}
            boardId={props.board.id}
            onClickDelete={handleDeleteButtonOnClick}
        >
            <Show when={!isTemplate()}>
                <BoardPermissionGate permissions={[Permission.ManageBoardProperties]}>
                    <Menu.Text
                        id='makeTemplate'
                        icon={
                            <CompassIcon
                                icon='plus'
                            />}
                        name={intl.formatMessage({id: 'CardDialog.new-template-from-card', defaultMessage: 'New template from card'})}
                        onClick={makeTemplateClicked}
                    />
                </BoardPermissionGate>
            </Show>
        </CardActionsMenu>
    )

    const removeUploadingAttachment = (uploadingBlock: Block) => {
        uploadingBlock.deleteAt = 1
        const removeUploadingAttachmentBlock = createAttachmentBlock(uploadingBlock)
        actions.attachments.updateAttachments([removeUploadingAttachmentBlock])
    }

    const selectAttachment = (boardId: string) => {
        return new Promise<AttachmentBlock>(
            (resolve) => {
                Utils.selectLocalFile(async (attachment) => {
                    const uploadingBlock = createBlock()
                    uploadingBlock.title = attachment.name
                    uploadingBlock.fields.fileId = attachment.name
                    uploadingBlock.boardId = boardId
                    const currentCard = card()
                    if (currentCard) {
                        uploadingBlock.parentId = currentCard.id
                    }
                    const attachmentBlock = createAttachmentBlock(uploadingBlock)
                    attachmentBlock.isUploading = true
                    actions.attachments.updateAttachments([attachmentBlock])
                    sendFlashMessage({content: intl.formatMessage({id: 'AttachmentBlock.upload', defaultMessage: 'Attachment uploading.'}), severity: 'normal'})
                    const xhr = await octoClient.uploadAttachment(boardId, attachment)
                    if (xhr) {
                        xhr.upload.onprogress = (event) => {
                            const percent = Math.floor((event.loaded / event.total) * 100)
                            actions.attachments.updateUploadPrecent({
                                blockId: attachmentBlock.id,
                                uploadPercent: percent,
                            })
                        }

                        xhr.onload = () => {
                            if (xhr.status === 200 && xhr.readyState === 4) {
                                const json = JSON.parse(xhr.response)
                                const fileId = json.fileId
                                if (fileId) {
                                    removeUploadingAttachment(uploadingBlock)
                                    const block = createAttachmentBlock()
                                    block.fields.fileId = fileId || ''
                                    block.title = attachment.name
                                    sendFlashMessage({content: intl.formatMessage({id: 'AttachmentBlock.uploadSuccess', defaultMessage: 'Attachment uploaded successfull.'}), severity: 'normal'})
                                    resolve(block)
                                } else {
                                    removeUploadingAttachment(uploadingBlock)
                                    sendFlashMessage({content: intl.formatMessage({id: 'AttachmentBlock.failed', defaultMessage: 'Unable to upload the file. Attachment size limit reached.'}), severity: 'normal'})
                                }
                            }
                        }
                    }
                },
                '')
            },
        )
    }

    const addElement = async () => {
        const currentCard = card()
        if (!currentCard) {
            return
        }
        const block = await selectAttachment(props.board.id)
        block.parentId = currentCard.id
        block.boardId = currentCard.boardId
        const typeName = block.type
        const description = intl.formatMessage({id: 'AttachmentBlock.addElement', defaultMessage: 'add {type}'}, {type: typeName})
        await mutator.insertBlock(block.boardId, block, description)
    }

    const deleteBlock = async (block: Block) => {
        if (!card()) {
            return
        }
        const description = intl.formatMessage({id: 'AttachmentBlock.DeleteAction', defaultMessage: 'delete'})
        await mutator.deleteBlock(block, description)
        sendFlashMessage({content: intl.formatMessage({id: 'AttachmentBlock.delete', defaultMessage: 'Attachment Deleted Successfully.'}), severity: 'normal'})
    }

    const attachBtn = (): JSX.Element => {
        return (
            <BoardPermissionGate permissions={[Permission.ManageBoardCards]}>
                <Button
                    icon={<CompassIcon icon='paperclip'/>}
                    class='cardFollowBtn cardFollowBtn--attach'
                    emphasis='gray'
                    size='medium'
                    onClick={addElement}
                >
                    {intl.formatMessage({id: 'CardDetail.Attach', defaultMessage: 'Attach'})}
                </Button>
            </BoardPermissionGate>
        )
    }

    // What the card knows about its agent: the branch the work is on, and how
    // much that work amounts to — which is what the changes button is drawn on.
    const agentState = cardAgentState(props.cardId)
    onSettled(() => {
        if (!isCardDiffAvailable()) {
            return undefined
        }
        refreshCardAgent(props.cardId)

        // The work moves while the dialog is open — an agent commits, a stage
        // ends — and a session event is the cheapest thing that means "look
        // again".
        return onAgentEvent('acp:session', () => refreshCardAgent(props.cardId))
    })

    // Comments are a conversation between people, so they are drawn where there
    // is a second person to have one with: an install of one person has them
    // switched off entirely, and everything an agent writes goes on being
    // written (docs/teamwork.md).
    const commentsBtn = (): JSX.Element => (
        <Show when={teamMode() && Boolean(card()) && !isTemplate()}>
            <Button
                icon={<CompassIcon icon='message-text-outline'/>}
                class='cardFollowBtn cardFollowBtn--attach cardFollowBtn--comments'
                emphasis='gray'
                size='medium'
                active={side() === 'comments'}
                onClick={() => toggleSide('comments')}
            >
                {intl.formatMessage({id: 'CardComments.title', defaultMessage: 'Comments'})}
            </Button>

            {/* On the button, not in the panel: the panel is closed until
                somebody opens it, and a tip inside it held the tour at a step
                nobody could see. */}
            <span class='cardDialog__tourAnchor'>
                <AddCommentTourStep/>
            </span>
        </Show>
    )

    // Drawn only where there is something to read. What decides is the count
    // the Go side keeps as the work moves (GetCardAgent's `changes`), not a
    // live look: on «На ревью» — the stage this is for — the card's working
    // copy has already been folded away, and asking git per card is a process
    // per card. A card nobody has worked on gets no button rather than a button
    // that opens «ничего не изменено».
    const hasChanges = () => {
        const changes = agentState().changes
        return Boolean(changes && (changes.files > 0 || changes.dirty > 0))
    }
    const diffBtn = (): JSX.Element => (
        <Show when={isCardDiffAvailable() && Boolean(card()) && !isTemplate() && hasChanges()}>
            <Button
                icon={<CompassIcon icon='file-patch-outline'/>}
                class='cardFollowBtn cardFollowBtn--attach'
                emphasis='gray'
                size='medium'
                active={side() === 'diff'}
                onClick={() => toggleSide('diff')}
            >
                {intl.formatMessage({id: 'CardDiff.title', defaultMessage: 'Changes'})}
            </Button>
            <span class='cardDialog__tourAnchor'>
                <HintTip
                    id={HINT_CHANGES}
                    wanted={true}
                    class='HintTip--button'
                    placement='bottom-end'
                    title={intl.formatMessage({id: 'Hint.changes.title', defaultMessage: 'What the agent changed'})}
                    body={intl.formatMessage({id: 'Hint.changes.body', defaultMessage: 'The changes the agent made in the folder, read from git: what is committed on the card’s branch and what is not committed yet.'})}
                />
            </span>
        </Show>
    )

    // Every card has one, because every write the board took left a row: even
    // a card nobody has touched since it was made says who made it and when.
    const historyBtn = (): JSX.Element => (
        <Show when={Boolean(card()) && !isTemplate()}>
            <Button
                icon={<CompassIcon icon='clock-outline'/>}
                class='cardFollowBtn cardFollowBtn--attach'
                emphasis='gray'
                size='medium'
                active={side() === 'history'}
                onClick={() => toggleSide('history')}
            >
                {intl.formatMessage({id: 'CardHistory.title-panel', defaultMessage: 'History'})}
            </Button>
        </Show>
    )

    return (
        <>
            <Dialog
                title={<div/>}
                class='cardDialog'
                onClose={props.onClose}
                toolsMenu={!props.readonly && menu()}
                toolbar={<>{historyBtn()}{diffBtn()}{commentsBtn()}{attachBtn()}</>}
            >
                <Show when={isTemplate()}>
                    <div class='banner'>
                        <FormattedMessage
                            id='CardDialog.editing-template'
                            defaultMessage="You're editing a template."
                        />
                    </div>
                </Show>

                {/* Two panels: the card on one side, what is said about it on
                    the other. The card scrolls on its own so the panel keeps
                    the dialog's full height rather than travelling with the
                    card's own content. */}
                <div class='cardDialog__body'>
                    <div class='cardDialog__main'>
                        <Show
                            when={card()}
                            fallback={
                                <div class='banner error'>
                                    <FormattedMessage
                                        id='CardDialog.nocard'
                                        defaultMessage="This card doesn't exist or is inaccessible."
                                    />
                                </div>
                            }
                        >
                            <CardDetail
                                board={props.board}
                                activeView={props.activeView}
                                views={props.views}
                                cards={props.cards}
                                card={card()!}
                                contents={contents()}
                                attachments={attachments()}
                                readonly={props.readonly}
                                onClose={props.onClose}
                                onDelete={deleteBlock}
                                addAttachment={addElement}
                            />
                        </Show>
                    </div>

                    <Show when={side() !== ''}>
                        <div class='cardDialog__side'>
                            <Show when={side() === 'comments'}>
                                <CardComments
                                    cardId={props.cardId}
                                    boardId={props.board.id}
                                    comments={comments()}
                                    readonly={props.readonly || !canCommentBoardCards()}
                                    onClose={() => setSide('')}
                                />
                            </Show>
                            <Show when={side() === 'diff'}>
                                <CardDiffPanel
                                    cardId={props.cardId}
                                    onClose={() => setSide('')}
                                />
                            </Show>
                            <Show when={side() === 'history'}>
                                <CardHistory
                                    cardId={props.cardId}
                                    onClose={() => setSide('')}
                                />
                            </Show>
                        </div>
                    </Show>
                </div>
            </Dialog>

            <Show when={showConfirmationDialogBox()}>
                <ConfirmationDialogBox dialogBox={confirmDialogProps}/>
            </Show>
        </>
    )
}

export default CardDialog
