import {Board, IPropertyOption, IPropertyTemplate} from './blocks/board'

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

// cardColumnProperty is the property whose options are the columns a new card
// stands in: the view's grouping when it is a select, else the board's first
// select — the same answer Go gives (boardmodel.ColumnPropertyName) — and
// never the folder, whose empty value is an answer of its own: no folder yet.
export function cardColumnProperty(board: Board, groupBy?: IPropertyTemplate): IPropertyTemplate | undefined {
    const folder = board.properties?.[BOARD_PROP_PROJECT_PROPERTY]
    const selects = selectProperties(board).filter((p) => p.id !== folder)
    if (groupBy && selects.some((p) => p.id === groupBy.id)) {
        return groupBy
    }
    return selects[0]
}

export function boardColumns(board: Board, propertyName?: string): BoardColumn[] {
    const property = columnProperty(board, propertyName)
    return (property?.options || []).map((o: IPropertyOption) => ({optionId: o.id, name: o.value, color: o.color}))
}

// The board's own keys, spelled as Go spells them
// (internal/acp/boardseed.go). A board made from a template brings them.

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
