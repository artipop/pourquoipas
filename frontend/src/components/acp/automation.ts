import type {IntlShape} from '../../intl'

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
    // brief on the review column, the builder's on the in-progress column.
    prompt?: string

    // What working here is expected to put on the card, and what it is handed
    // off the card on the way in.
    writes?: PropertyWrite[]
    reads?: string[]

    // The tools working in this column comes with, handed to whichever agent
    // works here on top of the servers that agent carries of its own.
    mcpServers?: MCPServers
}

// PropertyWrite is one property a stage puts on the card. The value is not
// here: an agent stage supplies it through finish_work (refused without it
// when required), a deploy stage writes its preview URL, a test its verdict.
export type PropertyWrite = {
    property: string
    required?: boolean
}

// Automation is a board's whole answer: what its columns do. It is what a
// template carries and what the board keeps in its own properties.
export type Automation = {
    columns: ColumnSpec[]
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

// ASK_WORKDIR is the folder the card's work happens in, written into the
// board's own folder field. It is an ASK_PROPERTY in everything but which
// property: which field a card names its folder in is the board's record
// rather than a person's to pick. Asked before ASK_WORKMODE, because a card
// that has not chosen a folder has nothing for that question to be about.
export const ASK_WORKDIR = 'workdir'

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

// AutomationChanges is what a save has to send: the columns that changed, and
// the ones that stopped being configured at all.
export type AutomationChanges = {
    savedColumns: ColumnSpec[]
    removedColumns: ColumnSpec[]
}

export function automationChanges(before: Automation, after: Automation): AutomationChanges {
    return {
        savedColumns: after.columns.filter((c) => {
            const was = before.columns.find((b) => sameColumn(b, c))
            return !was || JSON.stringify(was) !== JSON.stringify(c)
        }),
        removedColumns: before.columns.filter((c) => !after.columns.some((n) => sameColumn(n, c))),
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
    const wanted = (kind: string) => {
        switch (kind) {
        case 'deploy':
            return actions.has('deploy')
        case 'browser':
            return actions.has('test')
        default:
            return true
        }
    }
    return defs.filter((d) => wanted(d.kind)).map((d) => ({kind: d.kind}))
}

// routeOptionMissing says the board has no option a card could name this route
// with. A route nothing can be put on is the commonest way an editor's work
// disappears: the graph saves, the cards never take it.
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
