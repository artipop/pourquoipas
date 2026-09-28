import {Show, createSignal} from 'solid-js'

import {useIntl, IntlShape} from '../../intl'

import {CsvExporter} from '../../csvExporter'
import {Archiver} from '../../archiver'
import {Board} from '../../blocks/board'
import {BoardView} from '../../blocks/boardView'
import {Card} from '../../blocks/card'
import IconButton from '../../widgets/buttons/iconButton'
import OptionsIcon from '../../widgets/icons/options'
import Menu from '../../widgets/menu'
import MenuWrapper from '../../widgets/menuWrapper'
import {Utils} from '../../utils'

import ModalWrapper from '../modalWrapper'
import {sendFlashMessage} from '../flashMessages'
import WorkdirsDialog from '../acp/workdirsDialog'
import BoardPromptsDialog, {isBoardPromptsAvailable} from '../acp/boardPromptsDialog'
import DeployTargetsDialog from '../acp/deployTargetsDialog'
import AgentsDialog, {isAgentsDialogAvailable} from '../acp/agentsDialog'
import BoardSetupWizard from '../acp/boardSetupWizard'
import {createSetupPlan, isBoardSetupAvailable} from '../acp/boardSetup'
import SourcesDialog, {isSourcesAvailable} from '../acp/sourcesDialog'
import InboxSortingDialog, {isInboxSortingAvailable} from '../acp/inboxSortingDialog'
import TemplateEditor from '../acp/templateEditor'
import {isSaveAsTemplateAvailable, saveBoardAsTemplate} from '../acp/saveAsTemplate'
import {isInboxBoard} from '../acp/inboxView'
import {goError} from '../../goMessages'

// What this menu holds is what is true of *this board*, and only that. The
// registries it used to open — agents, where to deploy, how to reach the
// network — belong to the machine and are in the sidebar's settings, where they
// can be reached with no board open at all. Talking a task over with an agent
// is a way of making cards and lives on the "New" button. Setting the board up
// is offered by the board's own title while it is unanswered, and by the screen
// below afterwards.
//
// The inbox board is the one board about where cards come from, so its menu
// offers the sources dialog and the sorting that carries their cards on — and
// nothing else: it runs no routes, and exporting it or saving it as a template
// is not what an inbox is for. Every other board's
// menu is the board's, and sources are not in it: the inbox always exists (the
// app makes it at launch), so there is no board that needs a second door.

// the save-as-template item is parked rather than deleted, and this is the one
// line that parks it. A board set up the way somebody wants it is still the
// best template there is, and everything behind the door — saveAsTemplate.ts,
// templateEditor.tsx, their tests — is untouched and still reached by the
// pencil in the template picker. What it has not got is a reason to stand in
// the menu of every board: making a template is a rare and deliberate act, and
// here it sat one slot away from the things a board is set up with. Flip it
// back to open the door.
const OFFER_SAVE_AS_TEMPLATE = false

type Props = {
    board: Board
    activeView: BoardView
    cards: Card[]
}

function onExportCsvTrigger(board: Board, activeView: BoardView, cards: Card[], intl: IntlShape) {
    try {
        CsvExporter.exportTableCsv(board, activeView, cards, intl)
        const exportCompleteMessage = intl.formatMessage({
            id: 'ViewHeader.export-complete',
            defaultMessage: 'Export complete!',
        })
        sendFlashMessage({content: exportCompleteMessage, severity: 'normal'})
    } catch (e) {
        Utils.logError(`ExportCSV ERROR: ${e}`)
        const exportFailedMessage = intl.formatMessage({
            id: 'ViewHeader.export-failed',
            defaultMessage: 'Export failed!',
        })
        sendFlashMessage({content: exportFailedMessage, severity: 'high'})
    }
}

const ViewHeaderActionsMenu = (props: Props) => {
    const intl = useIntl()
    const [showSources, setShowSources] = createSignal(false)
    const [showSorting, setShowSorting] = createSignal(false)
    const [showWorkdirs, setShowWorkdirs] = createSignal(false)
    const [showDeploys, setShowDeploys] = createSignal(false)
    const [showAgents, setShowAgents] = createSignal(false)
    const [showPrompts, setShowPrompts] = createSignal(false)
    const [showSetup, setShowSetup] = createSignal(false)

    // Which of the setup questions this board asks is the board's own answer,
    // and the menu follows it: a board of shopping lists is offered folders and
    // nothing else, a board that deploys is offered somewhere to deploy to.
    // They were folds of the automation dialog, where setting up where an
    // agent works was a question about columns and routes — which it is not —
    // and a fold under a canvas is a place nobody opens.
    const [plan, refreshPlan] = createSetupPlan(() => props.board)
    const asks = (kind: string) => (plan()?.steps || []).some((s) => s.kind === kind)

    // The template a board was saved as, held so the editor opens on it: it is
    // a board of its own and the page has not navigated to it.
    const [template, setTemplate] = createSignal<Board | null>(null)

    const saveAsTemplate = async () => {
        try {
            setTemplate(await saveBoardAsTemplate(props.board, intl))
        } catch (e) {
            sendFlashMessage({content: goError(intl, e), severity: 'high'})
        }
    }

    const onInbox = () => isInboxBoard(props.board)
    const offerSources = () => isSourcesAvailable() && onInbox()
    const offerSorting = () => isInboxSortingAvailable() && onInbox()

    return (
        <ModalWrapper>
            {/* On the inbox with no sources to offer, the menu would be an empty
                one: a button that opens nothing is worse than no button. */}
            <Show when={!onInbox() || offerSources() || offerSorting()}>
                <MenuWrapper
                    label={intl.formatMessage({id: 'ViewHeader.view-header-menu', defaultMessage: 'View header menu'})}
                    menu={
                        <Menu position='left'>
                            <Show when={!onInbox()}>
                                {/* What the board's agents are told first. Its
                                    own item for the same reason the folders
                                    have one: it was a fold at the bottom of the
                                    route canvas, and a fold under a canvas is
                                    somewhere nobody scrolls to. Offered where
                                    the board is one agents work on. */}
                                <Show when={isBoardPromptsAvailable() && asks('agent')}>
                                    <Menu.Text
                                        id='boardPrompts'
                                        name={intl.formatMessage({id: 'ViewHeader.board-prompts', defaultMessage: 'The board’s system prompt…'})}
                                        onClick={() => setShowPrompts(true)}
                                    />
                                </Show>

                                {/* No setup step asks for a folder any more — a card
                                    does — so the folders are offered wherever
                                    agents work, which is where a card can name
                                    one. */}
                                <Show when={asks('agent')}>
                                    <Menu.Text
                                        id='workdirs'
                                        name={intl.formatMessage({id: 'ViewHeader.workdirs', defaultMessage: 'Folders…'})}
                                        onClick={() => setShowWorkdirs(true)}
                                    />
                                </Show>

                                {/* Who picks the cards up. The registry is the
                                    machine's, like the deploy hosts under it,
                                    and it is offered here for the same reason:
                                    it means nothing on a board no agent works.
                                    In the order the work needs them — the
                                    folder, who works in it, where it goes. */}
                                <Show when={isAgentsDialogAvailable() && asks('agent')}>
                                    <Menu.Text
                                        id='agents'
                                        name={intl.formatMessage({id: 'ViewHeader.agents', defaultMessage: 'Agents…'})}
                                        onClick={() => setShowAgents(true)}
                                    />
                                </Show>

                                <Show when={asks('deploy')}>
                                    <Menu.Text
                                        id='deploys'
                                        name={intl.formatMessage({id: 'ViewHeader.deploys', defaultMessage: 'Where to deploy…'})}
                                        onClick={() => setShowDeploys(true)}
                                    />
                                </Show>

                                <Show when={isBoardSetupAvailable() && (plan()?.steps.length || 0) > 0}>
                                    <Menu.Text
                                        id='setup'
                                        name={intl.formatMessage({id: 'ViewHeader.setup', defaultMessage: 'Walk the setup again…'})}
                                        onClick={() => setShowSetup(true)}
                                    />
                                </Show>

                                <Show when={OFFER_SAVE_AS_TEMPLATE && isSaveAsTemplateAvailable() && !props.board.isTemplate}>
                                    <Menu.Text
                                        id='saveAsTemplate'
                                        name={intl.formatMessage({id: 'ViewHeader.save-as-template', defaultMessage: 'Save as a template…'})}
                                        onClick={saveAsTemplate}
                                    />
                                </Show>
                            </Show>

                            <Show when={offerSources()}>
                                <Menu.Text
                                    id='sources'
                                    name={intl.formatMessage({id: 'ViewHeader.sources', defaultMessage: 'Sources…'})}
                                    onClick={() => setShowSources(true)}
                                />
                            </Show>

                            {/* Where what the sources bring goes next — the
                                other half of the inbox, so beside them. */}
                            <Show when={offerSorting()}>
                                <Menu.Text
                                    id='inboxSorting'
                                    name={intl.formatMessage({id: 'ViewHeader.inbox-sorting', defaultMessage: 'Sorting the inbox…'})}
                                    onClick={() => setShowSorting(true)}
                                />
                            </Show>

                            {/* Carrying the board out is not a setting of it,
                                and it used to open this menu: what a person
                                comes here for — what the board runs, what it
                                was asked at setup — stood under two exports
                                nobody uses twice. Last, behind a line. There is
                                no import beside them on purpose: an archive
                                brings boards, plural, so it is taken in where
                                boards are (the import-and-export section of
                                the settings). */}
                            <Show when={!onInbox()}>
                                <Menu.Separator/>
                                {/* One word on the menu, the two shapes behind
                                    it: even folded away and last, two export
                                    rows were still two rows about one act. The
                                    sidebar's board menu keeps the long name
                                    (ViewHeader.export-board-archive), because
                                    there the row stands alone and the short
                                    name without the word "export" over it says
                                    nothing. */}
                                <Menu.SubMenu
                                    id='export'
                                    name={intl.formatMessage({id: 'ViewHeader.export', defaultMessage: 'Export'})}
                                    position='auto'
                                >
                                    <Menu.Text
                                        id='exportCsv'
                                        name={intl.formatMessage({id: 'ViewHeader.export-csv', defaultMessage: 'To CSV'})}
                                        onClick={() => onExportCsvTrigger(props.board, props.activeView, props.cards, intl)}
                                    />
                                    <Menu.Text
                                        id='exportBoardArchive'
                                        name={intl.formatMessage({id: 'ViewHeader.export-archive', defaultMessage: 'Board archive'})}
                                        onClick={() => Archiver.exportBoardArchive(props.board)}
                                    />
                                </Menu.SubMenu>
                            </Show>
                        </Menu>
                    }
                >
                    <IconButton icon={<OptionsIcon/>}/>
                </MenuWrapper>
            </Show>
            <Show when={showWorkdirs()}>
                <WorkdirsDialog
                    board={props.board}
                    onClose={() => {
                        setShowWorkdirs(false)
                        refreshPlan()
                    }}
                />
            </Show>
            <Show when={showPrompts()}>
                <BoardPromptsDialog
                    board={props.board}
                    onClose={() => setShowPrompts(false)}
                />
            </Show>
            <Show when={showAgents()}>
                <AgentsDialog onClose={() => {
                    setShowAgents(false)
                    refreshPlan()
                }}/>
            </Show>
            <Show when={showDeploys()}>
                <DeployTargetsDialog
                    board={props.board}
                    onChange={refreshPlan}
                    onClose={() => setShowDeploys(false)}
                />
            </Show>
            <Show when={showSetup()}>
                <BoardSetupWizard
                    board={props.board}
                    onClose={() => {
                        setShowSetup(false)
                        refreshPlan()
                    }}
                />
            </Show>
            <Show when={showSources()}>
                <SourcesDialog
                    onClose={() => setShowSources(false)}
                />
            </Show>
            <Show when={showSorting()}>
                <InboxSortingDialog
                    onClose={() => setShowSorting(false)}
                />
            </Show>
            <Show when={template()}>
                {(saved) => (
                    <TemplateEditor
                        board={saved()}
                        onClose={() => setTemplate(null)}
                    />
                )}
            </Show>
        </ModalWrapper>
    )
}

export default ViewHeaderActionsMenu
