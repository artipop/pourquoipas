// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {Show, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import {Board} from '../../blocks/board'
import mutator from '../../mutator'
import Button from '../../widgets/buttons/button'
import Dialog from '../dialog'

import {goError} from '../../goMessages'

import {boardColumns, columnProperty} from './automation'

import './templateEditor.scss'

// A template is a board that has not been made yet: its columns, what happens
// in each of them, the routes cards take across it.
//
// What is edited here is only what the template list shows — its name, its icon and what it is for. The automation
// is **shown and not edited**: a template is made by building the board and
// saving it (the save-as-template item), so the routes it carries are already the
// ones that worked, and the way to change them is to change the board and save
// it again. It was the whole route canvas in a scrolling dialog, which put a
// graph editor between somebody and the two fields they came to fill in — and
// gave the same routes two places to be edited, one of them the copy nobody was
// looking at.
//
// It writes into the template board's own properties, which is where Go reads
// them from when a board is made from it (internal/acp/boardseed.go).

type Props = {
    board: Board
    onClose: () => void
    onSaved?: () => void
}

const TemplateEditor = (props: Props) => {
    const intl = useIntl()

    const [title, setTitle] = createSignal(props.board.title)
    const [icon, setIcon] = createSignal(props.board.icon || '')
    const [description, setDescription] = createSignal(props.board.description || '')
    const [error, setError] = createSignal('')

    const columns = () => boardColumns(props.board, columnProperty(props.board)?.name)

    // The columns a board made from this template arrives with.
    const columnLine = () => columns().map((c) => c.name)

    const save = async () => {
        setError('')
        try {
            const next: Board = {
                ...props.board,
                title: title().trim() || props.board.title,
                icon: icon(),
                description: description(),
            }
            await mutator.updateBoard(next, props.board, 'edit template')
            props.onSaved?.()
            props.onClose()
        } catch (e) {
            setError(goError(intl, e))
        }
    }

    return (
        <Dialog
            class='TemplateEditor'
            title={<span>{intl.formatMessage({id: 'TemplateEditor.title', defaultMessage: 'Template'})}</span>}
            subtitle={<span>{intl.formatMessage({id: 'TemplateEditor.subtitle', defaultMessage: 'A board made from this template arrives with the columns below. Its name and what it is for are set here.'})}</span>}
            onClose={props.onClose}
        >
            <div class='TemplateEditor__content'>
                <div class='TemplateEditor__meta'>
                    <label class='TemplateEditor__icon'>
                        {intl.formatMessage({id: 'TemplateEditor.icon', defaultMessage: 'Icon'})}
                        <input
                            value={icon()}
                            maxLength={2}
                            onInput={(e) => setIcon(e.currentTarget.value)}
                        />
                    </label>
                    <label class='TemplateEditor__name'>
                        {intl.formatMessage({id: 'TemplateEditor.name', defaultMessage: 'Name'})}
                        <input
                            value={title()}
                            onInput={(e) => setTitle(e.currentTarget.value)}
                        />
                    </label>
                    <label class='TemplateEditor__description'>
                        {intl.formatMessage({id: 'TemplateEditor.description', defaultMessage: 'What it is for'})}
                        <input
                            value={description()}
                            placeholder={intl.formatMessage({id: 'TemplateEditor.description-placeholder', defaultMessage: 'One line, shown beside the template'})}
                            onInput={(e) => setDescription(e.currentTarget.value)}
                        />
                    </label>
                </div>

                {/* What the template carries, in the words the board uses. It is
                    read here and changed on a board. */}
                <div class='TemplateEditor__carries'>
                    <div class='TemplateEditor__sectionTitle'>
                        {intl.formatMessage({id: 'TemplateEditor.carries', defaultMessage: 'What the template carries'})}
                    </div>
                    <div class='TemplateEditor__carriesRow'>
                        <span class='TemplateEditor__carriesLabel'>
                            {intl.formatMessage({id: 'TemplateEditor.columns', defaultMessage: 'Columns'})}
                        </span>
                        <span>{columnLine().join(' · ') || '—'}</span>
                    </div>
                    <div class='TemplateEditor__hint'>
                        {intl.formatMessage({id: 'TemplateEditor.carries-hint', defaultMessage: 'Columns are changed on the board itself, and come back here when the board is saved as a template again.'})}
                    </div>
                </div>

                <div class='TemplateEditor__actions'>
                    <Button
                        emphasis='primary'
                        onClick={save}
                    >
                        {intl.formatMessage({id: 'TemplateEditor.save', defaultMessage: 'Save template'})}
                    </Button>
                    <Button onClick={props.onClose}>
                        {intl.formatMessage({id: 'TemplateEditor.cancel', defaultMessage: 'Cancel'})}
                    </Button>
                </div>

                <Show when={error()}>
                    <div class='TemplateEditor__error'>{error()}</div>
                </Show>
            </div>
        </Dialog>
    )
}

export default TemplateEditor
