import {Board, IPropertyOption, IPropertyTemplate} from '../../blocks/board'

// What this application keeps on a board: the instructions it gives its agents,
// the questions it asks when it is set up, and which of the board's own
// properties hold the card's folder, branch and outcome.
//
// It used to keep what each column does and where a card goes next as well.
// That is gone: a column is an option of the board's own select and means what
// its name says, a card is worked by the agent it is assigned to, and the agent
// moves it on itself.

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

export function boardColumns(board: Board, propertyName?: string): BoardColumn[] {
    const property = columnProperty(board, propertyName)
    return (property?.options || []).map((o: IPropertyOption) => ({optionId: o.id, name: o.value, color: o.color}))
}

// The board's own keys, spelled as Go spells them
// (internal/acp/boardseed.go). A board made from a template brings them.
//
// BOARD_PROP_SETUP is the questions a board asks the machine when one is made
// from it: which steps, in which order, with the template's own hint beside
// each.
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

export function readBoardSetup(board: Board): BoardSetup | undefined {
    const setup = readProperty<BoardSetup | undefined>(board, BOARD_PROP_SETUP, undefined)
    return setup && Array.isArray(setup.steps) ? setup : undefined
}

// boardAutomationProperties returns the board's properties with the setup
// questions in them. They are written as JSON text rather than as objects
// because that is what the board's property map is typed to hold, and Go reads
// either.
export function boardAutomationProperties(
    board: Board,
    setup: BoardSetup | undefined,
): Record<string, string | string[]> {
    const properties = {...(board.properties || {})}

    if (setup) {
        properties[BOARD_PROP_SETUP] = JSON.stringify(setup)
    } else {
        delete properties[BOARD_PROP_SETUP]
    }
    return properties
}

// impliedSetupSteps is what the app asks for when a template says nothing, and
// it is only ever used to fill the editor in when somebody chooses to name the
// steps by hand. Nothing runs off this copy: an undeclared template is resolved
// on the Go side.
export function impliedSetupSteps(defs: SetupStepDef[]): BoardSetupStep[] {
    return defs.map((d) => ({kind: d.kind}))
}
