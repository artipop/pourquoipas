import {Board, IPropertyOption, IPropertyTemplate} from '../../blocks/board'

import {MCPServers} from './mcpServers'

// What a board does, in the two halves the engine keeps apart: a **column**
// says what happens when a card lands in it, a **route** says where the card
// goes next. Both are edited over the board's own columns — the options of one
// select property — which is why they live in one module: a stage that is not a
// column of the board is a stage no card can ever stand on.
//
// Nothing here talks to Go or to the board store. The editor is the same
// whether it is pointed at the registry (a live board) or at a template's own
// properties, and both containers hand it these shapes.

export type FlowNode = {
    id: string
    column: string

    // The option the stage stands on, so renaming the column on the board
    // changes nothing. Empty in a route written before columns had ids.
    optionId?: string

    // Empty means "whatever the column does"; 'none' means the stage runs
    // nothing and waits for an event.
    action: string

    // The crew for this stage, by registry id: renaming an agent used to empty
    // the crew of every stage on every board.
    agentIds?: string[]

    // The deploy target this stage publishes to, by the registry entry's id.
    // It used to be the entry's name, and renaming a target then unpinned every
    // stage that sent work to it.
    deployId?: string

    // The stage's own instructions, said to whoever works it here — overriding
    // the column's (ColumnSpec.prompt). Empty inherits.
    prompt?: string

    // The stage's declared outputs and inputs, overriding the column's. A write
    // is what makes a transition on a property deterministic: the edge that
    // reads it can point at the stage that must produce it.
    writes?: PropertyWrite[]
    reads?: string[]

    // The tools this stage hands its agent, overriding the column's whole set.
    // Empty inherits.
    mcpServers?: MCPServers

    // Where the stage works: 'owner' — the card's own workspace, so it sees the
    // card's branch; 'workdir' — the folder itself. Empty is the default for
    // what the stage does (Go's FlowNode.RunsIn).
    runIn?: string

    // Where the stage was left on the canvas. Absent means "lay it out for me".
    x?: number
    y?: number
}

export type FlowEdge = {
    from: string
    to: string
    on: string

    // A condition makes the transition ask about the card: several conditional
    // edges may share one (from, on) — the first that holds wins — and an edge
    // without one is the fallback. For the card.changed trigger the condition
    // is the event itself: which option firing it means.
    if?: EdgeCond

    // What a person is asked before the card is let across. The card waits
    // where it is until every question has an answer — see EdgeAsk.
    ask?: EdgeAsk[]
}

// EdgeAsk is one question the arrow asks. It is on the arrow and not on the
// stage because one stage is entered in more than one way and the answers
// differ: the way into work from the inbox is where somebody decides how the
// work will be arranged, and the same card coming back from a review has that
// decided already.
export type EdgeAsk = {

    // The question's own id, minted by Go on save. Empty on one just added.
    id?: string

    // ASK_WORKMODE or ASK_PROPERTY.
    kind: string

    // The board select the answer is written to, by name, for ASK_PROPERTY.
    property?: string

    // What is asked, in the board's own words. Empty takes the built-in
    // wording, which the editor shows as the placeholder.
    text?: string
}

// EdgeCond is exactly one of two questions: does the card's select property
// carry this value, or did the agent's closing words contain this text.
export type EdgeCond = {
    property?: string
    value?: string
    commentContains?: string
}

export type Flow = {

    // The route's own id, given by Go. It is what an edit is matched on, so a
    // route read from the board must be sent back with it — renaming one used
    // to lose the place of every card standing on it.
    id?: string
    name: string
    boardId?: string

    // The folder this route is tied to, by registry id.
    workspaceId?: string
    property?: string
    nodes: FlowNode[]
    edges: FlowEdge[]
}

// FlowTrigger is one transition kind the engine implements. The list comes from
// Go, so the editor can never offer a transition that does nothing.
export type FlowTrigger = {
    kind: string
    source: string
    label: string
}

// ColumnSpec is what happens in one column — said once for the board, rather
// than repeated in every route that passes through it.
export type ColumnSpec = {
    boardId?: string
    propertyId?: string
    optionId?: string
    property: string
    column: string
    action: string
    agentIds?: string[]
    deployId?: string
    maxRunning?: number

    // What working in this column means, said to the agent: the reviewer's
    // brief on the review column, the builder's on the in-progress column. A route node may override
    // it for its stage alone (FlowNode.prompt).
    prompt?: string

    // The column's declared outputs and inputs — see FlowNode.writes/reads.
    writes?: PropertyWrite[]
    reads?: string[]

    // The tools working in this column comes with, handed to whichever agent
    // works here on top of the servers that agent carries of its own. A route
    // node may replace the whole set for its stage (FlowNode.mcpServers).
    mcpServers?: MCPServers
}

// PropertyWrite is one property a stage puts on the card. The value is not
// here: an agent stage supplies it through finish_work (refused without it
// when required), a deploy stage writes its preview URL, a test its verdict.
export type PropertyWrite = {
    property: string
    required?: boolean
}

// Automation is a board's whole answer: what its columns do and where its
// routes lead. It is what the editor edits and what a template carries.
export type Automation = {
    columns: ColumnSpec[]
    flows: Flow[]
}

// The two outcome triggers are a stage's own outputs and are drawn as such;
// everything else is an event the stage waits for.
export const SUCCESS = 'success'
export const FAILURE = 'failure'
export const BLOCKED = 'blocked'

// The two kinds of question an arrow can ask. ASK_WORKMODE is the machine's
// own — how this card's work is arranged in the folder — and its answer goes
// into the card's workspace rather than onto the card. ASK_PROPERTY writes the
// answer into one of the board's own selects, which puts it in the route's
// dataflow like any other write.
export const ASK_WORKMODE = 'workmode'
export const ASK_PROPERTY = 'property'

// CARD_CHANGED is the board-side trigger: an option set on the card itself.
// Its edge must say which option through its condition.
export const CARD_CHANGED = 'card.changed'

export const ACTIONS = ['none', 'agent', 'deploy', 'test']

// The board property types a stage can actually write. SetCardFields resolves a
// select by one of its option names and stores everything else as text, so a
// date, a person or a file would be handed a string the board cannot read back,
// and createdBy/createdTime and their kin are the board's own bookkeeping.
export const WRITABLE_TYPES = ['text', 'url', 'email', 'phone', 'number', 'select']

// NewProperty is a board property a stage's outputs need and the board has not
// got. The editor asks for it rather than making it, the way it asks for a
// column: a property is the board's, visible on every card, and belongs to
// whoever owns the board rather than to the automation over it.
export type NewProperty = {
    name: string

    // 'select' is the one that makes the property branchable — a transition can
    // only ask "is it this value" of a closed set. 'text' and 'url' are carried
    // to later stages and read there.
    type: 'text' | 'url' | 'select'

    // The option names, for a select. Empty for the rest.
    options?: string[]
}

// The names this app gives the two machine outputs, and their shape. A deploy
// stage produces one address and a test one verdict, so neither is a question:
// the form asks for the name and says what kind it will be.
export const VERDICT_VALUES = ['pass', 'fail', 'blocked']

// OUTCOMES are the triggers produced by the stage's own action — the only ones
// where the agent spoke, and therefore the only ones a comment condition makes
// sense on.
export const OUTCOMES = [SUCCESS, FAILURE, BLOCKED]

// BoardColumn is a column as the board has it: an option of a select property.
// The editor never invents one — a column exists on the board first.
export type BoardColumn = {
    optionId: string
    name: string
    color?: string
}

// selectProperties are the properties a board can be organised in columns by.
export function selectProperties(board: Board): IPropertyTemplate[] {
    return (board.cardProperties || []).filter((p) => p.type === 'select')
}

// columnProperty is the property the automation is written against: the one it
// names, or the board's first select property.
export function columnProperty(board: Board, name?: string): IPropertyTemplate | undefined {
    const properties = selectProperties(board)
    return properties.find((p) => p.name === name) || properties[0]
}

// automationProperty is the same question answered by the automation itself:
// which of the board's select properties its columns and routes were written
// for. A spec carries the id of that property and the name it had, a route
// carries the name, and either is a better answer than "the first select" —
// which is a fact about the order somebody added fields in, and would have the
// editor draw one property's options while the engine runs another's.
//
// It is not a preference and there is no control for it: it is a fact about the
// board, read here.
export function automationProperty(board: Board, automation?: Automation): IPropertyTemplate | undefined {
    const properties = selectProperties(board)
    for (const spec of automation?.columns || []) {
        const found = properties.find((p) => (spec.propertyId && p.id === spec.propertyId) ||
            (spec.property && p.name === spec.property))
        if (found) {
            return found
        }
    }
    for (const flow of automation?.flows || []) {
        const found = properties.find((p) => flow.property && p.name === flow.property)
        if (found) {
            return found
        }
    }
    return properties[0]
}

export function boardColumns(board: Board, propertyName?: string): BoardColumn[] {
    const property = columnProperty(board, propertyName)
    return (property?.options || []).map((o: IPropertyOption) => ({optionId: o.id, name: o.value, color: o.color}))
}

// specFor is a column's saved behaviour: matched by the option it is, and only
// then by name — a spec written before the editor knew the ids carries none.
export function specFor(specs: ColumnSpec[], column: BoardColumn): ColumnSpec | undefined {
    return specs.find((s) => s.optionId && s.optionId === column.optionId) ||
        specs.find((s) => !s.optionId && s.column.toLowerCase() === column.name.toLowerCase())
}

// blankSpec is what a column nobody has configured does: nothing.
export function blankSpec(boardId: string, property: IPropertyTemplate | undefined, column: BoardColumn): ColumnSpec {
    return {
        boardId,
        propertyId: property?.id,
        optionId: column.optionId,
        property: property?.name || '',
        column: column.name,
        action: 'none',
    }
}

// upsertSpec replaces a column's settings in the list, or appends them.
export function upsertSpec(specs: ColumnSpec[], spec: ColumnSpec): ColumnSpec[] {
    const at = specs.findIndex((s) => sameColumn(s, spec))
    if (at < 0) {
        return [...specs, spec]
    }
    return specs.map((s, i) => (i === at ? spec : s))
}

// sameColumn is the predicate Go keys the registry on: the same option of the
// same board, or — before either is known — the same name.
export function sameColumn(a: ColumnSpec, b: ColumnSpec): boolean {
    if (a.optionId && b.optionId) {
        return a.optionId === b.optionId
    }
    return a.column.toLowerCase() === b.column.toLowerCase()
}

// nodeFor is the stage a column stands as in a route, if the route goes through
// it at all.
export function nodeFor(flow: Flow | undefined, column: BoardColumn): FlowNode | undefined {
    if (!flow) {
        return undefined
    }
    return flow.nodes.find((n) => n.optionId && n.optionId === column.optionId) ||
        flow.nodes.find((n) => !n.optionId && n.column.toLowerCase() === column.name.toLowerCase())
}

// nodeId is what a column is called inside a route. The option's own id is used
// so the same column keeps the same stage across edits — the engine records the
// stage a card stands on, and a regenerated id would lose it.
export function nodeId(column: BoardColumn): string {
    return column.optionId || column.name
}

// withColumn puts a column on the route, if it is not on it already. A stage
// added this way overrides nothing: it does whatever its column does.
export function withColumn(flow: Flow, column: BoardColumn, at?: {x: number, y: number}): Flow {
    if (nodeFor(flow, column)) {
        return flow
    }
    const node: FlowNode = {
        id: nodeId(column),
        column: column.name,
        optionId: column.optionId,
        action: '',
...(at || {}),
    }
    return {...flow, nodes: [...flow.nodes, node]}
}

// withoutColumn takes a stage off the route, and its transitions with it: an
// edge to nowhere is exactly what the engine refuses to save.
export function withoutColumn(flow: Flow, id: string): Flow {
    return {
...flow,
        nodes: flow.nodes.filter((n) => n.id !== id),
        edges: flow.edges.filter((e) => e.from !== id && e.to !== id),
    }
}

// withNode changes one stage of a route.
export function withNode(flow: Flow, id: string, patch: Partial<FlowNode>): Flow {
    return {...flow, nodes: flow.nodes.map((n) => (n.id === id ? {...n, ...patch} : n))}
}

// Edges are addressed by their index in the flow: with conditions, several may
// share one (from, on), so the pair stops being an identity.

// edgeTarget is the stage an event leads to unconditionally, or '' when the
// graph says nothing — the fallback edge, in engine terms.
export function edgeTarget(edges: FlowEdge[], from: string, on: string): string {
    return edges.find((e) => e.from === from && e.on === on && !e.if)?.to || ''
}

// outgoing is everything a stage leads to, with the index each edge is edited
// by — outcomes first, which is the order a person reads them in.
export function outgoing(edges: FlowEdge[], from: string): Array<{edge: FlowEdge, index: number}> {
    const rank = (on: string) => {
        const at = OUTCOMES.indexOf(on)
        return at < 0 ? OUTCOMES.length : at
    }
    return edges.
        map((edge, index) => ({edge, index})).
        filter(({edge}) => edge.from === from).
        sort((a, b) => (rank(a.edge.on) - rank(b.edge.on)) || (a.index - b.index))
}

// withEdge changes one transition of a route.
export function withEdge(flow: Flow, index: number, patch: Partial<FlowEdge>): Flow {
    return {...flow, edges: flow.edges.map((e, i) => (i === index ? {...e, ...patch} : e))}
}

// withoutEdge removes one transition.
export function withoutEdge(flow: Flow, index: number): Flow {
    return {...flow, edges: flow.edges.filter((_, i) => i !== index)}
}

// condIsComplete says the condition can actually decide something — the engine
// refuses half-filled ones, so the editor keeps them out of what it saves.
export function condIsComplete(cond: EdgeCond | undefined): boolean {
    if (!cond) {
        return true
    }
    if (cond.commentContains) {
        return !cond.property && !cond.value
    }
    return Boolean(cond.property && cond.value)
}

// columnsOf lists the columns a route goes through, in the board's own order —
// the route reads the way the board does.
export function columnsOf(flow: Flow | undefined, columns: BoardColumn[]): BoardColumn[] {
    return columns.filter((c) => Boolean(nodeFor(flow, c)))
}

// AutomationChanges is what has to be written to make the registry look like
// the draft. Routes are keyed by name, so a renamed route is a new one and the
// old is dropped — which is also what happens to the cards that named it.
export type AutomationChanges = {
    savedColumns: ColumnSpec[]
    removedColumns: ColumnSpec[]
    addedFlows: Flow[]
    updatedFlows: Flow[]
    removedFlows: Flow[]
}

export function automationChanges(before: Automation, after: Automation): AutomationChanges {
    const sameFlowName = (a: Flow, b: Flow) => a.name.trim().toLowerCase() === b.name.trim().toLowerCase()
    return {
        savedColumns: after.columns.filter((c) => {
            const was = before.columns.find((b) => sameColumn(b, c))
            return !was || JSON.stringify(was) !== JSON.stringify(c)
        }),
        removedColumns: before.columns.filter((c) => !after.columns.some((n) => sameColumn(n, c))),
        addedFlows: after.flows.filter((f) => !before.flows.some((b) => sameFlowName(b, f))),
        updatedFlows: after.flows.filter((f) => {
            const was = before.flows.find((b) => sameFlowName(b, f))
            return Boolean(was) && JSON.stringify(was) !== JSON.stringify(f)
        }),
        removedFlows: before.flows.filter((f) => !after.flows.some((n) => sameFlowName(n, f))),
    }
}

// The board properties a template carries its automation in — the same names Go
// reads (internal/acp/boardseed.go). A board made from the template brings them,
// and the first look at it takes them into the registry.
// KEY_PREFIX is the namespace this application's own keys live under wherever
// they share a map with somebody else's — a board's `properties`, a card's
// `fields`. Spelled the same on the Go side (acp.KeyPrefix), and deliberately
// not the product's name: a name can be revisited, and after a release every
// key carrying one is a migration in its own store.
export const KEY_PREFIX = 'app'

export const BOARD_PROP_COLUMNS = 'columns'
export const BOARD_PROP_FLOWS = 'flows'
export const BOARD_PROP_SETUP = 'setup'

// What this board's agents are told first. Nothing here writes it — the page
// asks Go for it (GetBoardPrompts/SetBoardPrompts) and Go keeps it on the board —
// but it is named here because it is one of the keys a template carries, and
// anything that rebuilds a board's properties has to leave it alone.
export const BOARD_PROP_PROMPT = 'prompt'

// Which card property holds the folders, by id. A name would have been a
// worse answer twice over: the field is a person's to rename, and the name
// this app gives it is Russian, so a board in any other language could only
// ever have been matched by luck.
export const BOARD_PROP_PROJECT_PROPERTY = 'projectProperty'

// And which one this app writes a stage's outcome into — «прошло», «не прошло»,
// «заблокировано» — for every stage, without anybody declaring it. Written by
// the engine (acp.BoardPropOutcome); named here because the editor must not
// offer it as something a stage declares, the way it does not offer the column
// property: both are the app's own answer about the card.
export const BOARD_PROP_OUTCOME_PROPERTY = 'outcomeProperty'

// The values it takes, in the engine's own spelling (acp.OutcomeValue). Two,
// because what the card carries here is whether the step worked — and every
// question anybody asks of it is that question. A check that could not run
// answers «не прошло» like anything else that did not pass; why is the stall's
// business, where it can be a sentence.
export const OUTCOME_PASSED = 'прошло'
export const OUTCOME_FAILED = 'не прошло'
export const OUTCOME_VALUES = [OUTCOME_PASSED, OUTCOME_FAILED]

// boardOutcomeProperty is the id of that field, or '' for a board without one.
export function boardOutcomeProperty(board: {properties?: Record<string, unknown>}): string {
    const recorded = board.properties?.[BOARD_PROP_OUTCOME_PROPERTY]
    return typeof recorded === 'string' ? recorded : ''
}

// And which one holds the branch a card's work is on, by id for the same
// reason. Go writes into it (workspace.go); the page is what creates it,
// because the board's card properties are the page's to patch.
export const BOARD_PROP_BRANCH_PROPERTY = 'branchProperty'

// boardBranchProperty is the id of that field, or '' for a board without one.
export function boardBranchProperty(board: {properties?: Record<string, unknown>}): string {
    const recorded = board.properties?.[BOARD_PROP_BRANCH_PROPERTY]
    return typeof recorded === 'string' ? recorded : ''
}
// BoardSetupStep is one question the template asks the machine when a board is
// made from it. Only the kind and the order are the template's to choose: what
// each step does is the app's, so a template can never ask for something no
// registry can hold.
export type BoardSetupStep = {
    kind: string
    hint?: string
    required?: boolean
}

export type BoardSetup = {
    steps: BoardSetupStep[]
}

// SetupStepDef is a step this build can carry out, as Go lists them.
export type SetupStepDef = {
    kind: string
    registry?: string
    optional: boolean
}

function readProperty<T>(board: Board, key: string, fallback: T): T {
    const properties = board.properties || {}
    const raw = properties[key]
    if (raw === undefined || raw === null || raw === '') {
        return fallback
    }
    if (typeof raw === 'string') {
        try {
            return JSON.parse(raw) as T
        } catch {
            return fallback
        }
    }
    return raw as unknown as T
}

// readBoardAutomation is what a template board carries. A template is not run,
// so its own properties are the whole truth about it — unlike a live board,
// whose automation lives in the registry the moment it is first opened.
export function readBoardAutomation(board: Board): Automation {
    return {
        columns: readProperty<ColumnSpec[]>(board, BOARD_PROP_COLUMNS, []),
        flows: readProperty<Flow[]>(board, BOARD_PROP_FLOWS, []),
    }
}

export function readBoardSetup(board: Board): BoardSetup | undefined {
    const setup = readProperty<BoardSetup | undefined>(board, BOARD_PROP_SETUP, undefined)
    return setup && Array.isArray(setup.steps) ? setup : undefined
}

// writeBoardAutomation returns the board's properties with the automation in
// them. They are written as JSON text rather than as objects because that is
// what the board's property map is typed to hold, and Go reads either.
export function boardAutomationProperties(
    board: Board,
    automation: Automation,
    setup: BoardSetup | undefined,
): Record<string, string | string[]> {
    const properties = {...(board.properties || {})}

    properties[BOARD_PROP_COLUMNS] = JSON.stringify(automation.columns)
    properties[BOARD_PROP_FLOWS] = JSON.stringify(automation.flows)
    if (setup) {
        properties[BOARD_PROP_SETUP] = JSON.stringify(setup)
    } else {
        delete properties[BOARD_PROP_SETUP]
    }
    return properties
}

// impliedSetupSteps is what the app would ask for if the template said nothing
// — the same reasoning as Go's impliedSetup, and only ever used to fill the
// editor in when somebody chooses to name the steps by hand. Nothing runs off
// this copy: an undeclared template is still resolved on the Go side.
export function impliedSetupSteps(automation: Automation, defs: SetupStepDef[]): BoardSetupStep[] {
    const actions = new Set<string>()
    for (const column of automation.columns) {
        actions.add(column.action)
    }
    for (const flow of automation.flows) {
        for (const node of flow.nodes) {
            if (node.action) {
                actions.add(node.action)
            }
        }
    }
    const wanted = (kind: string) => {
        switch (kind) {
        case 'deploy':
            return actions.has('deploy')
        case 'browser':
            return actions.has('test')

        // Never implied, mirroring the Go side (impliedSetup in setup.go): no
        // arrangement of columns says cards should arrive by themselves. A
        // template that wants the step declares it by hand.
        case 'source':
            return false
        default:
            return true
        }
    }
    return defs.filter((d) => wanted(d.kind)).map((d) => ({kind: d.kind}))
}

// routeOptionMissing says the board has no option a card could name this route
// with. A route nothing can be put on is the commonest way an editor's work
// disappears: the graph saves, the cards never take it.
export function routeOptionMissing(board: Board, flow: Flow): boolean {
    const name = flow.name.trim().toLowerCase()
    if (!name) {
        return false
    }
    return !selectProperties(board).some((p) =>
        (p.options || []).some((o) => o.value.trim().toLowerCase() === name))
}

// resolvedWrites is what a stage actually leaves on the card: its own list, or
// the column's — the inheritance the route itself knows nothing about, spelled
// once so the canvas, the reads and the condition pickers cannot disagree.
export function resolvedWrites(node: FlowNode, specs: ColumnSpec[]): PropertyWrite[] {
    if (node.writes?.length) {
        return node.writes
    }
    return specForNode(node, specs)?.writes || []
}

// specForNode is the column a stage stands on — the option first, across the
// whole list, and only then the name. The two passes must not be merged into
// one `find`: a board that has both a spec bound to the option and an older one
// carrying only the column's name would then be answered by whichever came
// first, which is the order somebody edited things in. That is specFor's rule,
// said again for a node — specFor takes a BoardColumn, which a route has not.
export function specForNode(node: FlowNode, specs: ColumnSpec[]): ColumnSpec | undefined {
    return (node.optionId ? specs.find((c) => c.optionId === node.optionId) : undefined) ||
        specs.find((c) => c.column.toLowerCase() === node.column.toLowerCase())
}

// UpstreamWrite is one property a stage will find on the card when it starts,
// and the stage that put it there — which is the half that makes the answer
// readable: «Превью · пишет «Деплой»».
export type UpstreamWrite = {
    property: string
    from: string
}

// upstreamWrites is every property the stages ahead of this one declare. It is
// what a stage's reads fall back to (Go's FlowEntry.UpstreamWrites, kept in step
// with it), so declaring a write once carries the value on by itself.
//
// Walked backwards over the incoming edges with a visited set, because a route
// has cycles on purpose: a failed check sends the card back to the agent. A
// stage never reads its own output back.
export function upstreamWrites(flow: Flow, from: string, specs: ColumnSpec[]): UpstreamWrite[] {
    const nodes = new Map(flow.nodes.map((n) => [n.id, n]))
    const seen = new Set([from])
    const taken = new Set<string>()
    const out: UpstreamWrite[] = []
    const queue = [from]
    while (queue.length > 0) {
        const at = queue.shift() as string
        for (const edge of flow.edges) {
            if (edge.to !== at || seen.has(edge.from)) {
                continue
            }
            seen.add(edge.from)
            queue.push(edge.from)
            const node = nodes.get(edge.from)
            if (!node) {
                continue
            }
            for (const w of resolvedWrites(node, specs)) {
                const key = w.property.trim().toLowerCase()
                if (!key || taken.has(key)) {
                    continue
                }
                taken.add(key)
                out.push({property: w.property, from: node.column})
            }
        }
    }
    return out
}

// unwrittenConditions is the dataflow check: every conditional edge of the
// route whose property no stage of the route declares as a write. Not an
// error — an approved option may be a person's own click — but worth a sentence, since
// an edge waiting on a property nothing produces is a route that quietly never
// moves. The column property itself is exempt: moving between columns is what
// the route does, not a value somebody writes.
export function unwrittenConditions(flow: Flow, specs: ColumnSpec[], columnPropertyName?: string): string[] {
    const written = new Set<string>()
    for (const node of flow.nodes) {
        for (const w of resolvedWrites(node, specs)) {
            written.add(w.property.toLowerCase())
        }
    }
    const out = new Set<string>()
    for (const edge of flow.edges) {
        const property = edge.if?.property
        if (!property || property.toLowerCase() === columnPropertyName?.toLowerCase()) {
            continue
        }
        if (!written.has(property.toLowerCase())) {
            out.add(property)
        }
    }
    return [...out]
}
