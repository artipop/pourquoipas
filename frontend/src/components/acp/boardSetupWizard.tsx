// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import {For, Show, createEffect, createSignal, onSettled} from 'solid-js'

import {useIntl} from '../../intl'

import {Board} from '../../blocks/board'
import {titleTaken} from '../../boardTitle'
import {listBoards} from '../../bindings/boards'
import {createLoader} from '../../hooks/loader'
import mutator from '../../mutator'
import Button from '../../widgets/buttons/button'
import Dialog from '../dialog'

import {sendFlashMessage} from '../flashMessages'

import {goApp} from '../../goBridge'

import AgentQuickAdd from './agentQuickAdd'
import {checkSetupAnswer, createSetupPlan, recordSetupStep, SetupStep, SetupStepKind, stepRequires} from './boardSetup'
import {Workdir, syncWorkdirsToBoard, useWorkdirHere} from './workdirSync'

import './boardSetupWizard.scss'

// A board made from a template arrives knowing how the work is organised — its
// columns, its routes, the fields a card picks a folder and an agent with.
// What it cannot know is the machine: which agent runs, in which folder, where
// it deploys, what it tests with. That lives in the desktop registries.
//
// Which questions this board has is not decided here: Go resolves them into a
// plan (internal/acp/setup.go) out of what the board asks for, what its
// automation implies and what this machine already has. This walks that plan
// and knows how to ask each kind of question — nothing more.

type Registry = {
    agents: Array<{name: string, kind: string}>
    workdirs: Array<{name: string, path: string}>
}

// readRegistry is what the steps show back: the names already registered. The
// plan says whether a question is answered; this says what the answer was.
export async function readRegistry(boardId: string): Promise<Registry | null> {
    const bindings = goApp()
    if (!bindings?.ListAgentWorkdirs || !bindings.ListAgents) {
        return null
    }
    const [workdirs, agents] = await Promise.all([bindings.ListAgentWorkdirs(boardId), bindings.ListAgents()])
    return {workdirs: JSON.parse(workdirs) || [], agents: JSON.parse(agents) || []}
}

type Props = {
    board: Board
    onClose: () => void
}

const STEP_NAME: SetupStepKind = 'name'
const STEP_WORKDIR: SetupStepKind = 'project'
const STEP_AGENT: SetupStepKind = 'agent'
const STEP_DEPLOY: SetupStepKind = 'deploy'
const STEP_DONE: SetupStepKind = 'done'

const BoardSetupWizard = (props: Props) => {
    const intl = useIntl()
    const bindings = goApp()

    const [plan, refreshPlan] = createSetupPlan(() => props.board)
    const [step, setStep] = createSignal<SetupStepKind>(STEP_WORKDIR)
    const [registry, setRegistry] = createSignal<Registry>({agents: [], workdirs: []})
    const [error, setError] = createSignal('')
    const [busy, setBusy] = createSignal(false)

    // The steps this board asks for, in the order it asks for them. A plan that
    // has not arrived yet is a dialog with one step, which is what it should
    // look like while it is loading: nothing to answer.
    const steps = (): SetupStep[] => plan()?.steps || []

    // Where the wizard goes on from a step: the next one this board asks for.
    const after = (current: SetupStepKind): SetupStepKind => {
        const order = steps()
        const next = order[order.findIndex((s) => s.kind === current) + 1]
        return next ? next.kind : STEP_DONE
    }

    // The step being shown, if this board asks for it at all — its own sentence
    // hangs off it.
    const stepAt = (kind: SetupStepKind) => steps().find((s) => s.kind === kind)

    // Skipping is the one answer no registry can be read for later, so it is
    // recorded; the plan reads it back as the step's status.
    const skip = (kind: SetupStepKind) => {
        recordSetupStep(props.board.id, kind, 'skipped').
            then(() => refreshPlan()).
            catch(() => undefined)
        setStep(after(kind))
    }

    // Step 1: a folder.
    const [workdirPath, setWorkdirPath] = createSignal('')
    const [workdirName, setWorkdirName] = createSignal('')

    // A folder already in the registry, picked again — offered rather than
    // refused (see the folders panel).
    const [taken, setTaken] = createSignal<Workdir | null>(null)
    const basename = (path: string) => path.split('/').filter(Boolean).pop() || ''

    // Step 3: an agent registered on this machine. Which agent works a card is
    // the card's own «Кто занимается», so the step asks for nothing beyond one
    // existing at all.
    const [addingAgent, setAddingAgent] = createSignal(false)

    // Step 1: what this board is called. It arrives named after the template it
    // was made from, so the field opens on that and on the whole of it
    // selected — the answer is usually a replacement, not an edit.
    const [boardName, setBoardName] = createSignal(props.board.title)

    // The other boards' names, read from Go rather than from the store: this
    // dialog talks to Go for everything else it knows, and a store dependency
    // here would need a provider around it in every test that renders it.
    const [boards] = createLoader(() => listBoards().catch(() => []), [] as Array<{id: string, title: string}>)
    const nameTaken = () => titleTaken(boards(), boardName(), props.board.id)

    // And what it is for, asked with the name: a sentence here is what the
    // inbox sorter reads to decide which cards belong on this board
    // (internal/inboxsort), and the moment the board is made is the one moment
    // somebody knows the answer without having to think back.
    const [boardDescription, setBoardDescription] = createSignal(props.board.description || '')

    const renameBoard = () => run(async () => {
        const wanted = boardName().trim()
        if (wanted && wanted !== props.board.title) {
            await mutator.changeBoardTitle(props.board.id, props.board.title, wanted)
        }
        const described = boardDescription().trim()
        if (described !== (props.board.description || '').trim()) {
            await mutator.changeBoardDescription(props.board.id, props.board.id, props.board.description, described)
        }
    }, STEP_NAME)

    // Step 3: a Dokku host.
    const [deploy, setDeploy] = createSignal({name: '', sshHost: '', sshUser: '', sshKey: '', baseDomain: ''})

    const refresh = async () => {
        try {
            const loaded = await readRegistry(props.board.id)
            if (loaded) {
                setRegistry(loaded)
            }
        } catch (e) {
            setError(String(e))
        }
    }

    onSettled(() => {
        refresh()
    })

    // The wizard opens on the first question this board still has. Walking
    // somebody through what they have already answered is how a setup dialog
    // earns being clicked through without reading. Only when the plan first
    // arrives, though: it is refetched after every answer, and moving the
    // person then would take the step out of their hands.
    let opened = false
    createEffect(steps, (list) => {
        if (opened || list.length === 0) {
            return
        }
        opened = true
        setStep((list.find((s) => s.status === 'pending') || list[0]).kind)
    })

    // Every step does its work through the same registry calls the dialogs use,
    // and shows what Go says when it refuses. The step is recorded as answered
    // for *this board* — the registries are the machine's and say nothing about
    // whether this board has been through the questions.
    const run = async (work: () => Promise<void>, answering: SetupStepKind) => {
        setError('')
        setBusy(true)
        try {
            await work()
            await recordSetupStep(props.board.id, answering, 'done')
            await refresh()
            refreshPlan()
            setStep(after(answering))
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    // Passing a step the machine can already answer — there is a folder, an
    // agent — is answering it too, and has to be recorded as one.
    const pass = (answering: SetupStepKind) => run(async () => {}, answering)

    const pickWorkdir = async () => {
        if (!bindings?.PickDirectory) {
            return
        }
        setError('')
        try {
            const picked = await bindings.PickDirectory(intl.formatMessage({id: 'BoardSetup.pick-folder', defaultMessage: 'Choose a folder'}))
            if (picked) {
                // The name follows the folder while nobody has typed one: it
                // was filled in from the last pick, so picking again must
                // refill it — a second choice with the first one's name is a
                // folder registered under the wrong word. A name somebody
                // actually typed is theirs and survives the change.
                const previous = basename(workdirPath())
                setWorkdirName((current) => (!current || current === previous ? basename(picked) : current))
                setWorkdirPath(picked)
                setTaken(JSON.parse(await bindings.FindAgentWorkdir?.(picked) || 'null'))
            }
        } catch (e) {
            setError(String(e))
        }
    }

    // The folder is already somebody's: using it here is one call and no new
    // registry entry, and the step is answered by it just the same.
    const useTakenWorkdir = () => run(async () => {
        const entry = taken()
        if (entry) {
            await useWorkdirHere(entry, props.board.id)
            setTaken(null)
        }
        await syncWorkdirsToBoard(props.board, JSON.parse(await bindings!.ListAgentWorkdirs(props.board.id)) || [])
    }, STEP_WORKDIR)

    const addWorkdir = () => run(async () => {
        // Asked before it is filed: a board that publishes a branch needs a
        // folder under git, and this is where that can still be answered.
        await checkSetupAnswer(props.board.id, STEP_WORKDIR, workdirPath())

        // Filed as a repository when that is what was asked for, so the answer
        // outlives the question: a folder that stops being one later says so
        // instead of quietly becoming an ordinary folder on a board whose
        // every route waits for a branch.
        const kind = stepRequires(stepAt(STEP_WORKDIR), 'git') ? 'git' : ''
        await bindings!.AddAgentWorkdir!(workdirName().trim(), workdirPath(), props.board.id, kind, false)

        // And onto the board, because the registry alone is not an answer: a
        // card names its folder with an option of the board's own field, so a
        // folder added here and nowhere else left the person who had just
        // answered this question with nothing to pick on the card.
        await syncWorkdirsToBoard(props.board, JSON.parse(await bindings!.ListAgentWorkdirs(props.board.id)) || [])
    }, STEP_WORKDIR)

    const addDeploy = () => run(async () => {
        await bindings!.AddDeployTarget!(JSON.stringify({
            name: deploy().name.trim(),
            sshHost: deploy().sshHost.trim(),
            sshUser: deploy().sshUser.trim(),
            sshKey: deploy().sshKey.trim(),
            baseDomain: deploy().baseDomain.trim(),
        }))
    }, STEP_DEPLOY)

    const finish = async () => {
        // Take the board's own columns and routes now, so what it can do is
        // visible without waiting for the first card to be moved.
        if (bindings?.SeedBoardAutomation) {
            try {
                await bindings.SeedBoardAutomation(props.board.id)
            } catch (e) {
                setError(String(e))
                return
            }
        }
        props.onClose()
    }

    const hasWorkdir = () => registry().workdirs.length > 0
    const hasAgent = () => registry().agents.length > 0

    // Whether the folder step has something to answer with: one already on this
    // board, one picked and named, or one somebody else registered and offered
    // for use here.
    const workdirAnswered = () => Boolean(taken()) || hasWorkdir() || Boolean(workdirPath() && workdirName().trim())

    const body = () => {
        switch (step()) {
        case STEP_NAME:
            return (
                <div class='BoardSetupWizard__step'>
                    <p>{intl.formatMessage({id: 'BoardSetup.name-why', defaultMessage: 'A board arrives called what its template is called. Name it after the work that will be on it — that is how you will find it in the sidebar, and how a card knows which board it is being carried to.'})}</p>
                    <label class='BoardSetupWizard__field'>
                        {intl.formatMessage({id: 'BoardSetup.name-label', defaultMessage: 'Board name'})}
                        <input
                            value={boardName()}
                            disabled={busy()}
                            onInput={(e) => setBoardName(e.currentTarget.value)}
                            onKeyDown={(e) => {
                                if (e.key === 'Enter' && boardName().trim() && !nameTaken()) {
                                    renameBoard()
                                }
                            }}
                        />
                    </label>
                    <Show when={nameTaken()}>
                        <p class='BoardSetupWizard__error'>
                            {intl.formatMessage({id: 'BoardSetup.name-taken', defaultMessage: 'Another board is already called that — boards are told apart by their names.'})}
                        </p>
                    </Show>
                    <label class='BoardSetupWizard__field'>
                        {intl.formatMessage({id: 'BoardSetup.description-label', defaultMessage: 'Description'})}
                        <textarea
                            class='BoardSetupWizard__description'
                            rows={3}
                            value={boardDescription()}
                            disabled={busy()}
                            placeholder={intl.formatMessage({id: 'BoardSetup.description-placeholder', defaultMessage: 'What cards belong on this board: topics, projects, places'})}
                            onInput={(e) => setBoardDescription(e.currentTarget.value)}
                        />
                    </label>
                    <p class='BoardSetupWizard__hint'>
                        {intl.formatMessage({id: 'BoardSetup.description-why', defaultMessage: 'The agent that sorts the inbox reads the description to decide which cards to move to this board. You can leave it empty and add it later in the inbox sorting settings.'})}
                    </p>
                </div>
            )
        case STEP_WORKDIR:
            return (
                <div class='BoardSetupWizard__step'>
                    <p>{intl.formatMessage({id: 'BoardSetup.folder-why', defaultMessage: 'An agent works in a folder on your machine. A card is matched to one by its folder field, which this fills in for you.'})}</p>
                    <Show when={stepRequires(stepAt(STEP_WORKDIR), 'git')}>
                        <p class='BoardSetupWizard__hint'>
                            {intl.formatMessage({id: 'BoardSetup.folder-git', defaultMessage: 'This board publishes a branch or waits for one, so its folder has to be a git repository. A board that does neither takes any folder.'})}
                        </p>
                    </Show>
                    <Show when={hasWorkdir()}>
                        <div class='BoardSetupWizard__known'>
                            {intl.formatMessage({id: 'BoardSetup.folder-known', defaultMessage: 'Already registered: {names}'}, {names: registry().workdirs.map((r) => r.name).join(', ')})}
                        </div>
                    </Show>

                    {/* What passing costs, said where it can be acted on: a
                        card with no folder can still be talked over — that
                        conversation opens in the board's drafts folder — but a
                        stage of a route has nowhere to work and waits. */}
                    <Show when={!workdirAnswered()}>
                        <p class='BoardSetupWizard__hint'>
                            {intl.formatMessage({id: 'BoardSetup.folder-skip', defaultMessage: 'This can be answered later. Without a folder a card can still be discussed with an agent — in the board’s drafts folder — but a card on a route will wait at the stage that works it.'})}
                        </p>
                    </Show>
                    <Button onClick={pickWorkdir}>
                        {intl.formatMessage({id: 'BoardSetup.choose-folder', defaultMessage: 'Choose a folder…'})}
                    </Button>
                    {/* Already added, on this board or another: an offer, not
                        a refusal — the person picked the folder they meant. */}
                    <Show when={taken()}>
                        {(entry) => (
                            <div class='BoardSetupWizard__known'>
                                {intl.formatMessage(
                                    {id: 'Workdirs.already-added', defaultMessage: 'This folder is already added as "{name}". Use it on this board too?'},
                                    {name: entry().name},
                                )}
                            </div>
                        )}
                    </Show>
                    <Show when={workdirPath() && !taken()}>
                        <span class='BoardSetupWizard__path'>{workdirPath()}</span>
                        <label>
                            {intl.formatMessage({id: 'BoardSetup.folder-name', defaultMessage: 'Name'})}
                            <input
                                value={workdirName()}
                                onInput={(e) => setWorkdirName(e.currentTarget.value)}
                            />
                        </label>
                    </Show>
                </div>
            )
        case STEP_AGENT:
            return (
                <div class='BoardSetupWizard__step'>
                    <p>{intl.formatMessage({id: 'BoardSetup.agent-why', defaultMessage: 'The agent that picks a card up. It has to be logged in already; here it is only given a name.'})}</p>

                    {/* Nothing is chosen here. The board does not keep a crew
                        any more — every registered agent may be assigned on any
                        board — so the step is answered by one existing, and who
                        works a given card is said on the card. */}
                    <Show when={hasAgent()}>
                        <div class='BoardSetupWizard__ask'>
                            {intl.formatMessage({id: 'BoardSetup.agent-registered', defaultMessage: 'Agents on this machine:'})}
                        </div>
                    </Show>

                    {/* The same two questions a card asks when it needs an
                        agent and has none, asked by the same component: a
                        second form here is how a kind ends up offered in one
                        place and not the other.

                        With nobody registered the form stands open — there is
                        nothing else on the step to do. Once somebody is, it
                        goes behind a link, because the step is then answered
                        and adding a second agent is an offer rather than the
                        question; the step used to show the names and no way to
                        add to them, which sent a person to the settings for the
                        one thing this screen exists for. Adding an extra one
                        stays on the step: «Дальше» is what moves on, and a
                        board is worked by a crew often enough. */}
                    <Show
                        when={!hasAgent() || addingAgent()}
                        fallback={

                            // In the row the QA step puts it in: on its own in
                            // the step's column the link stretched the width of
                            // the dialog and centred itself.
                            <div class='BoardSetupWizard__agentChoices'>
                                <For each={registry().agents}>
                                    {(a) => <span class='BoardSetupWizard__agentName'>{a.name}</span>}
                                </For>
                                <button
                                    type='button'
                                    class='BoardSetupWizard__pickAdd'
                                    onClick={() => setAddingAgent(true)}
                                >
                                    {intl.formatMessage({id: 'BoardSetup.add-agent', defaultMessage: 'Add an agent…'})}
                                </button>
                            </div>
                        }
                    >
                        <AgentQuickAdd
                            board={props.board}
                            onAdded={async () => {
                                if (addingAgent()) {
                                    setAddingAgent(false)
                                    await refresh()
                                    return
                                }
                                run(async () => {}, STEP_AGENT)
                            }}
                            onCancel={addingAgent() ? () => setAddingAgent(false) : undefined}
                        />
                    </Show>

                    {/* What the answer buys, said where it is given: the next
                        thing a person does with an agent is on a card. */}
                    <Show when={hasAgent()}>
                        <p class='BoardSetupWizard__hint'>
                            {intl.formatMessage({id: 'BoardSetup.agent-note', defaultMessage: 'A registered agent can be put in a card’s “assigned to” field, and a card assigned to one is worked by it as soon as the card is moved into another column.'})}
                        </p>
                    </Show>
                </div>
            )
        case STEP_DEPLOY:
            return (
                <div class='BoardSetupWizard__step'>
                    <p>{intl.formatMessage({id: 'BoardSetup.deploy-why', defaultMessage: 'Where a card’s branch is published from the "Деплой" column. Skip it if nothing is deployed from here — everything else still works.'})}</p>
                    <label>
                        {intl.formatMessage({id: 'BoardSetup.deploy-name', defaultMessage: 'Name'})}
                        <input
                            value={deploy().name}
                            onInput={(e) => setDeploy({...deploy(), name: e.currentTarget.value})}
                        />
                    </label>
                    <label>
                        {intl.formatMessage({id: 'BoardSetup.deploy-host', defaultMessage: 'Dokku host'})}
                        <input
                            value={deploy().sshHost}
                            onInput={(e) => setDeploy({...deploy(), sshHost: e.currentTarget.value})}
                        />
                    </label>
                    <label>
                        {intl.formatMessage({id: 'BoardSetup.deploy-user', defaultMessage: 'SSH user (default dokku)'})}
                        <input
                            value={deploy().sshUser}
                            onInput={(e) => setDeploy({...deploy(), sshUser: e.currentTarget.value})}
                        />
                    </label>
                    <label>
                        {intl.formatMessage({id: 'BoardSetup.deploy-key', defaultMessage: 'SSH key (absolute path, optional)'})}
                        <input
                            value={deploy().sshKey}
                            onInput={(e) => setDeploy({...deploy(), sshKey: e.currentTarget.value})}
                        />
                    </label>
                    <label>
                        {intl.formatMessage({id: 'BoardSetup.deploy-domain', defaultMessage: 'Preview domain (optional)'})}
                        <input
                            value={deploy().baseDomain}
                            onInput={(e) => setDeploy({...deploy(), baseDomain: e.currentTarget.value})}
                        />
                    </label>
                </div>
            )
        default:
            return (
                <div class='BoardSetupWizard__step'>
                    <p>{intl.formatMessage({id: 'BoardSetup.done-how', defaultMessage: 'Assign a card to an agent and drag it into another column: the move is what starts the work. The agent reads the card, does it, writes what it did in a comment and moves the card on itself.'})}</p>
                </div>
            )
        }
    }

    const actions = () => {
        switch (step()) {
        // The one step with no way past it: every other question is about the
        // machine and can be left for later, and this one is about the board
        // in front of you.
        case STEP_NAME:
            return (
                <Button
                    emphasis='primary'
                    disabled={busy() || !boardName().trim() || nameTaken()}
                    onClick={renameBoard}
                >
                    {intl.formatMessage({id: 'BoardSetup.next', defaultMessage: 'Next'})}
                </Button>
            )
        case STEP_WORKDIR:
            return (
                <>
                    <Button
                        emphasis='primary'
                        disabled={busy() || !workdirAnswered()}
                        onClick={() => {
                        // Three answers behind one button: use the folder that
                        // is already registered, add the one just picked, or —
                        // with a folder already on this board — pass the step.
                            if (taken()) {
                                return useTakenWorkdir()
                            }
                            return workdirPath() && workdirName().trim() ? addWorkdir() : pass(STEP_WORKDIR)
                        }}
                    >
                        {taken() ? intl.formatMessage({id: 'Workdirs.use-here', defaultMessage: 'Use it here'}) : intl.formatMessage({id: 'BoardSetup.next', defaultMessage: 'Next'})}
                    </Button>

                    {/* Offered exactly when there is no answer to give, which is
                    when «Дальше» is disabled: until this, somebody without the
                    folder to hand had to leave the whole wizard to reach the
                    questions after it. What a board gives up by passing is said
                    on the step itself. */}
                    <Show when={!workdirAnswered()}>
                        <Button onClick={() => skip(STEP_WORKDIR)}>
                            {intl.formatMessage({id: 'BoardSetup.skip', defaultMessage: 'Skip'})}
                        </Button>
                    </Show>
                </>
            )
        case STEP_AGENT:

            // The form above has its own button and moves the step itself, so
            // Next is offered only when there is nothing left to fill in.
            return (
                <Show when={hasAgent()}>
                    <Button
                        emphasis='primary'
                        disabled={busy()}
                        onClick={() => pass(STEP_AGENT)}
                    >
                        {intl.formatMessage({id: 'BoardSetup.next', defaultMessage: 'Next'})}
                    </Button>
                </Show>
            )
        case STEP_DEPLOY:
            return (
                <>
                    <Button
                        emphasis='primary'
                        disabled={busy() || !deploy().name.trim() || !deploy().sshHost.trim()}
                        onClick={addDeploy}
                    >
                        {intl.formatMessage({id: 'BoardSetup.save', defaultMessage: 'Save'})}
                    </Button>
                    <Button onClick={() => skip(STEP_DEPLOY)}>
                        {intl.formatMessage({id: 'BoardSetup.skip', defaultMessage: 'Skip'})}
                    </Button>
                </>
            )
        default:
            return (
                <Button
                    emphasis='primary'
                    onClick={finish}
                >
                    {intl.formatMessage({id: 'BoardSetup.finish', defaultMessage: 'Done'})}
                </Button>
            )
        }
    }

    const title = (of: SetupStepKind) => {
        switch (of) {
        case STEP_NAME:
            return intl.formatMessage({id: 'BoardSetup.step-name', defaultMessage: 'Name'})
        case STEP_WORKDIR:
            return intl.formatMessage({id: 'BoardSetup.step-folder', defaultMessage: 'Folder'})
        case STEP_AGENT:
            return intl.formatMessage({id: 'BoardSetup.step-agent', defaultMessage: 'Agent'})
        case STEP_DEPLOY:
            return intl.formatMessage({id: 'BoardSetup.step-deploy', defaultMessage: 'Deploy'})
        default:
            return intl.formatMessage({id: 'BoardSetup.step-done', defaultMessage: 'Ready'})
        }
    }

    return (
        <Dialog
            class='BoardSetupWizard'
            title={<span>{intl.formatMessage({id: 'BoardSetup.title', defaultMessage: 'Set up this board: {step}'}, {step: title(step())})}</span>}
            subtitle={<span>{intl.formatMessage({id: 'BoardSetup.subtitle', defaultMessage: 'The board\'s columns and routes are already in place. What is left is to say where things are on your machine.'})}</span>}
            onClose={props.onClose}
        >
            <div class='BoardSetupWizard__content'>
                <ol class='BoardSetupWizard__steps'>
                    <For each={steps()}>
                        {(entry) => (
                            <li
                                class={entry.kind === step() ? 'BoardSetupWizard__stepName--current' : ''}
                            >{title(entry.kind)}</li>
                        )}
                    </For>
                </ol>

                {/* The board's own sentence about this step, when it has one:
                    "the folder with your household notes" says more than any
                    wording of ours that has to fit every board. */}
                <Show when={stepAt(step())?.hint}>
                    <p class='BoardSetupWizard__hint'>{stepAt(step())!.hint}</p>
                </Show>

                {body()}

                <div class='BoardSetupWizard__actions'>{actions()}</div>

                <Show when={error()}>
                    <div class='BoardSetupWizard__error'>{error()}</div>
                </Show>

                {/* The soft way out, for somebody who would rather look around
                    first. Nothing here is mandatory any more — a board with no
                    deploy target is a board that does not deploy — so leaving
                    is fine, and the parting note says how to come back. The ✕
                    stays silent: it is the same leaving, chosen quietly.

                    Not on the last screen: there is nothing left to walk away
                    from there, so «Разберусь сам» beside «Готово» offered two
                    ways to do the same thing and made the finished wizard read
                    as though something were still being asked. */}
                <Show when={step() !== STEP_DONE}>
                    <button
                        type='button'
                        class='BoardSetupWizard__later'
                        onClick={() => {
                            sendFlashMessage({
                                content: intl.formatMessage({
                                    id: 'BoardSetup.come-back',
                                    defaultMessage: 'You can walk this again any time: the board’s ⋯ menu → “Walk the setup again…”.',
                                }),
                                severity: 'normal',
                                notice: true,

                                // Long enough to read a path of three menu
                                // names — and closable, for whoever does not
                                // need to.
                                milliseconds: 5000,
                            })
                            props.onClose()
                        }}
                    >
                        {intl.formatMessage({id: 'BoardSetup.later', defaultMessage: 'I’ll find my way around'})}
                    </button>
                </Show>
            </div>
        </Dialog>
    )
}

export default BoardSetupWizard
