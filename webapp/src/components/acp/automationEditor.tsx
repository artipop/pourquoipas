import {For, Show, createEffect, createMemo, createSignal} from 'solid-js'

import {useIntl, IntlShape} from '../../intl'

import {IPropertyTemplate} from '../../blocks/board'
import Button from '../../widgets/buttons/button'
import Select from '../../widgets/select'
import CompassIcon from '../../widgets/icons/compassIcon'

import FlowDiagram, {BLOCK_DRAG_TYPE, NODE_HEIGHT, NODE_WIDTH, StageCount, StageWrite, condLabel, edgeId, edgeIndexOf, stageLabel} from './flowDiagram'
import MCPField from './mcpField'
import {MCPServers} from './mcpServers'
import {
    ACTIONS,
    ASK_PROPERTY,
    ASK_WORKMODE,
    Automation,
    BoardColumn,
    CARD_CHANGED,
    ColumnSpec,
    EdgeAsk,
    EdgeCond,
    Flow,
    FlowEdge,
    FlowNode,
    FlowTrigger,
    NewProperty,
    OUTCOMES,
    PropertyWrite,
    SUCCESS,
    UpstreamWrite,
    VERDICT_VALUES,
    WRITABLE_TYPES,
    blankSpec,
    columnsOf,
    nodeFor,
    outgoing,
    resolvedWrites,
    specFor,
    unwrittenConditions,
    upsertSpec,
    upstreamWrites,
    withColumn,
    withEdge,
    withNode,
    withoutColumn,
    withoutEdge,
} from './automation'

import './automationEditor.scss'

// The board's automation on one screen: the columns as they are on the board,
// the routes drawn over them, and one panel that edits whichever of the two is
// selected.
//
// It replaces two dialogs that each held half the answer — a column's settings
// on the board and a route's graph in a menu — because the question a person
// actually has is neither half: "what happens to a card here, and where does it
// go next". Both halves are now the same picture.
//
// Nothing here loads or saves. A live board hands it the registry, a template
// hands it its own properties, and both get back the whole automation to write.

type Named = {name: string}

// A registry entry a column or a stage points at. The id is the reference; the
// name is only what the picker shows.
type Identified = Named & {id?: string}

// An agent as this editor needs to know it: its name, and whether it brings
// tools of its own — which is half the answer to "does anything here have a
// browser", the other half being the stage's own set.
type Agent = Identified & {mcpServers?: MCPServers}

type Props = {
    boardId: string

    // The select property the columns are options of — read off the automation
    // rather than chosen here (automationProperty), because which property the
    // board runs on is a fact about the board and not a preference. The others
    // are what a transition's condition can ask about.
    property?: IPropertyTemplate
    properties: IPropertyTemplate[]

    // Every property of the board — what a stage's writes and reads pick from.
    // Broader than `properties` on purpose: a preview URL wants a text field.
    allProperties?: IPropertyTemplate[]

    // The field this app writes every stage's outcome into. It is not a stage's
    // to declare — the engine writes it for all of them — so it is kept out of
    // the writes picker, exactly as the column property is. A condition may
    // still ask about it, which is the whole point of it being a closed set.
    outcomePropertyId?: string
    columns: BoardColumn[]

    automation: Automation
    triggers: FlowTrigger[]
    agents: Agent[]
    deploys: Identified[]

    // The board's folders, for the route that runs only on one of them.
    workspaces?: Identified[]

    // counts is where the board's cards actually stand, per route. Only a live
    // board has any; a template is a drawing.
    counts?: Record<string, StageCount[]>

    // worktrees says sessions get a worktree each, which is what lets a crew
    // work one column at the same time.
    worktrees?: boolean

    // The column the editor opens on — set when it was opened from that
    // column's own menu, so the answer is already on screen.
    focusColumnId?: string

    onChange: (next: Automation) => void

    // A column is the board's, not the automation's: making or renaming one
    // changes the board itself, which is why the editor asks rather than does.
    // onCreateColumn returns the created column, so a block dropped on the
    // canvas can become a stage without waiting for the board to come back.
    onCreateColumn?: (name: string) => Promise<BoardColumn | undefined>
    onRenameColumn?: (column: BoardColumn, name: string) => Promise<void>

    // onCreateProperty makes a board property for a stage's output — the same
    // bargain onCreateColumn takes, and for the same reason: a property is the
    // board's, shows on every card, and belongs to whoever owns the board. It
    // returns the created template so the write can be declared without waiting
    // for the board to come back. Absent where there is no board to patch.
    onCreateProperty?: (spec: NewProperty) => Promise<IPropertyTemplate | undefined>

    // onDeleteRoute writes a confirmed deletion through at once instead of
    // leaving it in the draft: see removeRoute. Absent where there is no
    // registry behind the editor, and then the draft is the whole of it.
    onDeleteRoute?: (flow: Flow) => void

    // onAddRouteOption puts the route's name among the options a card can name
    // it with. Without one, the route is drawn and never taken.
    onAddRouteOption?: (flow: Flow) => void
    routeOptionMissing?: (flow: Flow) => boolean

    // Registering an agent is the container's business — this editor knows
    // nothing about Go. Left out where there is nothing to register into: a
    // template is edited on a machine it will never run on.
    onAddAgent?: () => void
}

type Selection = {kind: 'node' | 'edge', id: string} | null

// NEW_PROPERTY is the writes picker's last option. A sentinel rather than the
// empty string, which already means "nowhere" in that same list.
const NEW_PROPERTY = '\u0000new'

type NewPropertyProps = {
    action: string
    taken: () => string[]

    // Answers whether the property was made: a refusal from the board leaves
    // the form standing with what was typed in it.
    onCreate: (spec: NewProperty) => Promise<boolean>
    onCancel: () => void
}

// The form that makes a board property for a stage's output.
//
// It knows the two machine outputs by their shape — a deploy stage produces one
// address, a test one verdict out of a closed set — so there it asks only what
// to call the field. For an agent stage the kind is the question that decides
// something: a transition can only ask "is it this value" of a property with a
// set of values, and a free-text one is carried to later stages instead. That
// is said here, where the choice is made, rather than in a document.
const NewPropertyForm = (props: NewPropertyProps) => {
    const intl = useIntl()
    const machine = () => props.action === 'deploy' || props.action === 'test'
    const suggested = () => {
        if (props.action === 'deploy') {
            return intl.formatMessage({id: 'Automation.new-property-preview', defaultMessage: 'Preview'})
        }
        if (props.action === 'test') {
            return intl.formatMessage({id: 'Automation.new-property-verdict', defaultMessage: 'Verdict'})
        }
        return ''
    }
    const firstKind = (): NewProperty['type'] => {
        switch (props.action) {
        case 'deploy':
            return 'url'
        case 'test':
            return 'select'
        default:
            return 'text'
        }
    }
    const [name, setName] = createSignal(suggested())
    const [kind, setKind] = createSignal<NewProperty['type']>(firstKind())
    const [values, setValues] = createSignal(props.action === 'test' ? VERDICT_VALUES.join(', ') : '')
    const [error, setError] = createSignal('')
    const [busy, setBusy] = createSignal(false)

    const options = () => values().split(',').map((v) => v.trim()).filter(Boolean)

    const submit = async () => {
        const wanted = name().trim()
        if (!wanted) {
            setError(intl.formatMessage({id: 'Automation.new-property-no-name', defaultMessage: 'Give the property a name.'}))
            return
        }
        if (props.taken().some((n) => n.trim().toLowerCase() === wanted.toLowerCase())) {
            setError(intl.formatMessage({id: 'Automation.new-property-taken', defaultMessage: 'The board already has a property with this name.'}))
            return
        }
        if (kind() === 'select' && options().length === 0) {
            setError(intl.formatMessage({id: 'Automation.new-property-no-values', defaultMessage: 'List the values, separated by commas.'}))
            return
        }
        setError('')
        setBusy(true)
        try {
            await props.onCreate({name: wanted, type: kind(), options: kind() === 'select' ? options() : undefined})
        } finally {
            setBusy(false)
        }
    }

    return (
        <div class='AutomationEditor__newProperty'>
            <div class='AutomationEditor__field'>
                <span class='AutomationEditor__label'>
                    {intl.formatMessage({id: 'Automation.new-property-name', defaultMessage: 'Name of the property'})}
                </span>
                <input
                    value={name()}
                    onInput={(e) => setName(e.currentTarget.value)}
                    onKeyDown={(e) => e.key === 'Enter' && submit()}
                />
            </div>

            <Show
                when={!machine()}
                fallback={
                    <span class='AutomationEditor__hint'>
                        <Show
                            when={props.action === 'deploy'}
                            fallback={intl.formatMessage(
                                {id: 'Automation.new-property-as-verdict', defaultMessage: 'It will hold one of: {values}.'},
                                {values: VERDICT_VALUES.join(', ')},
                            )}
                        >
                            {intl.formatMessage({id: 'Automation.new-property-as-link', defaultMessage: 'It will hold a link.'})}
                        </Show>
                    </span>
                }
            >
                <div class='AutomationEditor__field'>
                    <span class='AutomationEditor__label'>
                        {intl.formatMessage({id: 'Automation.new-property-kind', defaultMessage: 'What it holds'})}
                    </span>
                    <Select
                        value={kind()}
                        options={[
                            {value: 'text', label: intl.formatMessage({id: 'Automation.new-property-text', defaultMessage: 'text'})},
                            {value: 'url', label: intl.formatMessage({id: 'Automation.new-property-url', defaultMessage: 'a link'})},
                            {value: 'select', label: intl.formatMessage({id: 'Automation.new-property-select', defaultMessage: 'one of a set of values'})},
                        ]}
                        onChange={(next) => setKind(next as NewProperty['type'])}
                        label={intl.formatMessage({id: 'Automation.new-property-kind', defaultMessage: 'What it holds'})}
                    />
                </div>
                <Show when={kind() === 'select'}>
                    <div class='AutomationEditor__field'>
                        <span class='AutomationEditor__label'>
                            {intl.formatMessage({id: 'Automation.new-property-values', defaultMessage: 'The values, separated by commas'})}
                        </span>
                        <input
                            value={values()}
                            onInput={(e) => setValues(e.currentTarget.value)}
                            onKeyDown={(e) => e.key === 'Enter' && submit()}
                        />
                    </div>
                </Show>
                <span class='AutomationEditor__hint'>
                    {intl.formatMessage({id: 'Automation.new-property-branching', defaultMessage: 'A transition can branch on a property with a set of values. A text one is carried to the stages further on and read there.'})}
                </span>
            </Show>

            <Show when={error()}>
                <div class='AutomationEditor__warning'>{error()}</div>
            </Show>

            <div class='AutomationEditor__newPropertyActions'>
                <Button
                    emphasis='primary'
                    onClick={submit}
                    disabled={busy()}
                >
                    {intl.formatMessage({id: 'Automation.new-property-add', defaultMessage: 'Add the property'})}
                </Button>
                <Button onClick={props.onCancel}>
                    {intl.formatMessage({id: 'Automation.new-property-cancel', defaultMessage: 'Cancel'})}
                </Button>
            </div>
        </div>
    )
}

// COLUMNS_VIEW is the canvas with no route on it: what each column does, which
// is the half of the answer that holds for every card.
const COLUMNS_VIEW = ''

// How many columns stand in a row when there is no route to lay them out by.
const COLUMNS_PER_ROW = 5

// What a new column can be made as. 'none' is a plain column — a place cards
// stand, nothing runs.
const PALETTE = ['agent', 'deploy', 'test', 'none']

// promptPreview is the column's instructions squeezed into a placeholder: just
// enough to say what "as the column" would mean here.
const promptPreview = (prompt: string): string => {
    const line = prompt.split('\n')[0].trim()
    return line.length > 60 ? line.slice(0, 60) + '…' : line
}

const AutomationEditor = (props: Props) => {
    const intl = useIntl()
    const [route, setRoute] = createSignal(COLUMNS_VIEW)

    // Whether the tab on screen is the one somebody asked for. Until it is, the
    // editor may still move off the columns view by itself (below).
    const [picked, setPicked] = createSignal(false)
    const showRoute = (name: string) => {
        setPicked(true)
        setRoute(name)
    }
    const [selected, setSelected] = createSignal<Selection>(
        props.focusColumnId ? {kind: 'node', id: props.focusColumnId} : null,
    )

    // Which writes picker has its "a new property" form open, by the action it
    // belongs to. One signal because one panel is drawn at a time — the column
    // half and the stage half are exclusive — and it is dropped when the panel
    // stops being about the same thing, or the form would follow the eye onto
    // the next stage half-typed.
    const [creating, setCreating] = createSignal<string | null>(null)

    // Which route has been asked about, if any. One at a time, and dropped
    // whenever the panel stops being about it.
    const [confirmingRoute, setConfirmingRoute] = createSignal<string | null>(null)
    createEffect(() => {
        selected()
        route()
        setCreating(null)
        setConfirmingRoute(null)
    })

    const flows = () => props.automation.flows
    const specs = () => props.automation.columns
    const flow = () => flows().find((f) => f.name === route())
    const waitTriggers = createMemo(() => props.triggers.filter((t) => t.source !== 'outcome'))

    // The stages on the canvas: the route's, or one box per column when no
    // route is chosen — the columns view is the same picture without arrows.
    const nodes = createMemo<FlowNode[]>(() => {
        const current = flow()
        if (current) {
            return current.nodes
        }

        // With no route there are no arrows to lay the columns out by, and a
        // graph layout would stack all ten in one tall column. A grid in the
        // board's own order reads the way the board does.
        return props.columns.map((c, i) => ({
            id: c.optionId || c.name,
            column: c.name,
            optionId: c.optionId,
            action: '',
            x: (i % COLUMNS_PER_ROW) * (NODE_WIDTH + 40),
            y: Math.floor(i / COLUMNS_PER_ROW) * (NODE_HEIGHT + 28),
        }))
    })

    const edges = () => flow()?.edges || []

    // The screen opens on a route rather than on the bare columns, once the
    // routes have arrived. The arrows are what somebody came here to read, and
    // ten boxes with nothing drawn between them read as a route that failed to
    // draw rather than as a different view of the same board. The columns are
    // one tab away — and they are still what a click on a column's own badge
    // opens, because that click is about one column and not about a route.
    createEffect(() => {
        const first = flows()[0]
        if (first && !picked() && !props.focusColumnId) {
            setRoute(first.name)
        }
    })

    // Columns the route does not go through are still on the canvas — faded.
    const spare = createMemo(() => (flow() ? props.columns.filter((c) => !nodeFor(flow(), c)) : []))

    const columnOf = (node: FlowNode): BoardColumn | undefined =>
        props.columns.find((c) => (node.optionId && c.optionId === node.optionId) ||
            c.name.toLowerCase() === node.column.toLowerCase())

    const specOf = (node: FlowNode): ColumnSpec | undefined => {
        const column = columnOf(node)
        return column ? specFor(specs(), column) : undefined
    }

    // A crew is stored as registry ids; this is the only place that turns them
    // back into the names a person reads.
    const agentNamesOf = (ids: string[]): string[] =>
        ids.map((id) => props.agents.find((a) => a.id === id)?.name).filter((n): n is string => Boolean(n))

    // What a stage does is the column's business unless the stage says
    // otherwise — the same order the engine resolves it in.
    const actionOf = (node: FlowNode) => node.action || specOf(node)?.action || 'none'
    const crewOf = (node: FlowNode) => (node.agentIds?.length ? node.agentIds : specOf(node)?.agentIds) || []

    const selectedNode = () => {
        const current = selected()
        return current?.kind === 'node' ? nodes().find((n) => n.id === current.id) : undefined
    }

    // An edge is addressed by its index in the route; the canvas id carries it.
    const selectedEdgeIndex = (): number => {
        const current = selected()
        if (current?.kind !== 'edge') {
            return -1
        }
        const index = edgeIndexOf(current.id)
        const edge = edges()[index]
        return edge && edgeId(edge, index) === current.id ? index : -1
    }

    const selectedEdge = () => edges()[selectedEdgeIndex()]

    const updateFlow = (name: string, patch: (f: Flow) => Flow) => {
        props.onChange({...props.automation, flows: flows().map((f) => (f.name === name ? patch(f) : f))})
    }

    const updateSpec = (column: BoardColumn, patch: Partial<ColumnSpec>, base?: Automation) => {
        const automation = base || props.automation
        const current = specFor(automation.columns, column) || blankSpec(props.boardId, props.property, column)
        props.onChange({
            ...automation,
            columns: upsertSpec(automation.columns, {
                ...current,
                ...patch,
                boardId: props.boardId,
                propertyId: props.property?.id || current.propertyId,
                property: props.property?.name || current.property,
                column: column.name,
                optionId: column.optionId,
            }),
        })
    }

    const updateNodeSpec = (node: FlowNode, patch: Partial<ColumnSpec>) => {
        const column = columnOf(node)
        if (column) {
            updateSpec(column, patch)
        }
    }

    const onCanvasChange = (nextNodes: FlowNode[], nextEdges: FlowEdge[]) => {
        const current = flow()
        if (!current) {
            return
        }
        updateFlow(current.name, (f) => ({...f, nodes: nextNodes, edges: nextEdges}))
    }

    const addColumnToRoute = (column: BoardColumn, at?: {x: number, y: number}) => {
        const current = flow()
        if (!current) {
            return
        }
        updateFlow(current.name, (f) => withColumn(f, column, at))
        setSelected({kind: 'node', id: column.optionId || column.name})
    }

    // A new column of the board, doing what the button said, standing where it
    // was dropped — or wherever the layout puts it, when it was clicked rather
    // than dragged.
    const dropBlock = async (kind: string, at?: {x: number, y: number}) => {
        if (!props.onCreateColumn) {
            return
        }
        const taken = new Set(props.columns.map((c) => c.name.toLowerCase()))
        let name = blockName(intl, kind)
        for (let n = 2; taken.has(name.toLowerCase()); n++) {
            name = `${blockName(intl, kind)} ${n}`
        }
        const column = await props.onCreateColumn(name)
        if (!column) {
            return
        }

        // One onChange for the whole outcome: the spec (what the column does)
        // and, on a route, the stage standing where the block landed.
        let next = props.automation
        const current = flow()
        if (current) {
            next = {
                ...next,
                flows: next.flows.map((f) => (f.name === current.name ? withColumn(f, column, at) : f)),
            }
        }
        if (kind === 'none') {
            props.onChange(next)
        } else {
            updateSpec(column, {action: kind}, next)
        }
        setSelected({kind: 'node', id: column.optionId || column.name})
    }

    const removeFromRoute = (node: FlowNode) => {
        const current = flow()
        if (!current) {
            return
        }
        updateFlow(current.name, (f) => withoutColumn(f, node.id))
        setSelected(null)
    }

    const patchEdge = (index: number, patch: Partial<FlowEdge>) => {
        const current = flow()
        if (!current) {
            return
        }
        updateFlow(current.name, (f) => withEdge(f, index, patch))
        const selection = selected()
        if (selection?.kind === 'edge' && edgeIndexOf(selection.id) === index) {
            const edge = {...current.edges[index], ...patch}
            setSelected({kind: 'edge', id: edgeId(edge, index)})
        }
    }

    const dropEdge = (index: number) => {
        const current = flow()
        if (!current) {
            return
        }
        updateFlow(current.name, (f) => withoutEdge(f, index))
        if (selected()?.kind === 'edge') {
            setSelected(null)
        }
    }

    const addTransition = (node: FlowNode) => {
        const current = flow()
        if (!current) {
            return
        }
        const used = new Set(outgoing(current.edges, node.id).map(({edge}) => edge.on))
        const kind = [...OUTCOMES, ...waitTriggers().map((t) => t.kind)].
            find((k) => k !== CARD_CHANGED && !used.has(k)) || SUCCESS
        const target = current.nodes.find((n) => n.id !== node.id)
        if (!target) {
            return
        }
        updateFlow(current.name, (f) => ({...f, edges: [...f.edges, {from: node.id, to: target.id, on: kind}]}))

        // …and the new arrow is what the panel is about now. Without this the
        // button made something the person then had to find on the canvas,
        // which is the step the list used to save them.
        setSelected({kind: 'edge', id: edgeId({from: node.id, to: target.id, on: kind}, current.edges.length)})
        setSelected({kind: 'edge', id: edgeId({from: node.id, to: target.id, on: kind}, current.edges.length)})
    }

    // A fork: a second edge on the same event, told apart by its condition. The
    // condition starts empty and the panel opens on it — an empty condition is
    // exactly what the engine refuses, so it has to be filled to be saved.
    const addBranch = (index: number) => {
        const current = flow()
        const edge = current?.edges[index]
        if (!current || !edge) {
            return
        }
        const branch: FlowEdge = {...edge, if: {property: '', value: ''}}
        updateFlow(current.name, (f) => ({...f, edges: [...f.edges, branch]}))
        setSelected({kind: 'edge', id: edgeId(branch, current.edges.length)})
    }

    const addRoute = () => {
        const name = intl.formatMessage({id: 'Automation.new-route-name', defaultMessage: 'New route'})
        const taken = new Set(flows().map((f) => f.name.toLowerCase()))
        let unique = name
        for (let n = 2; taken.has(unique.toLowerCase()); n++) {
            unique = `${name} ${n}`
        }

        // A new route starts on the column an agent works in, if the board has
        // one: an empty canvas is a question, and this is the answer to it that
        // is right nearly every time.
        const working = props.columns.find((c) => specFor(specs(), c)?.action === 'agent')
        const blank: Flow = {
            name: unique,
            boardId: props.boardId,
            property: props.property?.name,
            nodes: [],
            edges: [],
        }
        props.onChange({...props.automation, flows: [...flows(), working ? withColumn(blank, working) : blank]})
        showRoute(unique)
        setSelected(null)
    }

    const renameRoute = (from: string, to: string) => {
        props.onChange({...props.automation, flows: flows().map((f) => (f.name === from ? {...f, name: to} : f))})
        showRoute(to)
    }

    const removeRoute = (name: string) => {
        const gone = flows().find((f) => f.name === name)
        const left = flows().filter((f) => f.name !== name)
        setConfirmingRoute(null)
        props.onChange({...props.automation, flows: left})

        // …and written through now rather than waiting for Save. Everything
        // else in this editor is a draft somebody may still be arranging; a
        // route somebody has just confirmed deleting is not, and leaving it on
        // screen until a second button reads as the first one not having
        // worked.
        if (gone) {
            props.onDeleteRoute?.(gone)
        }

        // The next route, or the columns when that was the last one — landing
        // on the columns while three routes are still there reads as having
        // deleted more than was asked for.
        showRoute(left[0]?.name ?? COLUMNS_VIEW)
        setSelected(null)
    }

    // Renaming a column happens on the board (through the container) and in the
    // draft at once: the specs and stages that name it must follow, or the
    // engine would look for a column that no longer exists.
    const renameColumn = async (node: FlowNode, name: string) => {
        const column = columnOf(node)
        if (!column || !name.trim() || name.trim() === column.name) {
            return
        }
        const trimmed = name.trim()
        await props.onRenameColumn?.(column, trimmed)
        props.onChange({
            ...props.automation,
            columns: specs().map((s) => (s.optionId === column.optionId ? {...s, column: trimmed} : s)),
            flows: flows().map((f) => ({
                ...f,
                nodes: f.nodes.map((n) => (n.optionId === column.optionId ? {...n, column: trimmed} : n)),
            })),
        })
    }

    // The select properties a condition may ask about. Two are left out. The
    // route's own column property, because a change of that is a move and not a
    // mark. And the outcome field: the app writes it for every stage, a person
    // answers it on the card in one click, and putting it in a list of things
    // to configure would make a mechanism out of something that has no
    // settings. An arrow that already asks about it says so and is not offered
    // for editing (condEditor).
    const condProperties = () => props.properties.filter((p) =>
        p.name !== (flow()?.property || props.property?.name) && p.id !== props.outcomePropertyId)

    // Whether this condition is the outcome one — matched by name, because that
    // is what an edge carries, and by the id the board records for the field.
    const isOutcomeCond = (cond: EdgeCond | undefined) => {
        const named = (props.allProperties || []).find((p) => p.id === props.outcomePropertyId)
        return Boolean(cond?.property && named && cond.property.toLowerCase() === named.name.toLowerCase())
    }

    // The same list, ordered by the graph: what the stage this arrow leaves
    // produces comes first, then what the stages before it produce, then the
    // rest of the board. A condition is a question about a value, and the
    // useful ones are the values this route actually puts on the card — an
    // alphabetical list of every select on the board buries them.
    //
    // Each is labelled with the stage that writes it, which is the answer to
    // "will this ever be filled in" — the question unwrittenConditions warns
    // about after the fact.
    const condChoices = (from: string) => {
        const current = flow()
        const written = new Map<string, string>()
        if (current) {
            const node = current.nodes.find((n) => n.id === from)
            if (node) {
                for (const w of resolvedWrites(node, specs())) {
                    written.set(w.property.toLowerCase(), node.column)
                }
            }
            for (const w of upstreamWrites(current, from, specs())) {
                if (!written.has(w.property.toLowerCase())) {
                    written.set(w.property.toLowerCase(), w.from)
                }
            }
        }
        const rank = (p: IPropertyTemplate) => (written.has(p.name.toLowerCase()) ? 0 : 1)
        return condProperties().
            slice().
            sort((a, b) => rank(a) - rank(b)).
            map((p) => {
                const stage = written.get(p.name.toLowerCase())
                return {
                    value: p.name,
                    label: stage ? intl.formatMessage(
                        {id: 'Automation.cond-property-from', defaultMessage: '{property} · written by “{stage}”'},
                        {property: p.name, stage},
                    ) : p.name,
                }
            })
    }

    // Who works something, ticked off the registry. One component for both
    // answers a stage has — the column's crew and the stage's own — because
    // they are the same question asked of two owners, and two hand-written
    // lists are how the two drifted apart.
    const crewPicker = (
        label: string,
        chosen: () => string[],
        write: (next: string[]) => void,
        note?: () => string,
    ) => (
        <div class='AutomationEditor__crew'>
            <span class='AutomationEditor__label'>{label}</span>
            <Show when={props.agents.length === 0}>
                <span class='AutomationEditor__hint'>
                    {intl.formatMessage({id: 'Automation.no-agents', defaultMessage: 'No agents registered yet.'})}
                </span>
            </Show>
            <For each={props.agents}>
                {(a) => (
                    <label class='AutomationEditor__agent'>
                        <input
                            type='checkbox'
                            checked={chosen().includes(a.id || '')}
                            onChange={() => {
                                const crew = chosen()
                                const id = a.id || ''
                                write(crew.includes(id) ? crew.filter((n) => n !== id) : [...crew, id])
                            }}
                        />
                        {a.name}
                    </label>
                )}
            </For>
            <Show when={note?.()}>
                <span class='AutomationEditor__hint'>{note?.()}</span>
            </Show>

            {/* Registering one is two answers, and asking them here beats
                sending somebody to the settings and back with the stage
                half-configured. */}
            <Show when={props.onAddAgent}>
                <button
                    type='button'
                    class='AutomationEditor__addAgent'
                    onClick={() => props.onAddAgent?.()}
                >
                    {intl.formatMessage({id: 'Automation.add-agent', defaultMessage: 'Add an agent…'})}
                </button>
            </Show>
        </div>
    )

    // What an unticked stage crew means: the column's answer, named. Nothing to
    // say on a board where no agent is registered — the picker says that
    // already.
    const columnCrewNote = (node: FlowNode): string => {
        if (props.agents.length === 0) {
            return ''
        }
        const crew = agentNamesOf(specOf(node)?.agentIds || [])
        if (crew.length === 0) {
            return intl.formatMessage({id: 'Automation.crew-column-none', defaultMessage: 'Nobody ticked, and the column names nobody either — whoever the board has will take it.'})
        }
        return intl.formatMessage(
            {id: 'Automation.crew-column', defaultMessage: 'Nobody ticked — the column’s crew works it: {crew}'},
            {crew: crew.join(', ')},
        )
    }

    // A test stage is the one kind that refuses to start without something: an
    // agent clicking through a browser needs a browser MCP server, from the
    // stage or from whoever works it. Said where the stage is, and said as what
    // it is — the card stands here and waits for a person, which is a way of
    // working a column and not a broken board. Not said at all while no agent
    // is registered: the crew picker is already saying that, and two notes
    // about one gap read as two gaps.
    const testStageHasNothingToTestWith = (node: FlowNode): boolean => {
        if (actionOf(node) !== 'test' || props.agents.length === 0) {
            return false
        }
        const own = node.mcpServers || specOf(node)?.mcpServers
        if (own && Object.keys(own).length > 0) {
            return false
        }
        const crew = crewOf(node)
        const pool = crew.length > 0 ? props.agents.filter((a) => crew.includes(a.id || '')) : props.agents
        return !pool.some((a) => a.mcpServers && Object.keys(a.mcpServers).length > 0)
    }

    const noBrowserNote = (node: FlowNode) => (
        <Show when={testStageHasNothingToTestWith(node)}>
            <div class='AutomationEditor__warning'>
                {intl.formatMessage({id: 'Automation.no-browser', defaultMessage: 'Nothing here brings a browser, so this stage will not start by itself: a card that arrives waits for a person to check it. A browser is an MCP server — put one on this column above, or on the agent in Settings → Agents.'})}
            </div>
        </Show>
    )

    // What an empty stage set means: the column's servers, named. The same
    // shape columnCrewNote has, and for the same reason — an inherited answer
    // that is not shown reads as no answer at all.
    const columnMCPNote = (node: FlowNode): string => {
        const names = Object.keys(specOf(node)?.mcpServers || {})
        if (names.length === 0) {
            return intl.formatMessage({id: 'Automation.mcp-none', defaultMessage: '— only what the agent carries itself —'})
        }
        return intl.formatMessage(
            {id: 'Automation.mcp-as-column', defaultMessage: '— as the column: {servers} —'},
            {servers: names.join(', ')},
        )
    }

    // What a stage leaves on the card, for the chips on its box: its own list
    // or its column's, with each marked branchable or not. Only a property with
    // a closed set can be branched on — an arrow asks "is it this value" — so
    // that is read off the board rather than guessed at from the name.
    const writesOf = (node: FlowNode): StageWrite[] =>
        resolvedWrites(node, specs()).map((w) => ({
            name: w.property,
            branchable: Boolean((props.allProperties || []).
                find((p) => p.name === w.property && p.type === 'select')),
        }))

    // The properties a stage may read: every board property except the column
    // one — moving between columns is the route's own business.
    const dataProperties = () =>
        (props.allProperties || []).filter((p) => p.id !== props.property?.id).map((p) => p.name)

    // And the ones it may write, which is a narrower question: SetCardFields
    // resolves a select by one of its option names and puts everything else on
    // the card as text, so a date, a person or a file would take a string the
    // board cannot read back — and createdBy/createdTime and their kin are the
    // board's own bookkeeping and are not writable at all. Offering them was
    // offering an answer that fails at the moment the stage reports.
    const writableProperties = () =>
        (props.allProperties || []).
            filter((p) => p.id !== props.property?.id && p.id !== props.outcomePropertyId && WRITABLE_TYPES.includes(p.type)).
            map((p) => p.name)

    // createProperty makes the board property and declares it in the same
    // breath: a field created for an output and left undeclared is the state
    // this whole control exists to prevent. A board that refuses the property
    // leaves the form standing with what was typed in it.
    const createProperty = async (
        spec: NewProperty,
        machine: boolean,
        value: () => PropertyWrite[],
        write: (next: PropertyWrite[] | undefined) => void,
    ): Promise<boolean> => {
        const made = await props.onCreateProperty?.(spec)
        if (!made) {
            return false
        }
        setCreating(null)
        write(machine ? [{property: made.name}] : [...value(), {property: made.name}])
        return true
    }

    // writesPicker edits a stage's declared outputs. For an agent stage each row
    // is a property plus a required flag — finish_work is refused without a
    // required one; a deploy or test stage writes one machine value (the
    // preview URL, the verdict), so it gets a single picker under the action's
    // own label. `note` names the inherited answer, the crew picker's shape.
    // A test stage has nothing left to declare. Its verdict *is* how the stage
    // ended, and how a stage ended is «Исход» — written by the engine for every
    // stage, with no declaration and no column. Offering a second field for the
    // same fact is the duplication «Исход» was made to remove.
    //
    // Go still writes a verdict into whatever a board declared before this
    // (writeStageFields), so nobody's board loses a value; there is just no way
    // to ask for a new one.
    const writesApply = (action: string) => action !== 'test'

    const writesPicker = (
        action: string,
        value: () => PropertyWrite[],
        write: (next: PropertyWrite[] | undefined) => void,
        note?: () => string,
    ) => {
        const machine = action === 'deploy' || action === 'test'
        let label = intl.formatMessage({id: 'Automation.writes', defaultMessage: 'Writes onto the card'})
        if (action === 'deploy') {
            label = intl.formatMessage({id: 'Automation.writes-deploy', defaultMessage: 'Preview address goes to the property'})
        } else if (action === 'test') {
            label = intl.formatMessage({id: 'Automation.writes-test', defaultMessage: 'Verdict goes to the property'})
        }
        const free = () => dataProperties().filter((name) => !value().some((w) => w.property === name))
        return (
            <div class='AutomationEditor__writes'>
                <span class='AutomationEditor__label'>{label}</span>
                <Show when={!machine}>
                    <For each={value()}>
                        {(w, i) => (
                            <div class='AutomationEditor__writeRow'>
                                <span class='AutomationEditor__writeName'>{w.property}</span>
                                <label class='AutomationEditor__writeRequired'>
                                    <input
                                        type='checkbox'
                                        checked={Boolean(w.required)}
                                        onChange={(e) => {
                                            const next = value().slice()
                                            next[i()] = {...w, required: e.currentTarget.checked || undefined}
                                            write(next)
                                        }}
                                    />
                                    {intl.formatMessage({id: 'Automation.write-required', defaultMessage: 'required'})}
                                </label>
                                <button
                                    type='button'
                                    class='AutomationEditor__writeRemove'
                                    title={intl.formatMessage({id: 'Automation.write-remove', defaultMessage: 'Remove'})}
                                    aria-label={intl.formatMessage({id: 'Automation.write-remove', defaultMessage: 'Remove'})}
                                    onClick={() => {
                                        const next = value().filter((_, at) => at !== i())
                                        write(next.length > 0 ? next : undefined)
                                    }}
                                >
                                    <CompassIcon icon='close'/>
                                </button>
                            </div>
                        )}
                    </For>
                </Show>
                <Select
                    value={machine ? (value()[0]?.property || '') : ''}
                    options={[
                        {value: '', label: machine ? intl.formatMessage({id: 'Automation.writes-nowhere', defaultMessage: '— nowhere —'}) : intl.formatMessage({id: 'Automation.writes-add', defaultMessage: '+ property…'})},
                        ...(machine ? writableProperties() : free()).map((name) => ({value: name, label: name})),

                        // The last resort, and on most boards the only right
                        // answer: nothing here holds a preview address or a
                        // verdict until somebody makes a field for it, and
                        // leaving the dialog to make one on the board is how a
                        // stage ends up declaring nothing.
                        ...(props.onCreateProperty ? [{value: NEW_PROPERTY, label: intl.formatMessage({id: 'Automation.writes-new', defaultMessage: '+ a new property…'})}] : []),
                    ]}
                    onChange={(name) => {
                        if (name === NEW_PROPERTY) {
                            setCreating(action)
                            return
                        }
                        if (machine) {
                            write(name ? [{property: name}] : undefined)
                            return
                        }
                        if (name) {
                            write([...value(), {property: name}])
                        }
                    }}
                    label={label}
                />
                <Show when={creating() === action}>
                    <NewPropertyForm
                        action={action}
                        taken={() => (props.allProperties || []).map((p) => p.name)}
                        onCancel={() => setCreating(null)}
                        onCreate={(spec) => createProperty(spec, machine, value, write)}
                    />
                </Show>
                <Show when={note && note()}>
                    <span class='AutomationEditor__hint'>{note!()}</span>
                </Show>
            </div>
        )
    }

    // readsPicker is the mirror: what the stage is handed on the way in.
    //
    // `inherited` is the third answer, and the one nobody has to tick: what the
    // stages ahead of this one on the route declare they write. Shown as ghost
    // rows naming the stage that produces each, and shown rather than editable
    // — this is the crew's and the prompt's own shape, where an override is
    // written by *saying* something and an empty field means the answer above.
    // A ✕ on a ghost could not work: with one row left, removing it would write
    // an empty list, and an empty list is what asks the route again.
    const readsPicker = (
        value: () => string[],
        write: (next: string[] | undefined) => void,
        note?: () => string,
        inherited?: () => UpstreamWrite[],
    ) => {
        const free = () => dataProperties().filter((name) => !value().includes(name))
        const ghosts = () => (value().length > 0 ? [] : (inherited?.() || []))
        return (
            <div class='AutomationEditor__writes'>
                <span class='AutomationEditor__label'>
                    {intl.formatMessage({id: 'Automation.reads', defaultMessage: 'Gets from the card'})}
                </span>
                <For each={ghosts()}>
                    {(w) => (
                        <div class='AutomationEditor__writeRow AutomationEditor__writeRow--inherited'>
                            <span class='AutomationEditor__writeName'>
                                {intl.formatMessage(
                                    {id: 'Automation.reads-from-stage', defaultMessage: '{property} · written by “{stage}”'},
                                    {property: w.property, stage: w.from},
                                )}
                            </span>
                        </div>
                    )}
                </For>
                <Show when={ghosts().length > 0}>
                    <span class='AutomationEditor__hint'>
                        {intl.formatMessage({id: 'Automation.reads-inherited', defaultMessage: 'The route hands these over by itself. Naming a property below replaces the whole list.'})}
                    </span>
                </Show>
                <For each={value()}>
                    {(name) => (
                        <div class='AutomationEditor__writeRow'>
                            <span class='AutomationEditor__writeName'>{name}</span>
                            <button
                                type='button'
                                class='AutomationEditor__writeRemove'
                                title={intl.formatMessage({id: 'Automation.write-remove', defaultMessage: 'Remove'})}
                                aria-label={intl.formatMessage({id: 'Automation.write-remove', defaultMessage: 'Remove'})}
                                onClick={() => {
                                    const next = value().filter((n) => n !== name)
                                    write(next.length > 0 ? next : undefined)
                                }}
                            >
                                <CompassIcon icon='close'/>
                            </button>
                        </div>
                    )}
                </For>
                <Select
                    value={''}
                    options={[
                        {value: '', label: intl.formatMessage({id: 'Automation.writes-add', defaultMessage: '+ property…'})},
                        ...free().map((name) => ({value: name, label: name})),
                    ]}
                    onChange={(name) => name && write([...value(), name])}
                    label={intl.formatMessage({id: 'Automation.reads', defaultMessage: 'Gets from the card'})}
                />
                <Show when={note && note()}>
                    <span class='AutomationEditor__hint'>{note!()}</span>
                </Show>
            </div>
        )
    }

    const deployPicker = (value: () => string, write: (next: string) => void, blank: string) => (
        <>
            <div class='AutomationEditor__field'>
                <span class='AutomationEditor__label'>
                    {intl.formatMessage({id: 'Automation.deploy', defaultMessage: 'Deploy target'})}
                </span>
                <Select
                    value={value()}
                    options={[
                        {value: '', label: blank},
                        ...props.deploys.map((d) => ({value: d.id || '', label: d.name})),
                    ]}
                    onChange={write}
                    label={intl.formatMessage({id: 'Automation.deploy', defaultMessage: 'Deploy target'})}
                />
            </div>

            {/* The registry moved out of the app's settings, and this select is
                exactly where somebody discovers it is empty — so this is where
                the way to it is said. */}
            <Show when={props.deploys.length === 0}>
                <span class='AutomationEditor__hint'>
                    {intl.formatMessage({id: 'Automation.no-deploys', defaultMessage: 'No deploy targets yet — the board’s ⋯ menu, "Where to deploy".'})}
                </span>
            </Show>
        </>
    )

    // The condition editor, shared by the node panel's rows and the edge panel:
    // one select for the kind of question, then the question's own fields.
    // What the arrow asks before it lets the card across. It is edited here and
    // not on the stage because one stage is entered in more than one way: the
    // way into work from the backlog is where somebody decides how the work will
    // be arranged, and the same card coming back from a review has that
    // decided already.
    //
    // Only a closed set can be asked about, for the reason a condition can only
    // ask about one: the answer is a click, and a click needs the answers named.
    const askChoices = (edge: FlowEdge) => {
        const taken = new Set((edge.ask || []).map((a) => (a.kind === ASK_WORKMODE ? ASK_WORKMODE : (a.property || '').toLowerCase())))
        const out = []
        if (!taken.has(ASK_WORKMODE)) {
            out.push({
                value: ASK_WORKMODE,
                label: intl.formatMessage({id: 'Automation.ask-workmode', defaultMessage: 'how the agent works with the folder'}),
            })
        }
        for (const p of condProperties()) {
            if (!taken.has(p.name.toLowerCase())) {
                out.push({value: `${ASK_PROPERTY}:${p.name}`, label: p.name})
            }
        }
        return out
    }

    const askLabel = (ask: EdgeAsk) => (ask.kind === ASK_WORKMODE ? intl.formatMessage({id: 'Automation.ask-workmode', defaultMessage: 'how the agent works with the folder'}) : ask.property || '')

    const addAsk = (index: number, choice: string) => {
        const edge = edges()[index]
        if (!edge || !choice) {
            return
        }
        const added: EdgeAsk = choice === ASK_WORKMODE ? {kind: ASK_WORKMODE} : {kind: ASK_PROPERTY, property: choice.slice(ASK_PROPERTY.length + 1)}
        patchEdge(index, {ask: [...(edge.ask || []), added]})
    }

    const patchAsk = (index: number, at: number, patch: Partial<EdgeAsk>) => {
        const edge = edges()[index]
        if (!edge) {
            return
        }
        patchEdge(index, {ask: (edge.ask || []).map((a, i) => (i === at ? {...a, ...patch} : a))})
    }

    const dropAsk = (index: number, at: number) => {
        const edge = edges()[index]
        if (!edge) {
            return
        }
        patchEdge(index, {ask: (edge.ask || []).filter((_, i) => i !== at)})
    }

    const askEditor = (index: number, edge: FlowEdge) => (
        <div class='AutomationEditor__field'>
            <span class='AutomationEditor__label'>
                {intl.formatMessage({id: 'Automation.ask-label', defaultMessage: 'Ask before the card moves on'})}
            </span>
            <For each={edge.ask || []}>
                {(ask, at) => (
                    <div class='AutomationEditor__ask'>
                        <div class='AutomationEditor__askHead'>
                            <span class='AutomationEditor__askName'>{askLabel(ask)}</span>
                            <button
                                type='button'
                                class='AutomationEditor__askDrop'
                                aria-label={intl.formatMessage({id: 'Automation.ask-remove', defaultMessage: 'Remove the question'})}
                                onClick={() => dropAsk(index, at())}
                            >
                                <CompassIcon icon='close'/>
                            </button>
                        </div>
                        <input
                            class='AutomationEditor__input'
                            value={ask.text || ''}
                            placeholder={ask.kind === ASK_WORKMODE ? intl.formatMessage({id: 'Automation.ask-workmode-default', defaultMessage: 'How should the agent work with the folder for this card?'}) : intl.formatMessage({id: 'Automation.ask-property-default', defaultMessage: 'What should “{property}” be for this card?'}, {property: ask.property})}
                            onInput={(e) => patchAsk(index, at(), {text: e.currentTarget.value})}
                        />
                    </div>
                )}
            </For>
            <Show when={askChoices(edge).length > 0}>
                <Select
                    value=''
                    options={[
                        {value: '', label: intl.formatMessage({id: 'Automation.ask-add', defaultMessage: '+ ask something here'})},
                        ...askChoices(edge),
                    ]}
                    onChange={(choice) => addAsk(index, choice)}
                    label={intl.formatMessage({id: 'Automation.ask-pick', defaultMessage: 'Add a question to this transition'})}
                />
            </Show>
            <Show when={(edge.ask || []).length > 0}>
                <span class='AutomationEditor__hint'>
                    {intl.formatMessage({id: 'Automation.ask-hint', defaultMessage: 'The card waits where it is until somebody answers on it. Nothing starts before that.'})}
                </span>
            </Show>
        </div>
    )

    const condEditor = (index: number, edge: FlowEdge) => {
        const cond = edge.if
        const isOutcome = OUTCOMES.includes(edge.on)

        // An arrow that waits for the outcome field is shown and not edited.
        // There is nothing to choose in it — the field has two values and the
        // app owns both — so a pair of dropdowns would be asking a question
        // whose answer is already the only one.
        if (isOutcomeCond(cond)) {
            return (
                <div class='AutomationEditor__cond'>
                    <span class='AutomationEditor__hint'>
                        {intl.formatMessage(
                            {id: 'Automation.cond-outcome', defaultMessage: 'When the card is marked “{value}”. That mark is the app’s own — it is set on the card, in one click.'},
                            {value: cond?.value},
                        )}
                    </span>
                </div>
            )
        }
        const kind = () => {
            if (!cond) {
                return 'always'
            }
            return cond.commentContains === undefined ? 'property' : 'comment'
        }
        const setKind = (next: string) => {
            switch (next) {
            case 'always':
                patchEdge(index, {if: undefined})
                break
            case 'comment':
                patchEdge(index, {if: {commentContains: ''}})
                break
            default:
                patchEdge(index, {if: {property: condProperties()[0]?.name || '', value: ''}})
            }
        }
        const condProperty = () => condProperties().find((p) => p.name === cond?.property)

        return (
            <div class='AutomationEditor__cond'>
                <Show when={edge.on !== CARD_CHANGED}>
                    <Select
                        value={kind()}
                        options={[
                            {value: 'always', label: intl.formatMessage({id: 'Automation.cond-always', defaultMessage: 'always'})},
                            {value: 'property', label: intl.formatMessage({id: 'Automation.cond-property', defaultMessage: 'only if the card has…'})},
                            ...(isOutcome ? [{value: 'comment', label: intl.formatMessage({id: 'Automation.cond-comment', defaultMessage: 'only if the agent wrote…'})}] : []),
                        ]}
                        onChange={setKind}
                        label={intl.formatMessage({id: 'Automation.cond-when', defaultMessage: 'Condition'})}
                    />
                </Show>

                <Show when={kind() === 'property' || edge.on === CARD_CHANGED}>
                    <div class='AutomationEditor__condFields'>
                        <Select
                            value={cond?.property || ''}
                            options={[
                                {value: '', label: intl.formatMessage({id: 'Automation.cond-pick-property', defaultMessage: '— property —'})},
                                ...condChoices(edge.from),
                            ]}
                            onChange={(next) => patchEdge(index, {if: {property: next, value: ''}})}
                            label={intl.formatMessage({id: 'Automation.cond-pick-property', defaultMessage: '— property —'})}
                        />
                        <span class='AutomationEditor__arrow'>{'='}</span>
                        <Select
                            value={cond?.value || ''}
                            options={[
                                {value: '', label: intl.formatMessage({id: 'Automation.cond-pick-value', defaultMessage: '— value —'})},
                                ...(condProperty()?.options || []).map((o) => ({value: o.value, label: o.value})),
                            ]}
                            onChange={(next) => patchEdge(index, {if: {property: cond?.property || '', value: next}})}
                            label={intl.formatMessage({id: 'Automation.cond-pick-value', defaultMessage: '— value —'})}
                        />
                    </div>
                </Show>

                <Show when={kind() === 'comment' && edge.on !== CARD_CHANGED}>
                    <input
                        value={cond?.commentContains || ''}
                        placeholder={intl.formatMessage({id: 'Automation.cond-comment-placeholder', defaultMessage: 'text in the agent’s closing comment'})}
                        onInput={(e) => patchEdge(index, {if: {commentContains: e.currentTarget.value}})}
                    />
                </Show>
            </div>
        )
    }

    return (
        <div class='AutomationEditor'>
            {/* Routes, and nothing else. «Колонки» stood at the head of this
                row and is not a route — a list of routes with something that
                is not one in it makes the reader ask what kind of thing the
                row holds. The columns are still there to be edited: the way in
                is «Настройки самой колонки…» in a stage's panel, and a board
                with no routes opens on them. While they are what is on screen
                no tab is active, which is true — the card is not on a route —
                and every route is still one click away. */}
            <div class='AutomationEditor__routes'>
                <For each={flows()}>
                    {(f) => (
                        <button
                            type='button'
                            class={`AutomationEditor__route${route() === f.name ? ' AutomationEditor__route--active' : ''}`}
                            onClick={() => {
                                showRoute(f.name)
                                setSelected(null)
                            }}
                        >
                            {f.name}
                        </button>
                    )}
                </For>
                <button
                    type='button'
                    class='AutomationEditor__route AutomationEditor__route--add'
                    onClick={addRoute}
                >
                    {intl.formatMessage({id: 'Automation.add-route', defaultMessage: '+ route'})}
                </button>
            </div>

            {/* Making a column is one line above the canvas rather than a
                column of blocks beside it: what each kind does is a sentence
                the panel says anyway, and repeating it four times took a sixth
                of the screen the picture is for. Clicking adds the column;
                dragging still places it where it lands. */}
            <Show when={props.onCreateColumn}>
                <div class='AutomationEditor__add'>
                    <span class='AutomationEditor__label'>
                        {intl.formatMessage({id: 'Automation.palette', defaultMessage: 'Add a column'})}
                    </span>
                    <For each={PALETTE}>
                        {(kind) => (
                            <button
                                type='button'
                                class={`AutomationEditor__block AutomationEditor__block--${kind}`}
                                draggable={true}
                                data-block={kind}
                                title={actionLabel(intl, kind)}
                                onDragStart={(e) => {
                                    e.dataTransfer?.setData(BLOCK_DRAG_TYPE, kind)
                                }}
                                onClick={() => dropBlock(kind)}
                            >
                                {blockName(intl, kind)}
                            </button>
                        )}
                    </For>
                </div>
            </Show>

            <div class='AutomationEditor__body'>
                <div class='AutomationEditor__canvas'>
                    <FlowDiagram
                        nodes={nodes()}
                        edges={edges()}
                        spare={spare()}
                        triggers={props.triggers}
                        counts={props.counts?.[route()]}
                        actionOf={actionOf}
                        crewOf={(node) => agentNamesOf(crewOf(node))}
                        writesOf={writesOf}
                        outcomeProperty={(props.allProperties || []).find((p) => p.id === props.outcomePropertyId)?.name}
                        selected={selected()}
                        onSelect={setSelected}
                        onChange={flow() ? onCanvasChange : undefined}
                        onAddColumn={addColumnToRoute}
                        onDropBlock={props.onCreateColumn ? dropBlock : undefined}
                    />

                    {/* The hint stands while there is nothing to look at — an
                        empty route, or a board nobody has picked a column on —
                        and goes when the picture can answer for itself. It used
                        to be two lines under the canvas at all times, which is
                        two lines the route did not get. */}
                    <Show when={flow() ? edges().length === 0 : !selectedNode()}>
                        <p class='AutomationEditor__hint'>
                            <Show
                                when={flow()}
                                fallback={intl.formatMessage({id: 'Automation.columns-hint', defaultMessage: 'Every column of the board is here. Pick one to say what happens when a card lands in it.'})}
                            >
                                {intl.formatMessage({id: 'Automation.route-hint', defaultMessage: 'Pull from the right side of a column to join it to the next one (upper point — on success, lower — on failure), from the bottom point to wait for an event. A faded column joins the route when you click or drag it.'})}
                            </Show>
                        </p>
                    </Show>
                </div>

                <div class='AutomationEditor__panel'>
                    <Show when={selectedNode()}>
                        {(node) => (
                            <div class='AutomationEditor__section'>
                                <Show
                                    when={props.onRenameColumn}
                                    fallback={<div class='AutomationEditor__panelTitle'>{node().column}</div>}
                                >
                                    <input
                                        class='AutomationEditor__panelName'
                                        value={node().column}
                                        onChange={(e) => renameColumn(node(), e.currentTarget.value)}
                                    />
                                </Show>

                                {/* The columns view is about the column, so the
                                    panel is too: what it does wherever a card
                                    lands in it, and who works it on every
                                    route. */}
                                <Show when={!flow()}>
                                    <div class='AutomationEditor__field'>
                                        <span class='AutomationEditor__label'>
                                            {intl.formatMessage({id: 'Automation.on-arrival', defaultMessage: 'When a card lands here'})}
                                        </span>
                                        <Select
                                            value={specOf(node())?.action || 'none'}
                                            options={ACTIONS.map((a) => ({value: a, label: actionLabel(intl, a)}))}
                                            onChange={(action) => updateNodeSpec(node(), {action})}
                                            label={intl.formatMessage({id: 'Automation.on-arrival', defaultMessage: 'When a card lands here'})}
                                        />
                                    </div>

                                    <Show when={(specOf(node())?.action || 'none') !== 'none'}>
                                        {crewPicker(
                                            intl.formatMessage({id: 'Automation.crew', defaultMessage: 'Worked by'}),
                                            () => specOf(node())?.agentIds || [],
                                            (agentIds) => updateNodeSpec(node(), {agentIds}),
                                        )}

                                        <label>
                                            {intl.formatMessage({id: 'Automation.limit', defaultMessage: 'At once (0 — no limit)'})}
                                            <input
                                                type='number'
                                                min={0}
                                                value={specOf(node())?.maxRunning || 0}
                                                onInput={(e) => updateNodeSpec(node(), {maxRunning: Number(e.currentTarget.value)})}
                                            />
                                        </label>

                                        <Show when={props.worktrees === false && (specOf(node())?.agentIds || []).length > 1}>
                                            <div class='AutomationEditor__warning'>
                                                {intl.formatMessage({id: 'Automation.no-worktrees', defaultMessage: 'This board works on a branch in the folder itself, so two agents cannot work one repository at the same time: the crew will take cards one after another.'})}
                                            </div>
                                        </Show>
                                    </Show>

                                    <Show when={specOf(node())?.action === 'deploy'}>
                                        {deployPicker(
                                            () => specOf(node())?.deployId || '',
                                            (deployId) => updateNodeSpec(node(), {deployId}),
                                            intl.formatMessage({id: 'Automation.deploy-default', defaultMessage: '— the card’s own —'}),
                                        )}
                                    </Show>

                                    {/* What working in this column means, said
                                        to the agent before the card's task —
                                        and into a conversation somebody opens
                                        here. In whatever language the board
                                        works in: it is passed through as
                                        written. */}
                                    <Show when={(specOf(node())?.action || 'none') !== 'none'}>
                                        <label>
                                            {intl.formatMessage({id: 'Automation.column-prompt', defaultMessage: 'What the agent is told here'})}
                                            <textarea
                                                class='AutomationEditor__prompt'
                                                rows={4}
                                                value={specOf(node())?.prompt || ''}
                                                onChange={(e) => updateNodeSpec(node(), {prompt: e.currentTarget.value.trim() || undefined})}
                                            />
                                        </label>

                                        {/* The column's outputs and inputs: what
                                            a stage here writes is what an edge
                                            can branch on and a later stage can
                                            read off the card. */}
                                        <Show when={writesApply(specOf(node())?.action || 'none')}>
                                            {writesPicker(
                                                specOf(node())?.action || 'none',
                                                () => specOf(node())?.writes || [],
                                                (writes) => updateNodeSpec(node(), {writes}),
                                            )}
                                        </Show>
                                        {readsPicker(
                                            () => specOf(node())?.reads || [],
                                            (reads) => updateNodeSpec(node(), {reads}),
                                        )}

                                        {/* The tools working here comes with.
                                            They belong to the column because
                                            the browser «QA» needs is a fact
                                            about that column rather than about
                                            whoever works it — otherwise one
                                            agent has to be registered twice to
                                            be configured differently in two
                                            columns. */}
                                        <MCPField
                                            owner={`column:${node().id}`}
                                            label={intl.formatMessage({id: 'Automation.mcp', defaultMessage: 'MCP servers of this column'})}
                                            value={specOf(node())?.mcpServers}
                                            emptySummary={intl.formatMessage({id: 'Automation.mcp-none', defaultMessage: '— only what the agent carries itself —'})}
                                            onChange={(mcpServers) => updateNodeSpec(node(), {mcpServers})}
                                        />

                                        {noBrowserNote(node())}
                                    </Show>
                                </Show>

                                {/* A route's panel is about the stage. Everything
                                    here is this stage's own answer, with the
                                    column's shown as what it falls back to —
                                    which is what puts a different agent on each
                                    node of one route. It was a fold called
                                    "only on this route…" under a second crew
                                    list, and two crews for one question read as
                                    a bug rather than as an override. */}
                                <Show when={flow()}>
                                    <div class='AutomationEditor__field'>
                                        <span class='AutomationEditor__label'>
                                            {intl.formatMessage({id: 'Automation.stage-action', defaultMessage: 'What happens at this stage'})}
                                        </span>
                                        <Select
                                            value={node().action || ''}
                                            options={[

                                                // The short name the box on the
                                                // canvas uses, not the sentence:
                                                // it has to fit in a select in a
                                                // 300px panel, and the sentence
                                                // was cut off mid-word.
                                                {
                                                    value: '',
                                                    label: intl.formatMessage(
                                                        {id: 'Automation.as-column', defaultMessage: '— as the column: {what} —'},
                                                        {what: stageLabel(intl, specOf(node())?.action || 'none')},
                                                    ),
                                                },
                                                ...ACTIONS.map((a) => ({value: a, label: actionLabel(intl, a)})),
                                            ]}
                                            onChange={(action) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {action}))}
                                            label={intl.formatMessage({id: 'Automation.stage-action', defaultMessage: 'What happens at this stage'})}
                                        />
                                    </div>

                                    <Show when={actionOf(node()) !== 'none'}>
                                        {crewPicker(
                                            intl.formatMessage({id: 'Automation.route-crew', defaultMessage: 'Worked here by'}),
                                            () => node().agentIds || [],
                                            (agentIds) => updateFlow(flow()!.name, (f) =>
                                                withNode(f, node().id, {agentIds: agentIds.length > 0 ? agentIds : undefined})),
                                            () => (node().agentIds?.length ? '' : columnCrewNote(node())),
                                        )}

                                        {/* Where the stage works, and only where
                                            that decides something. For a deploy
                                            and a test it is the whole of "QA
                                            before the merge" — one checks the
                                            card's own branch, the other what is
                                            already published. An agent stage
                                            works on the card's branch and there
                                            is no second sensible answer, so the
                                            control only stands there when
                                            somebody has already given one. */}
                                        <Show when={actionOf(node()) === 'deploy' || actionOf(node()) === 'test' || node().runIn}>
                                            <div class='AutomationEditor__field'>
                                                <span class='AutomationEditor__label'>
                                                    {intl.formatMessage({id: 'Automation.run-in', defaultMessage: 'The stage works'})}
                                                </span>
                                                <Select
                                                    value={node().runIn || ''}
                                                    options={[
                                                        {value: '', label: intl.formatMessage({id: 'Automation.run-in-default', defaultMessage: '— as this kind of stage usually does —'})},
                                                        {value: 'owner', label: intl.formatMessage({id: 'Automation.run-in-owner', defaultMessage: 'on the card’s own branch'})},
                                                        {value: 'workdir', label: intl.formatMessage({id: 'Automation.run-in-workdir', defaultMessage: 'in the folder itself'})},
                                                    ]}
                                                    onChange={(runIn) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {runIn: runIn || undefined}))}
                                                    label={intl.formatMessage({id: 'Automation.run-in', defaultMessage: 'The stage works'})}
                                                />
                                            </div>
                                        </Show>
                                    </Show>

                                    <Show when={actionOf(node()) === 'deploy'}>
                                        {deployPicker(
                                            () => node().deployId || '',
                                            (deployId) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {deployId})),
                                            intl.formatMessage({id: 'Automation.deploy-as-column', defaultMessage: '— as the column —'}),
                                        )}
                                    </Show>

                                    {/* The stage's own instructions, overriding
                                        the column's — «Ревью» on this route may
                                        brief its reviewer differently. Empty
                                        inherits, and the placeholder names the
                                        answer it falls back to. */}
                                    <Show when={actionOf(node()) !== 'none'}>
                                        <label>
                                            {intl.formatMessage({id: 'Automation.stage-prompt', defaultMessage: 'What the agent is told at this stage'})}
                                            <textarea
                                                class='AutomationEditor__prompt'
                                                rows={4}
                                                value={node().prompt || ''}
                                                placeholder={specOf(node())?.prompt ? intl.formatMessage({id: 'Automation.prompt-as-column', defaultMessage: '— as the column: “{prompt}” —'}, {prompt: promptPreview(specOf(node())!.prompt!)}) : intl.formatMessage({id: 'Automation.prompt-as-column-none', defaultMessage: '— as the column: nothing extra —'})}
                                                onChange={(e) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {prompt: e.currentTarget.value.trim() || undefined}))}
                                            />
                                        </label>

                                        {/* The stage's own outputs and inputs,
                                            overriding the column's — shown
                                            resolved, written to the node. */}
                                        <Show when={writesApply(actionOf(node()))}>
                                            {writesPicker(
                                                actionOf(node()),
                                                () => node().writes || specOf(node())?.writes || [],
                                                (writes) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {writes})),
                                            )}
                                        </Show>
                                        {readsPicker(
                                            () => node().reads || specOf(node())?.reads || [],
                                            (reads) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {reads})),
                                            undefined,
                                            () => upstreamWrites(flow()!, node().id, specs()),
                                        )}

                                        {/* The stage's own tools, replacing the
                                            column's whole set — the same
                                            inheritance the crew and the prompt
                                            have, and the summary names what it
                                            falls back to. */}
                                        <MCPField
                                            owner={`stage:${flow()!.name}:${node().id}`}
                                            label={intl.formatMessage({id: 'Automation.stage-mcp', defaultMessage: 'MCP servers at this stage'})}
                                            value={node().mcpServers}
                                            emptySummary={columnMCPNote(node())}
                                            onChange={(mcpServers) => updateFlow(flow()!.name, (f) => withNode(f, node().id, {mcpServers}))}
                                        />

                                        {noBrowserNote(node())}
                                    </Show>

                                    {/* The way to the other half of the answer.
                                        Without it "as the column" names a place
                                        with no door to it. */}
                                    <button
                                        type='button'
                                        class='AutomationEditor__addAgent'
                                        onClick={() => showRoute(COLUMNS_VIEW)}
                                    >
                                        {intl.formatMessage({id: 'Automation.open-column', defaultMessage: 'Settings of the column itself…'})}
                                    </button>

                                    {/* The arrows are on the canvas, and this
                                        used to be all of them again: two or
                                        three selects each, in a 300px column,
                                        saying what the picture beside it says.
                                        One arrow is edited by clicking it —
                                        the edge panel is the same panel — so
                                        what is left here is the way to make a
                                        new one. */}
                                    <div class='AutomationEditor__transitions'>
                                        <span class='AutomationEditor__label'>
                                            {intl.formatMessage({id: 'Automation.transitions', defaultMessage: 'From here the card goes'})}
                                        </span>
                                        <span class='AutomationEditor__hint'>
                                            <Show
                                                when={outgoing(edges(), node().id).length > 0}
                                                fallback={intl.formatMessage({id: 'Automation.transitions-none', defaultMessage: 'Nowhere yet. Pull from the stage’s edge on the canvas, or add one below.'})}
                                            >
                                                {intl.formatMessage(
                                                    {id: 'Automation.transitions-count', defaultMessage: '{count, plural, one {# transition} few {# transitions} other {# transitions}} — click an arrow on the canvas to edit it.'},
                                                    {count: outgoing(edges(), node().id).length},
                                                )}
                                            </Show>
                                        </span>
                                        <Button onClick={() => addTransition(node())}>
                                            {intl.formatMessage({id: 'Automation.add-transition', defaultMessage: 'Add a transition'})}
                                        </Button>
                                        <Button onClick={() => removeFromRoute(node())}>
                                            {intl.formatMessage({id: 'Automation.remove-from-route', defaultMessage: 'Take off this route'})}
                                        </Button>
                                    </div>
                                </Show>
                            </div>
                        )}
                    </Show>

                    <Show when={selectedEdge()}>
                        {(edge) => (
                            <div class='AutomationEditor__section'>
                                <div class='AutomationEditor__panelTitle'>
                                    {intl.formatMessage({id: 'Automation.transition', defaultMessage: 'Transition'})}
                                    <Show when={condLabel(intl, edge().if)}>
                                        <span class='AutomationEditor__condBadge'>{condLabel(intl, edge().if)}</span>
                                    </Show>
                                </div>
                                <div class='AutomationEditor__field'>
                                    <span class='AutomationEditor__label'>
                                        {intl.formatMessage({id: 'Automation.transition-on', defaultMessage: 'When'})}
                                    </span>
                                    <Select
                                        value={edge().on}
                                        options={props.triggers.map((t) => ({value: t.kind, label: t.label}))}
                                        onChange={(on) => patchEdge(selectedEdgeIndex(), {on})}
                                        label={intl.formatMessage({id: 'Automation.transition-on', defaultMessage: 'When'})}
                                    />
                                </div>
                                {condEditor(selectedEdgeIndex(), edge())}
                                {askEditor(selectedEdgeIndex(), edge())}

                                {/* A fork is two arrows on the same event, and
                                    the ordinary way to draw one is to pull
                                    twice from the stage's output. This is the
                                    way for an arrow with no output to pull
                                    from — a condition on what the agent wrote.
                                    It lived on every row of a list of every
                                    arrow; it belongs to the one selected. */}
                                <Show when={OUTCOMES.includes(edge().on) && !edge().if}>
                                    <button
                                        type='button'
                                        class='AutomationEditor__branch'
                                        onClick={() => addBranch(selectedEdgeIndex())}
                                    >
                                        {intl.formatMessage({id: 'Automation.add-branch', defaultMessage: '+ branch on a condition'})}
                                    </button>
                                </Show>
                                <div class='AutomationEditor__field'>
                                    <span class='AutomationEditor__label'>
                                        {intl.formatMessage({id: 'Automation.transition-to', defaultMessage: 'The card moves to'})}
                                    </span>
                                    <Select
                                        value={edge().to}
                                        options={nodes().filter((n) => n.id !== edge().from).map((n) => ({value: n.id, label: n.column}))}
                                        onChange={(to) => patchEdge(selectedEdgeIndex(), {to})}
                                        label={intl.formatMessage({id: 'Automation.transition-to', defaultMessage: 'The card moves to'})}
                                    />
                                </div>
                                <Button onClick={() => dropEdge(selectedEdgeIndex())}>
                                    {intl.formatMessage({id: 'Automation.remove-transition', defaultMessage: 'Remove'})}
                                </Button>
                            </div>
                        )}
                    </Show>

                    <Show when={!selectedNode() && !selectedEdge() ? flow() : undefined}>
                        {(current) => (
                            <div class='AutomationEditor__section'>
                                <div class='AutomationEditor__panelTitle'>
                                    {intl.formatMessage({id: 'Automation.route', defaultMessage: 'Route'})}
                                </div>
                                <label>
                                    {intl.formatMessage({id: 'Automation.route-name', defaultMessage: 'Name'})}
                                    <input
                                        value={current().name}
                                        onChange={(e) => renameRoute(current().name, e.currentTarget.value.trim() || current().name)}
                                    />
                                </label>
                                <div class='AutomationEditor__hint'>
                                    {intl.formatMessage({id: 'Automation.route-name-hint', defaultMessage: 'A card takes this route by naming it — the option of the same name on the card.'})}
                                </div>
                                <Show when={props.routeOptionMissing?.(current()) && props.onAddRouteOption}>
                                    <div class='AutomationEditor__warning'>
                                        {intl.formatMessage({id: 'Automation.route-option-missing', defaultMessage: 'No card can name this route: the board has no option called that.'})}
                                        <Button onClick={() => props.onAddRouteOption?.(current())}>
                                            {intl.formatMessage({id: 'Automation.add-route-option', defaultMessage: 'Add the option'})}
                                        </Button>
                                    </div>
                                </Show>

                                {/* The dataflow check: an edge waiting on a
                                    property nothing on this route writes may be
                                    a person's own click — or a route that
                                    quietly never moves. Named, not refused. */}
                                <For each={unwrittenConditions(current(), specs(), props.property?.name)}>
                                    {(property) => (
                                        <div class='AutomationEditor__warning'>
                                            {intl.formatMessage(
                                                {id: 'Automation.unwritten-condition', defaultMessage: 'A transition reads «{property}», but no stage of this route writes it. Fine if a person sets it by hand; otherwise mark the stage that produces it in "Writes onto the card".'},
                                                {property},
                                            )}
                                        </div>
                                    )}
                                </For>
                                <div class='AutomationEditor__field'>
                                    <span class='AutomationEditor__label'>
                                        {intl.formatMessage({id: 'Automation.route-project', defaultMessage: 'Cards from this folder take this route'})}
                                    </span>
                                    <Select
                                        value={current().workspaceId || ''}
                                        options={[
                                            {value: '', label: intl.formatMessage({id: 'Automation.route-project-any', defaultMessage: '— no, the card names the route itself —'})},
                                            ...(props.workspaces || []).map((w) => ({value: w.id || '', label: w.name})),
                                        ]}
                                        onChange={(workspaceId) => updateFlow(current().name, (f) => ({...f, workspaceId}))}
                                        label={intl.formatMessage({id: 'Automation.route-project', defaultMessage: 'Cards from this folder take this route'})}
                                    />
                                </div>
                                <div class='AutomationEditor__hint'>
                                    {intl.formatMessage({id: 'Automation.route-project-hint', defaultMessage: 'A folder here is a rule about which route a card takes, not about where the agent works: a card that names no route and resolves to this folder goes this way. Where the work happens is the card’s own field.'})}
                                </div>
                                <div class='AutomationEditor__hint'>
                                    {intl.formatMessage(
                                        {id: 'Automation.route-columns', defaultMessage: 'Goes through: {columns}'},
                                        {columns: columnsOf(current(), props.columns).map((c) => c.name).join(' → ') || '—'},
                                    )}
                                </div>
                                {/* Asked about, and then done — not queued behind
                                    Save. Deleting a route is the one edit here
                                    that cannot be undone by editing it back:
                                    what goes with it is where every card
                                    standing on it was. So it takes a second
                                    press, and the second press is the whole
                                    act. */}
                                <Show
                                    when={confirmingRoute() === current().name}
                                    fallback={
                                        <Button onClick={() => setConfirmingRoute(current().name)}>
                                            {intl.formatMessage({id: 'Automation.remove-route', defaultMessage: 'Delete this route'})}
                                        </Button>
                                    }
                                >
                                    <div class='AutomationEditor__warning'>
                                        {intl.formatMessage(
                                            {id: 'Automation.remove-route-ask', defaultMessage: 'Delete “{name}”? The cards standing on it lose their place.'},
                                            {name: current().name},
                                        )}
                                        <div class='AutomationEditor__newPropertyActions'>
                                            <Button
                                                emphasis='primary'
                                                onClick={() => removeRoute(current().name)}
                                            >
                                                {intl.formatMessage({id: 'Automation.remove-route-yes', defaultMessage: 'Delete'})}
                                            </Button>
                                            <Button onClick={() => setConfirmingRoute(null)}>
                                                {intl.formatMessage({id: 'Automation.remove-route-no', defaultMessage: 'Keep it'})}
                                            </Button>
                                        </div>
                                    </div>
                                </Show>
                            </div>
                        )}
                    </Show>

                    {/* One sentence, so not a `__section`: that class is a
                        column flex container, and a bare string inside one is
                        laid out at its full one-line width rather than wrapped
                        — which is what pushed the panel's content half again
                        past its own edge. */}
                    <Show when={!selectedNode() && !selectedEdge() && !flow()}>
                        <p class='AutomationEditor__hint'>
                            {intl.formatMessage({id: 'Automation.empty-panel', defaultMessage: 'Settings of the selected column or route appear here. Pick a column to start.'})}
                        </p>
                    </Show>
                </div>
            </div>
        </div>
    )
}

// blockName is what a palette block's column is called until somebody renames
// it — the noun, where actionLabel is the sentence.
export function blockName(intl: IntlShape, kind: string): string {
    switch (kind) {
    case 'agent':
        return intl.formatMessage({id: 'Automation.block-agent', defaultMessage: 'Agent'})
    case 'deploy':
        return intl.formatMessage({id: 'Automation.block-deploy', defaultMessage: 'Deploy'})
    case 'test':
        return intl.formatMessage({id: 'Automation.block-test', defaultMessage: 'Test'})
    default:
        return intl.formatMessage({id: 'Automation.block-none', defaultMessage: 'Column'})
    }
}

// actionLabel names what happens in a column, in the reader's language.
export function actionLabel(intl: IntlShape, action: string): string {
    switch (action) {
    case 'agent':
        return intl.formatMessage({id: 'Automation.action-agent', defaultMessage: 'an agent works on the card'})
    case 'deploy':
        return intl.formatMessage({id: 'Automation.action-deploy', defaultMessage: 'deploy the card’s branch'})
    case 'test':
        return intl.formatMessage({id: 'Automation.action-test', defaultMessage: 'test the preview'})
    default:
        return intl.formatMessage({id: 'Automation.action-none', defaultMessage: 'nothing — the card waits'})
    }
}

export default AutomationEditor
