import {For, createSignal} from 'solid-js'
import type {Component} from 'solid-js'
import {Dynamic} from '@solidjs/web'
import type {JSX} from '@solidjs/web'

import {Board, IPropertyTemplate} from './blocks/board'
import {BoardView} from './blocks/boardView'
import {Card} from './blocks/card'
import {IUser} from './user'
import type {IntlShape} from './intl'

// Where features built on top of the board plug into it. The board calls what
// is registered here and imports nothing that fills it, so it builds and runs
// with nothing registered at all; the features register themselves from
// main.tsx (docs/architecture.md, «The board knows nothing of the agents»).
//
// Three shapes: sources the board asks (people it knows besides its members),
// filters it runs a list through (card fields, assignees), and slots — named
// places in a component where registered components are drawn.

// People a board knows without them being its members: the agents a person
// has, which may be assigned a card on any board (docs/modes.md).
type BoardUsersSource = (boardId: string) => Promise<IUser[]>

const boardUsersSources: BoardUsersSource[] = []
const boardUsersListeners: Array<() => void> = []

export function addBoardUsersSource(source: BoardUsersSource): void {
    boardUsersSources.push(source)
}

export async function extraBoardUsers(boardId: string): Promise<IUser[]> {
    const found = await Promise.all(boardUsersSources.map((source) => source(boardId).catch(() => [] as IUser[])))
    return found.flat()
}

// A source says its answer changed (an agent was connected); the board that is
// open asks again.
export function boardUsersChanged(): void {
    boardUsersListeners.forEach((listener) => listener())
}

export function onBoardUsersChanged(listener: () => void): () => void {
    boardUsersListeners.push(listener)
    return () => {
        const at = boardUsersListeners.indexOf(listener)
        if (at >= 0) {
            boardUsersListeners.splice(at, 1)
        }
    }
}

// Card fields a feature owns and hides when it is off: the board's views draw
// what is left.
type CardPropertiesFilter = (board: Board, templates: IPropertyTemplate[]) => IPropertyTemplate[]

const cardPropertiesFilters: CardPropertiesFilter[] = []

export function addCardPropertiesFilter(filter: CardPropertiesFilter): void {
    cardPropertiesFilters.push(filter)
}

export function shownCardProperties(board: Board, templates: IPropertyTemplate[]): IPropertyTemplate[] {
    return cardPropertiesFilters.reduce((list, filter) => filter(board, list), templates)
}

// Who a person field may be set to, out of whom the board found.
type AssigneesFilter = (users: IUser[]) => Promise<IUser[]>

const assigneesFilters: AssigneesFilter[] = []

export function addAssigneesFilter(filter: AssigneesFilter): void {
    assigneesFilters.push(filter)
}

export async function assignableUsers(users: IUser[]): Promise<IUser[]> {
    let list = users
    for (const filter of assigneesFilters) {
        // eslint-disable-next-line no-await-in-loop
        list = await filter(list)
    }
    return list
}

// Defaults a new card is born with besides what the board's view gives it.
// Never over an answer already given: `given` is what the filter and the group
// a card was dropped into say, and what a source returns only fills the rest.
type NewCardDefaults = (board: Board, given: Record<string, string>, groupBy?: IPropertyTemplate) => Record<string, string>

const newCardDefaults: NewCardDefaults[] = []

export function addNewCardDefaults(source: NewCardDefaults): void {
    newCardDefaults.push(source)
}

export function withNewCardDefaults(board: Board, given: Record<string, string>, groupBy?: IPropertyTemplate): Record<string, string> {
    const extra = newCardDefaults.reduce((all, source) => ({...all, ...source(board, given, groupBy)}), {} as Record<string, string>)
    return {...extra, ...given}
}

// Dialogs a feature opens from a menu item. A menu closes when its item is
// clicked and takes what it drew with it, so the dialog is drawn here instead,
// by one host the app mounts (ExtensionDialogs, in app.tsx).
const [dialogs, setDialogs] = createSignal<Array<{id: number, draw: (close: () => void) => JSX.Element}>>([])
let nextDialog = 0

export function openExtensionDialog(draw: (close: () => void) => JSX.Element): void {
    const id = ++nextDialog
    setDialogs((open) => [...open, {id, draw}])
}

export function ExtensionDialogs() {
    return (
        <For each={dialogs()}>
            {(dialog) => dialog.draw(() => setDialogs((open) => open.filter((d) => d.id !== dialog.id)))}
        </For>
    )
}

// A card property a feature answers for in the card's dialog: what its row is
// called, how its value is drawn and edited, and a sentence under it. Every
// part is optional; what is not given is drawn the board's own way.
export type CardPropertyRow = {
    claims: (board: Board, template: IPropertyTemplate) => boolean

    // A class on the row, for what draws itself next to it (the tour).
    rowClass?: string

    // The row's name, drawn read-only: the property is the feature's record,
    // not a field to rename or delete.
    Name?: Component<{board: Board, card: Card, template: IPropertyTemplate}>

    // Read-only on top of what the board already says.
    readonly?: (board: Board, card: Card, template: IPropertyTemplate) => boolean

    Value?: Component<{board: Board, card: Card, template: IPropertyTemplate, readonly: boolean}>

    // Inside the row, after the value.
    InRow?: Component<{board: Board, card: Card, template: IPropertyTemplate}>

    // Under the row.
    Below?: Component<{board: Board, card: Card, template: IPropertyTemplate}>
}

const cardPropertyRows: CardPropertyRow[] = []

export function addCardPropertyRow(row: CardPropertyRow): void {
    cardPropertyRows.push(row)
}

export function cardPropertyRow(board: Board, template: IPropertyTemplate): CardPropertyRow | undefined {
    return cardPropertyRows.find((row) => row.claims(board, template))
}

// A panel beside a card in its dialog, opened by a button of its own at the
// top of the dialog, the way History and Comments are.
export type CardSidePanel = {
    id: string

    // The button; `active` says its panel is the one open.
    Button: Component<{board: Board, card: Card, active: boolean, toggle: () => void}>
    Panel: Component<{card: Card, onClose: () => void}>
}

const cardSidePanels: CardSidePanel[] = []

export function addCardSidePanel(panel: CardSidePanel): void {
    cardSidePanels.push(panel)
}

export function registeredCardSidePanels(): CardSidePanel[] {
    return cardSidePanels
}

// A section of the settings dialog. `first` puts it right after «The app
// itself»; otherwise it comes last.
export type SettingsSection = {
    id: string
    name: (intl: IntlShape) => string
    when: () => boolean
    body: Component<{onClose?: () => void}>
    first?: boolean
}

const settingsSections: SettingsSection[] = []

export function addSettingsSection(section: SettingsSection): void {
    settingsSections.push(section)
}

export function registeredSettingsSections(): SettingsSection[] {
    return settingsSections
}

// The named places a feature can draw into, and what each one is given.
export type SlotProps = {

    // Over the whole window, outside the router: what is worth saying wherever
    // in the app the person is.
    app: object

    // On a card on a board, under its properties.
    cardBadges: {
        board: Board
        card: Card
        readonly: boolean
        groupId: string
        visiblePropertyIds: string[]
        showCard: (cardId?: string) => void
    }

    // Under a card's title in its dialog, above its properties.
    cardDetailTop: {board: Board, card: Card, readonly: boolean}

    // After a card's properties in its dialog, before «Add a property».
    cardPropertiesEnd: {board: Board, card: Card, readonly: boolean}

    // Items of a board's ⋯ menu, before Export. Not drawn on the inbox.
    boardMenu: {board: Board, activeView: BoardView}

    // Items of the inbox's ⋯ menu, after «Sources…».
    inboxMenu: {board: Board}

    // Items of the «New» button's menu, after the card templates.
    newCardMenu: {board: Board}

    // Under the board's header, above its view. `readFirst` is the board's
    // own «Where to start» card, when it has one.
    boardHeader: {
        board: Board
        cards: Card[]
        columnPropertyId?: string
        readFirst?: {title: string, open: () => void}
    }
}

const slots: {[K in keyof SlotProps]?: Array<Component<SlotProps[K]>>} = {}

export function addToSlot<K extends keyof SlotProps>(slot: K, component: Component<SlotProps[K]>): void {
    const list = (slots[slot] || []) as Array<Component<SlotProps[K]>>
    list.push(component)
    slots[slot] = list as typeof slots[K]
}

export function hasSlot(slot: keyof SlotProps): boolean {
    return (slots[slot]?.length || 0) > 0
}

// ExtensionSlot draws what is registered under a name, in the order it was
// registered, and nothing when nothing is.
export function ExtensionSlot<K extends keyof SlotProps>(props: {name: K} & SlotProps[K]) {
    const list = () => (slots[props.name] || []) as Array<Component<SlotProps[K]>>
    return (
        <For each={list()}>
            {(component) => (
                <Dynamic
                    component={component as Component<Record<string, unknown>>}
                    {...(props as unknown as Record<string, unknown>)}
                />
            )}
        </For>
    )
}

// For tests: forget everything registered.
export function resetExtensions(): void {
    boardUsersSources.length = 0
    boardUsersListeners.length = 0
    cardPropertiesFilters.length = 0
    assigneesFilters.length = 0
    newCardDefaults.length = 0
    setDialogs([])
    cardSidePanels.length = 0
    cardPropertyRows.length = 0
    settingsSections.length = 0
    for (const key of Object.keys(slots) as Array<keyof SlotProps>) {
        delete slots[key]
    }
}
