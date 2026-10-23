import {For, Show, createSignal} from 'solid-js'
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

import {registeredCardSidePanels} from '../extensions'

import BoardPermissionGate from './permissions/boardPermissionGate'

import CardComments from './cardDetail/cardComments'
import CardHistory from './cardDetail/cardHistory'

import CardDetail from './cardDetail/cardDetail'
import Dialog from './dialog'

import CardActionsMenu from './cardActionsMenu/cardActionsMenu'
import AddCommentTourStep from './onboardingTour/addComments/addComments'
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
    // Which panel stands beside the card: comments, history, or one a feature
    // registered (extensions.tsx).
    const [side, setSide] = createSignal('')
    const toggleSide = (which: string) => setSide(side() === which ? '' : which)
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

    // Comments are drawn in every install: an agent's result is one, and on a
    // desktop it used to be written and never shown.
    const commentsBtn = (): JSX.Element => (
        <Show when={Boolean(card()) && !isTemplate()}>
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

    // The panels features add, each with its own button.
    const featureBtns = (): JSX.Element => (
        <Show when={card() && !isTemplate()}>
            <For each={registeredCardSidePanels()}>
                {(panel) => (
                    <panel.Button
                        board={props.board}
                        card={card()!}
                        active={side() === panel.id}
                        toggle={() => toggleSide(panel.id)}
                    />
                )}
            </For>
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
                toolbar={<>{historyBtn()}{featureBtns()}{commentsBtn()}{attachBtn()}</>}
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
                            <For each={registeredCardSidePanels()}>
                                {(panel) => (
                                    <Show when={side() === panel.id && card()}>
                                        <panel.Panel
                                            card={card()!}
                                            onClose={() => setSide('')}
                                        />
                                    </Show>
                                )}
                            </For>
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
