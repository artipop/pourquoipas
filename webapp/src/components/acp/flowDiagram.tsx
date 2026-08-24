import {For, Show, createEffect, createMemo, createSignal, onCleanup} from 'solid-js'
import {
    Background,
    BaseEdge,
    EdgeLabel,
    EdgeReconnectAnchor,
    Connection,
    Controls,
    Edge,
    EdgeProps,
    Handle,
    MarkerType,
    Node,
    NodeProps,
    Position,
    SolidFlow,
    createEdgeStore,
    createNodeStore,
    getSmoothStepPath,
    useSolidFlow,
    useViewport,
} from '@dschz/solid-flow'

import {useIntl, IntlShape} from '../../intl'

import CompassIcon from '../../widgets/icons/compassIcon'

import {BoardColumn, CARD_CHANGED, EdgeCond, FlowEdge, FlowNode, FlowTrigger, OUTCOME_PASSED, SUCCESS, FAILURE, nodeId} from './automation'

import '@dschz/solid-flow/dist/style.css'
import './flowDiagram.scss'

// The route as a graph. Pan, zoom and drag are Solid Flow's; the layout is ours.
//
// Every box is a column of the board, including the ones the route does not go
// through — drawn faded, and joining one to the route is what puts it on the
// route. There is no "add a stage, then choose its column": a stage cards can
// stand on is a column, and the picture says so.

export const NODE_WIDTH = 190
export const NODE_HEIGHT = 58

// How much taller a stage stands when it declares outputs: one row of chips,
// which are also the ports a conditional arrow is pulled from.
export const WRITE_ROW = 22

// HANDLE_WRITE is the prefix of a data port's handle id — the property's own
// name follows it. A prefix rather than a second field because a handle id is
// the only thing a connection carries back.
export const HANDLE_WRITE = 'write:'
const GAP_X = 80
const GAP_Y = 24

// How far under the route the columns it does not use are parked, and how
// close together — a shelf, so tighter than the graph above it.
const SPARE_GAP_Y = 90
const SPARE_GAP_X = GAP_X / 2

// StageCount is how many cards stand on a stage right now — the viewer's half
// of the canvas.
export type StageCount = {
    nodeId: string
    cards: number
    running: number
    queued: number
}

type Props = {
    nodes: FlowNode[]
    edges: FlowEdge[]
    triggers: FlowTrigger[]

    // spare are the board's other columns: on the canvas, faded, so the route
    // is drawn against the board it runs on rather than against a blank sheet.
    spare?: BoardColumn[]

    // counts turn the picture into a map of the board: how many cards are on
    // each stage, and how many of them are moving.
    counts?: StageCount[]

    // actionOf says what a stage does when its own action is empty — the
    // column's, which is where the behaviour actually lives.
    actionOf?: (node: FlowNode) => string

    // crewOf is who works the stage, for the second line of the box.
    crewOf?: (node: FlowNode) => string[]

    // writesOf is what the stage leaves on the card — the chips on the box, and
    // the ports a conditional arrow is pulled from. Resolved by the caller,
    // which is the half that knows a stage inherits its column's outputs and
    // which board properties have a closed set.
    writesOf?: (node: FlowNode) => StageWrite[]

    // The name of the field a person answers a stage in. Transitions that ask
    // about it are drawn on the box rather than as arrows — see isOutcomeEdge.
    outcomeProperty?: string

    // onChange makes it a builder rather than a picture: stages are dragged,
    // joined by pulling from an output, and removed with the keyboard. Absent
    // means the route is only being looked at.
    onChange?: (nodes: FlowNode[], edges: FlowEdge[]) => void

    // onAddColumn is a faded column being joined to the route — by a click on
    // it, an arrow drawn to or from it, or a drag that carries it into the
    // graph (`at` is then where it was dropped).
    onAddColumn?: (column: BoardColumn, at?: {x: number, y: number}) => void

    // onDropBlock is a palette block landing on the canvas: a new column of
    // this kind, at this point. Absent means there is no palette.
    onDropBlock?: (kind: string, at: {x: number, y: number}) => void

    // selected/onSelect drive the inspector beside the canvas: what is selected
    // is what the panel is about.
    selected?: {kind: 'node' | 'edge', id: string} | null
    onSelect?: (selection: {kind: 'node' | 'edge', id: string} | null) => void
}

// StageWrite is one property the stage leaves on the card, as the box draws it.
// `branchable` is whether an arrow can ask about it, which is a question only a
// closed set can answer — so a text output is drawn and not connectable.
export type StageWrite = {
    name: string
    branchable: boolean
}

// isOutcomeEdge says this transition is a person answering on the card rather
// than the stage reporting for itself. It is drawn like any other outcome —
// green on, red back, out of the same sockets — because that is what it is: on
// a stage where nothing runs, these two *are* the success and the failure, and
// they were the only two arrows a person ever looked for on «На ревью».
//
// What it is not is a wait. It rides card.changed, and colouring by the trigger
// drew it grey and dashed among «ветка влита в основную», which is how the way
// back from a review managed to be on the canvas and invisible.
// sameFold is two names being the same name — trimmed and case-insensitive,
// the way every match on a person's word in this editor is.
function sameFold(a: string | undefined, b: string): boolean {
    return (a || '').trim().toLowerCase() === b.trim().toLowerCase()
}

export function isOutcomeEdge(edge: FlowEdge, outcomeProperty: string | undefined): boolean {
    return Boolean(outcomeProperty && edge.on === CARD_CHANGED && edge.if?.property &&
        edge.if.property.toLowerCase() === outcomeProperty.toLowerCase())
}

type StageData = {
    column: string
    action: string
    actionLabel: string
    crew?: string[]
    count?: StageCount
    writes?: StageWrite[]
    editable?: boolean
    spare?: boolean
    selected?: boolean
}

// stageHeight is how tall this stage stands: the base, plus the row of outputs
// when it has any. Stated rather than measured, like everything else about the
// box — the arrows are drawn on the first paint.
export function stageHeight(writes?: StageWrite[]): number {
    return NODE_HEIGHT + ((writes && writes.length > 0) ? WRITE_ROW : 0)
}

// LANE is how far apart two long-haul arrows are pushed. An arrow that skips
// columns runs the width of the graph, and several of them at the same height
// read as one thick line — which is what «дубли линий» turned out to be: a
// failure arrow from one stage and a return arrow from another, travelling
// side by side into the same box.
const LANE = 26

// How far under the lowest of the two stages the first lane runs — clear of a
// box, which is 58 tall and 78 with outputs on it.
const LANE_BASE = 48

// longHaul says the arrow needs a corridor of its own rather than the direct
// route between two boxes: anything going backwards — a failed check returning
// the card to the agent — and anything jumping past a stage. A step to the next
// stage is the one case that never crosses anything.
//
// Backwards counts however short it is. An arrow one column back still has to
// travel the width of both boxes to get there, and the library draws it through
// them: opaque boxes then cut it into pieces, which reads as two broken stubs
// rather than as one arrow.
export function longHaul(nodes: FlowNode[], edges: FlowEdge[]): Map<string, number> {
    const depth = depths(nodes, edges)
    const out = new Map<string, number>()
    let lane = 0
    edges.forEach((edge, index) => {
        const from = depth.get(edge.from)
        const to = depth.get(edge.to)
        if (from === undefined || to === undefined) {
            return
        }
        if (to > from + 1) {
            // forward, past at least one stage
        } else if (to > from) {
            return // the next stage along: the direct route crosses nothing
        }
        out.set(edgeId(edge, index), lane)
        lane += 1
    })
    return out
}

// forward drops the transitions that lead back the way the card came — a failed
// check returning to the agent is a normal route, and laying it out as progress
// would push every later stage off to the right for ever.
function forward(nodes: FlowNode[], edges: FlowEdge[]): FlowEdge[] {
    const out = new Map<string, FlowEdge[]>(nodes.map((n) => [n.id, []]))
    for (const edge of edges) {
        if (out.has(edge.from) && out.has(edge.to) && edge.from !== edge.to) {
            (out.get(edge.from) as FlowEdge[]).push(edge)
        }
    }

    const kept: FlowEdge[] = []
    const state = new Map<string, 'open' | 'done'>()

    // Depth-first, iteratively: an edge into a stage still on the stack closes
    // a loop and is left out of the layout.
    const walk = (start: string) => {
        const stack: Array<{id: string, next: number}> = [{id: start, next: 0}]
        state.set(start, 'open')
        while (stack.length > 0) {
            const top = stack[stack.length - 1]
            const outgoing = out.get(top.id) as FlowEdge[]
            if (top.next >= outgoing.length) {
                state.set(top.id, 'done')
                stack.pop()
                continue
            }
            const edge = outgoing[top.next++]
            if (state.get(edge.to) === 'open') {
                continue
            }
            kept.push(edge)
            if (!state.has(edge.to)) {
                state.set(edge.to, 'open')
                stack.push({id: edge.to, next: 0})
            }
        }
    }

    // Start where a card would: at the stages nothing leads to, then at
    // whatever the walk has not reached (a route that is one closed loop).
    const reached = new Set(edges.map((e) => e.to))
    for (const node of nodes.filter((n) => !reached.has(n.id))) {
        walk(node.id)
    }
    for (const node of nodes) {
        if (!state.has(node.id)) {
            walk(node.id)
        }
    }
    return kept
}

// depths puts every stage as far right as the longest path that reaches it, so
// an arrow points forward wherever the route does.
export function depths(nodes: FlowNode[], edges: FlowEdge[]): Map<string, number> {
    const depth = new Map<string, number>(nodes.map((n) => [n.id, 0]))
    const dag = forward(nodes, edges)
    for (let pass = 0; pass < nodes.length; pass++) {
        let moved = false
        for (const edge of dag) {
            const next = (depth.get(edge.from) as number) + 1
            if ((depth.get(edge.to) as number) < next) {
                depth.set(edge.to, next)
                moved = true
            }
        }
        if (!moved) {
            break
        }
    }
    return depth
}

// layout places the stages in columns of equal depth, keeping the editor's own
// order within a column.
export function layout(nodes: FlowNode[], edges: FlowEdge[], rowHeight = NODE_HEIGHT): Map<string, {x: number, y: number}> {
    const depth = depths(nodes, edges)
    const taken = new Map<number, number>()
    const out = new Map<string, {x: number, y: number}>()
    for (const node of nodes) {
        // A stage that was placed by hand stays where it was put.
        if (node.x !== undefined && node.y !== undefined) {
            out.set(node.id, {x: node.x, y: node.y})
            continue
        }
        const column = depth.get(node.id) || 0
        const row = taken.get(column) || 0
        taken.set(column, row + 1)
        out.set(node.id, {
            x: column * (NODE_WIDTH + GAP_X),
            y: row * (rowHeight + GAP_Y),
        })
    }
    return out
}

// spareLayout parks the board's unused columns in a row under the route, in the
// board's own order. They are a shelf, not part of the graph, so they are laid
// out by counting rather than by following arrows.
export function spareLayout(
    placed: Map<string, {x: number, y: number}>,
    spare: BoardColumn[],
    lanes = 0,
): Map<string, {x: number, y: number}> {
    let bottom = 0
    for (const position of placed.values()) {
        bottom = Math.max(bottom, position.y + NODE_HEIGHT)
    }

    // Below the corridors, not below the boxes: a long arrow runs under the
    // graph, and the shelf parked at the boxes' feet had those arrows — and
    // their captions — drawn straight across it.
    if (lanes > 0) {
        bottom += LANE_BASE + ((lanes - 1) * LANE)
    }
    const out = new Map<string, {x: number, y: number}>()
    spare.forEach((column, i) => {
        out.set(nodeId(column), {
            x: i * (NODE_WIDTH + SPARE_GAP_X),
            y: bottom + SPARE_GAP_Y,
        })
    })
    return out
}

// An arrow and its head must be one colour, and the head is drawn from a shared
// SVG marker that no class of ours can reach — so the colours are literals here
// rather than CSS variables. They are picked to read on a light and a dark
// board alike.
const EDGE_COLOR: Record<string, string> = {
    success: '#3db887',
    failure: '#d24b4e',
    event: '#8b8d94',
}

// edgeKind styles a transition by what produces it: the stage's own outcome, or
// something that happened in the project.
export function edgeKind(on: string): string {
    if (on === SUCCESS) {
        return 'success'
    }
    if (on === FAILURE) {
        return 'failure'
    }
    return 'event'
}

// edgeId names a transition on the canvas. Conditions allow several edges per
// (from, on), so the identity is the edge's index in the route — which is also
// how the inspector addresses it.
export function edgeId(edge: FlowEdge, index: number): string {
    return `${edge.from}-${edge.on}-${index}`
}

// edgeIndexOf reads the index back out of a canvas id.
export function edgeIndexOf(id: string): number {
    const at = id.lastIndexOf('-')
    return at < 0 ? -1 : Number(id.slice(at + 1))
}

// The three ways out of a stage. A route is drawn by pulling from one of them
// to another stage, so the handle carries the meaning of the transition and
// nothing has to be chosen afterwards — except which event, for the third.
export const HANDLE_SUCCESS = 'success'
export const HANDLE_FAILURE = 'failure'
export const HANDLE_EVENT = 'event'

// StageNode is one column: what runs when a card lands there, who works it,
// and — on a board being watched rather than edited — how many cards stand on
// it. A faded one is a column the route does not use yet.
const StageNode = (props: NodeProps) => {
    const intl = useIntl()
    const data = () => props.data as StageData
    const count = () => data().count
    const classes = () => [
        'FlowDiagram__stage',
        `FlowDiagram__stage--${data().action || 'none'}`,
        data().spare ? 'FlowDiagram__stage--spare' : '',
        data().selected ? 'FlowDiagram__stage--selected' : '',
    ].filter(Boolean).join(' ')

    return (
        <div class={classes()}>
            <Handle
                type='target'
                position='left'
                isConnectable={Boolean(data().editable)}
            />
            <div class='FlowDiagram__column'>{data().column || '—'}</div>
            <div class='FlowDiagram__action'>
                {data().actionLabel}
                <Show when={data().crew && data().crew!.length > 0}>
                    <span class='FlowDiagram__crew'>{` · ${data().crew!.join(', ')}`}</span>
                </Show>
            </div>
            <Show when={count() && count()!.cards > 0}>
                <span class='FlowDiagram__count'>
                    {count()!.cards}
                    <Show when={count()!.running > 0}>
                        <span class='FlowDiagram__running'><CompassIcon icon='play'/></span>
                    </Show>
                    <Show when={count()!.queued > 0}>
                        <span class='FlowDiagram__queued'><CompassIcon icon='pause'/></span>
                    </Show>
                </span>
            </Show>
            <Handle
                id={HANDLE_SUCCESS}
                type='source'
                position='right'
                style={{top: '30%'}}
                class='FlowDiagram__out FlowDiagram__out--success'
                isConnectable={Boolean(data().editable)}
            />
            <Handle
                id={HANDLE_FAILURE}
                type='source'
                position='right'
                style={{top: '70%'}}
                class='FlowDiagram__out FlowDiagram__out--failure'
                isConnectable={Boolean(data().editable)}
            />
            <Handle
                id={HANDLE_EVENT}
                type='source'
                position='bottom'
                class='FlowDiagram__out FlowDiagram__out--event'
                isConnectable={Boolean(data().editable)}
            />

            {/* What the stage leaves on the card, and — for a property with a
                closed set — the port a fork is pulled from. This is where the
                dataflow becomes visible: the arrow that asks «Вердикт = fail»
                is drawn from the thing that produces «Вердикт», rather than
                being assembled in a panel out of a list of every board field.

                A text output is drawn and not connectable, because an arrow
                asks "is it this value" and only a set has values to offer. */}
            <Show when={data().writes && data().writes!.length > 0}>
                <div class='FlowDiagram__writes'>
                    <For each={data().writes}>
                        {(w) => (
                            <span
                                class={`FlowDiagram__write${w.branchable ? ' FlowDiagram__write--branchable' : ''}`}
                                title={w.branchable ? undefined : intl.formatMessage({id: 'FlowDiagram.write-not-branchable', defaultMessage: '{property} travels to the stages further on. To branch on it, give the property a set of values.'}, {property: w.name})}
                            >
                                {w.name}
                                <Show when={w.branchable && data().editable}>
                                    <Handle
                                        id={HANDLE_WRITE + w.name}
                                        type='source'
                                        position='bottom'
                                        class='FlowDiagram__out FlowDiagram__out--write'
                                        isConnectable={true}
                                    />
                                </Show>
                            </span>
                        )}
                    </For>
                </div>
            </Show>
        </div>
    )
}

const nodeTypes = {stage: StageNode}

// The ring on an arrow's end, in screen pixels: GRAB is what the pointer has to
// land in, MARK is what a person sees. Two numbers because one is a target and
// the other is a mark on a picture — a ring drawn as big as it needs to be
// grabbed sat on the arrow like a bead.
//
// Both are divided by the zoom, because the canvas draws in flow units and a
// stated size shrinks with the picture: measured, 14 units came out 11px on a
// route fitted at 0.78 and 4px at minZoom, which is a thing to grab that a hand
// cannot land on.
const GRAB = 20
const MARK = 9

// How thick the mark's own ring is drawn, in screen pixels — divided like the
// rest of it, or the outline thins away as the picture is zoomed out.
const MARK_RING = 2

// lanePath is a long arrow's own way round: out of the handle, down into a
// corridor of its own under the graph, across, and up into the target. Written
// by hand because the corridor is the point — getSmoothStepPath puts every
// arrow between the same two rows at the same height, whatever `offset` says
// (measured: offset widens the stubs and never moves the crossing), so two long
// arrows travelled the picture side by side and read as one thick line.
export function lanePath(sx: number, sy: number, tx: number, ty: number, lane: number): {d: string, labelX: number, labelY: number} {
    const out = 18
    const y = Math.max(sy, ty) + LANE_BASE + (lane * LANE)
    const x1 = sx + out
    const x2 = tx - out
    return {
        d: `M ${sx},${sy} L ${x1},${sy} L ${x1},${y} L ${x2},${y} L ${x2},${ty} L ${tx},${ty}`,
        labelX: (x1 + x2) / 2,
        labelY: y,
    }
}

// LaneEdge draws a short arrow the way the library would and a long one through
// a lane of its own. It exists at all because the library's edge wrapper
// forwards a fixed list of props, and nothing that shapes the path is on it.
const LaneEdge = (props: EdgeProps) => {
    const path = createMemo(() => {
        const lane = (props.data as {lane?: number} | undefined)?.lane
        if (lane !== undefined) {
            return lanePath(props.sourceX, props.sourceY, props.targetX, props.targetY, lane)
        }
        const [d, labelX, labelY] = getSmoothStepPath({
            sourceX: props.sourceX,
            sourceY: props.sourceY,
            targetX: props.targetX,
            targetY: props.targetY,
            sourcePosition: props.sourcePosition,
            targetPosition: props.targetPosition,
        })
        return {d, labelX, labelY}
    })

    // A route being read has no way to redraw itself, so the anchors are the
    // canvas' own answer to whether this is a builder: `editable` rides in the
    // edge's data, the way a stage's does. Nothing but data is put there — the
    // canvas owns that store and copies it, and a callback in one is a function
    // surviving a proxy by luck.
    const editable = () => Boolean((props.data as {editable?: boolean} | undefined)?.editable)

    const viewport = useViewport()
    const zoom = () => viewport().zoom || 1
    const mark = () => ({
        width: `${MARK / zoom()}px`,
        height: `${MARK / zoom()}px`,
        '--mark-ring': `${MARK_RING / zoom()}px`,
    })
    return (
        <>
            <BaseEdge
                path={path().d}
                style={props.style}
                markerEnd={props.markerEnd}
                interactionWidth={props.interactionWidth}
            />
            {/* How an arrow is broken or re-pointed: grab its end and drag.
                Dropping it on another stage moves the arrow there; dropping it
                on nothing at all breaks it (onReconnectEnd). It is per-end
                rather than per-socket on purpose — a socket holds as many
                arrows as the stage forks into, so a control on the socket could
                not say which one it meant. */}
            <Show when={props.selected && editable()}>
                <EdgeReconnectAnchor
                    type='source'
                    class='FlowDiagram__anchor'
                    size={GRAB / zoom()}
                    position={{x: props.sourceX, y: props.sourceY}}
                >
                    <span
                        class='FlowDiagram__anchorMark'
                        style={mark()}
                    />
                </EdgeReconnectAnchor>
                <EdgeReconnectAnchor
                    type='target'
                    class='FlowDiagram__anchor'
                    size={GRAB / zoom()}
                    position={{x: props.targetX, y: props.targetY}}
                >
                    <span
                        class='FlowDiagram__anchorMark'
                        style={mark()}
                    />
                </EdgeReconnectAnchor>
            </Show>

            <Show when={props.label}>
                <EdgeLabel
                    x={path().labelX}
                    y={path().labelY}
                    style={props.labelStyle}
                >
                    <span class='FlowDiagram__edgeLabel'>{props.label}</span>
                </EdgeLabel>
            </Show>
        </>
    )
}

const edgeTypes = {lane: LaneEdge}

// The palette's drags travel as this content type, so a file dragged from the
// desktop is not mistaken for a block.
export const BLOCK_DRAG_TYPE = 'application/x-xciii-block'

// FlowHandle is the part of the canvas API this component reaches for: turning
// a drop point into graph coordinates, and re-fitting the picture when what is
// on the canvas changes. `fitView` is optional because the handle is also read
// under jsdom, where the canvas measures nothing and has nothing to fit.
type FlowHandle = {
    screenToFlowPosition: (p: {x: number, y: number}) => {x: number, y: number}
    fitView?: (options?: {padding?: number, maxZoom?: number, nodes?: Array<{id: string}>}) => Promise<boolean> | void
    getInternalNode?: (id: string) => {measured?: {width?: number, height?: number}} | undefined
}

// How the graph sits in the canvas: the whole picture, with a margin small
// enough that the boxes are the point and the emptiness around them is not.
//
// maxZoom is 1, which is "the box at the size it was drawn". It was 1.5, and a
// route of three stages was magnified half again to fill the canvas — boxes
// that size read as a mistake rather than as a short route, and the same stage
// changed size depending on which tab it was on. The margin is the second half
// of that: a picture pressed against the edges of its frame reads as one that
// did not fit.
const FIT_VIEW = {padding: 0.16, maxZoom: 1}

// How many frames a re-fit waits for the canvas to measure the boxes it has
// just been handed. Generous, because being late costs nothing and being early
// costs the whole picture — see fitWhenMeasured.
const FIT_FRAMES = 30

// fitWhenMeasured waits for the canvas to have measured every box before
// fitting to them, and this is the whole of why a re-fit is not one line.
// `fitView` fits to what the canvas *knows*, and the canvas learns a node's
// size from a resize observer a frame or more after the node is handed to it —
// so fitting straight away fits to boxes of no size, which on the columns tab
// came out at the maximum zoom with two thirds of the grid off the right-hand
// edge. Bounded, because a box that is never measured (a canvas with no
// layout, as in jsdom) must not spin for ever: after FIT_FRAMES it fits to
// whatever there is, which is what it would have done anyway.
//
// `live` is asked before every frame, because a fit that was asked for is not
// owed once the picture it was about is gone. Each re-fit is a chain of
// frames, and a chain that outlived its tab waited its thirty frames for boxes
// no longer on the canvas and then fitted to none of them — a viewport of
// nothing, landing half a second after the switch. `done` is told when the
// fit has landed, which is what the canvas waits for before it is drawn.
const fitWhenMeasured = (handle: FlowHandle, ids: string[], live: () => boolean, done: () => void, frame = 0) => {
    if (!live()) {
        return
    }
    const measured = ids.every((id) => (handle.getInternalNode?.(id)?.measured?.width || 0) > 0)
    if (!measured && frame < FIT_FRAMES) {
        requestAnimationFrame(() => fitWhenMeasured(handle, ids, live, done, frame + 1))
        return
    }

    // To the route and not to the canvas: the board's other columns are parked
    // on a shelf under the graph, and fitting to them too pushed a short route
    // — «Хотфикс» is three stages — into a corner of a picture mostly made of
    // columns it does not use. The shelf is there to be dragged from, not to be
    // read, so it does not decide the zoom.
    void handle.fitView?.({...FIT_VIEW, nodes: ids.map((id) => ({id}))})
    done()
}

// CanvasHook runs inside the canvas' context and hands its API out: the drop
// events land on the wrapper div, which is outside, and converting a drop
// point into graph coordinates is the canvas' own knowledge.
const CanvasHook = (props: {onReady: (flow: FlowHandle) => void}) => {
    props.onReady(useSolidFlow() as unknown as FlowHandle)
    return null
}

// connectEdge is what pulling a connection means: the handle says which
// transition it is, and an event connection takes the first trigger the stage
// does not already wait for — the inspector is where it is changed.
export function connectEdge(
    edges: FlowEdge[],
    from: string,
    to: string,
    handle: string | null | undefined,
    waitTriggers: FlowTrigger[],
): FlowEdge[] {
    if (!from || !to || from === to) {
        return edges
    }

    // A connection pulled from a data port is the fork this whole row exists
    // for: "the card goes here depending on that property". The value is left
    // empty and the caption says so, because which value is the one thing the
    // gesture cannot carry — it is chosen where the arrow is, in the panel.
    if (handle && handle.startsWith(HANDLE_WRITE)) {
        const property = handle.slice(HANDLE_WRITE.length)
        return [...edges, {from, to, on: SUCCESS, if: {property, value: ''}}]
    }

    let on = handle || HANDLE_SUCCESS
    if (on === HANDLE_EVENT) {
        // card.changed is never auto-picked: it is meaningless without its
        // condition, which the inspector is where to state.
        const used = new Set(edges.filter((e) => e.from === from).map((e) => e.on))
        const free = waitTriggers.find((t) => t.kind !== CARD_CHANGED && !used.has(t.kind))
        if (!free) {
            return edges
        }
        on = free.kind
    }

    // Pulling the same output again redraws the unconditional edge; the
    // conditional ones are the fork's branches and stay as they are.
    return [...edges.filter((e) => !(e.from === from && e.on === on && !e.if)), {from, to, on}]
}

// condLabel is a condition as an edge caption: the question, not a sentence.
export function condLabel(intl: IntlShape, cond: EdgeCond | undefined): string {
    if (!cond) {
        return ''
    }
    if (cond.commentContains) {
        return intl.formatMessage({id: 'FlowDiagram.cond-comment', defaultMessage: 'agent said "{text}"'}, {text: cond.commentContains})
    }
    if (!cond.value) {
        // A fork pulled from a data port arrives without its value, and an
        // arrow captioned «Вердикт = » reads as a bug rather than as a question
        // waiting to be answered. The stylesheet draws this one in amber.
        return intl.formatMessage({id: 'FlowDiagram.cond-no-value', defaultMessage: '{property} = ?'}, {property: cond.property})
    }
    return `${cond.property} = ${cond.value}`
}

// stageLabel names what a stage does, in the reader's language. Kept short:
// this is a box on a canvas, not a form field.
export function stageLabel(intl: IntlShape, action: string): string {
    switch (action) {
    case 'agent':
        return intl.formatMessage({id: 'FlowDiagram.action-agent', defaultMessage: 'agent'})
    case 'deploy':
        return intl.formatMessage({id: 'FlowDiagram.action-deploy', defaultMessage: 'deploy'})
    case 'test':
        return intl.formatMessage({id: 'FlowDiagram.action-test', defaultMessage: 'test'})
    default:
        return intl.formatMessage({id: 'FlowDiagram.action-none', defaultMessage: 'waits'})
    }
}

const FlowDiagram = (props: Props) => {
    const intl = useIntl()
    const editable = () => Boolean(props.onChange)
    const waitTriggers = createMemo(() => props.triggers.filter((t) => t.source !== 'outcome'))
    const spare = () => props.spare || []

    // A faded column is on the canvas but not in the route, so it is looked up
    // by the id it would have as a stage.
    const spareById = createMemo(() => new Map(spare().map((c) => [nodeId(c), c])))

    // The transitions that are answers rather than wiring, kept off the canvas
    // and put on the box they belong to.
    const wiring = createMemo(() => props.edges.filter((e) => !isOutcomeEdge(e, props.outcomeProperty)))
    const lanes = createMemo(() => longHaul(props.nodes, wiring()))

    const graph = createMemo(() => {
        const tallest = props.nodes.reduce((at, n) => Math.max(at, stageHeight(props.writesOf?.(n))), NODE_HEIGHT)
        const positions = layout(props.nodes, props.edges, tallest)
        const sparePositions = spareLayout(positions, spare(), lanes().size)
        const box = (id: string, data: StageData, position: {x: number, y: number}): Node => ({
            id,
            type: 'stage',
            position,

            // Stated rather than measured: the box is a fixed size in CSS and
            // its handles sit at fixed points, so the arrows are drawn on the
            // first paint instead of after the browser reports a layout.
            //
            // Every handle must state its own size too. The canvas finds where
            // an arrow starts by adding the handle's width and height to its
            // corner, and the library defaults those by writing them back into
            // the node — which a Solid store's proxy ignores, silently. The sum
            // is then NaN and the path is drawn as `MNaN NaN…`, which renders
            // as nothing.
            width: NODE_WIDTH,
            height: stageHeight(data.writes),
            handles: [
                {type: 'target', position: Position.Left, x: 0, y: NODE_HEIGHT / 2, width: 1, height: 1},
                {type: 'source', id: HANDLE_SUCCESS, position: Position.Right, x: NODE_WIDTH, y: NODE_HEIGHT * 0.3, width: 1, height: 1},
                {type: 'source', id: HANDLE_FAILURE, position: Position.Right, x: NODE_WIDTH, y: NODE_HEIGHT * 0.7, width: 1, height: 1},
                {type: 'source', id: HANDLE_EVENT, position: Position.Bottom, x: NODE_WIDTH / 2, y: NODE_HEIGHT, width: 1, height: 1},

                // The data ports, spread along the bottom of the row of chips.
                // Their x is an even share of the width rather than the chip's
                // real position: the box is stated and not measured, and an
                // arrow leaving a little to one side of its own chip is a
                // smaller lie than an arrow drawn from NaN.
                ...(data.writes || []).
                    filter((w) => w.branchable).
                    map((w, i, all) => ({
                        type: 'source' as const,
                        id: HANDLE_WRITE + w.name,
                        position: Position.Bottom,
                        x: (NODE_WIDTH * (i + 1)) / (all.length + 1),
                        y: stageHeight(data.writes),
                        width: 1,
                        height: 1,
                    })),
            ],
            data,
            sourcePosition: Position.Right,
            targetPosition: Position.Left,
        })

        const rfNodes: Node[] = props.nodes.map((node) => {
            const action = node.action || props.actionOf?.(node) || 'none'
            return box(node.id, {
                column: node.column,
                action,
                actionLabel: stageLabel(intl, action),
                crew: props.crewOf?.(node),
                writes: props.writesOf?.(node),
                count: props.counts?.find((c) => c.nodeId === node.id),
                editable: editable(),
                selected: props.selected?.kind === 'node' && props.selected.id === node.id,
            }, positions.get(node.id) || {x: 0, y: 0})
        })

        for (const column of spare()) {
            const id = nodeId(column)
            rfNodes.push(box(id, {
                column: column.name,
                action: 'spare',
                actionLabel: intl.formatMessage({id: 'FlowDiagram.not-on-route', defaultMessage: 'not on this route'}),
                editable: editable(),
                spare: true,
            }, sparePositions.get(id) || {x: 0, y: 0}))
        }

        const known = new Set(props.nodes.map((n) => n.id))
        const rfEdges: Edge[] = props.edges.
            map((edge, index) => ({edge, index})).
            filter(({edge}) => known.has(edge.from) && known.has(edge.to)).
            map(({edge, index}) => {
                // An outcome answered on the card is a success or a failure
                // like any other, and is drawn as one: green on, red back, out
                // of the same two sockets. It rides card.changed because that
                // is how a person's answer reaches the engine, and colouring by
                // the trigger made the one arrow anybody looks for — the way
                // back from a review — a grey dashed line among the waits.
                const answered = isOutcomeEdge(edge, props.outcomeProperty)
                let kind = edgeKind(edge.on)
                if (answered) {
                    kind = sameFold(edge.if?.value, OUTCOME_PASSED) ? SUCCESS : FAILURE
                }
                const color = EDGE_COLOR[kind]

                // The caption is what decides where the card goes: the event
                // for a wait, the condition for a fork — both where the arrow
                // is, not three clicks away.
                const parts: string[] = []
                const cond = answered ? '' : condLabel(intl, edge.if)

                // For card.changed the condition is not a guard on the event,
                // it *is* the event — «на карточке выбрано» followed by «Исход =
                // не прошло» says the same thing twice, in a caption that then
                // does not fit anywhere.
                if (kind === 'event' && !(edge.on === CARD_CHANGED && cond)) {
                    parts.push(props.triggers.find((t) => t.kind === edge.on)?.label || edge.on)
                }
                if (cond) {
                    parts.push(edge.on === CARD_CHANGED ? cond : intl.formatMessage({id: 'FlowDiagram.cond-if', defaultMessage: 'if {cond}'}, {cond}))
                }
                const chosen = props.selected?.kind === 'edge' && props.selected.id === edgeId(edge, index)

                // A fork pulled from a data port arrives without its value, and
                // its caption reads «Вердикт = ?». Amber rather than red: the
                // arrow is not wrong, it is waiting for the one thing the
                // gesture could not carry. Said as a style rather than as a
                // class, because the label is rendered into a portal of the
                // library's own and nothing of the edge's class reaches it.
                const unanswered = Boolean(edge.if && !edge.if.value && !edge.if.commentContains)

                // A long arrow gets a corridor of its own: `offset` is how far
                // a smoothstep path runs from the handle before it turns, so a
                // different one per arrow is a different lane to travel in.
                const lane = lanes().get(edgeId(edge, index))
                return {
                    id: edgeId(edge, index),
                    source: edge.from,
                    target: edge.to,
                    sourceHandle: kind === 'event' ? HANDLE_EVENT : kind,
                    type: 'lane',
                    class: `FlowDiagram__edge FlowDiagram__edge--${kind}${chosen ? ' FlowDiagram__edge--selected' : ''}`,

                    // The canvas' own idea of "selected", set from ours: it is
                    // what reaches the edge component, and the ✕ hangs off it.
                    selected: chosen,
                    label: parts.join(' · '),
                    labelStyle: unanswered ? {color: 'rgb(var(--away-indicator-rgb))', 'font-weight': '600'} : undefined,
                    style: {stroke: color, 'stroke-width': chosen ? '3' : '1.5', 'stroke-dasharray': kind === 'event' ? '4 3' : undefined},
                    markerEnd: {type: MarkerType.ArrowClosed, width: 16, height: 16, color},
                    data: {lane, editable: editable()},
                }
            })
        return {rfNodes, rfEdges}
    })

    // Stages can always be dragged apart when arrows overlap: the canvas moves
    // them inside these stores. Where the route is being edited the position is
    // part of it and is saved; where it is only being read, the move is the
    // reader's and is forgotten when the route next arrives through props.
    // The stores are typed by node-type name for authoring by hand; the graph
    // above already produces the library's Node/Edge shapes, so the input
    // union is narrowed back to them.
    const [drawnNodes, setDrawnNodes] = createNodeStore([]) as unknown as [Node[], (nodes: Node[]) => void]
    const [drawnEdges, setDrawnEdges] = createEdgeStore([]) as unknown as [Edge[], (edges: Edge[]) => void]

    createEffect(() => {
        setDrawnNodes(graph().rfNodes)
        setDrawnEdges(graph().rfEdges)
    })

    // An arrow drawn to or from a faded column puts it on the route first: the
    // gesture says "this column is part of this route", and asking for a second
    // one afterwards would be asking twice.
    const joinSpares = (...ids: string[]) => {
        for (const id of ids) {
            const column = spareById().get(id)
            if (column) {
                props.onAddColumn?.(column)
            }
        }
    }

    const onConnect = (connection: Connection) => {
        if (!props.onChange) {
            return
        }
        joinSpares(connection.source, connection.target)
        props.onChange(props.nodes, connectEdge(props.edges, connection.source, connection.target, connection.sourceHandle, waitTriggers()))
    }

    const onNodeDragStop = ({targetNode}: {targetNode: Node | null}) => {
        if (!props.onChange || !targetNode) {
            return
        }

        // A faded column carried into the graph joins the route where it was
        // let go — the same gesture as dropping a palette block.
        const column = spareById().get(targetNode.id)
        if (column) {
            props.onAddColumn?.(column, {x: targetNode.position.x, y: targetNode.position.y})
            return
        }
        props.onChange(props.nodes.map((n) => (n.id === targetNode.id ? {...n, x: targetNode.position.x, y: targetNode.position.y} : n)), props.edges)
    }

    const onNodesDelete = (deleted: Node[]) => {
        if (!props.onChange) {
            return
        }
        const gone = new Set(deleted.map((n) => n.id))
        props.onChange(
            props.nodes.filter((n) => !gone.has(n.id)),

            // A transition to a stage that is no longer there is exactly what
            // the engine refuses to save.
            props.edges.filter((e) => !gone.has(e.from) && !gone.has(e.to)),
        )
    }

    // Breaking one arrow, by the id the canvas knows it as — which is the id
    // edgeId minted, so it carries the index the route stores it at.
    const removeEdgeById = (id: string) => {
        if (!props.onChange) {
            return
        }
        props.onChange(props.nodes, props.edges.filter((e, i) => edgeId(e, i) !== id))
        props.onSelect?.(null)
    }

    // An arrow's end dropped on another stage: the arrow moves there rather
    // than a second one appearing beside it.
    const onReconnect = (edge: Edge, connection: Connection) => {
        if (!props.onChange) {
            return
        }
        joinSpares(connection.source, connection.target)
        props.onChange(props.nodes, props.edges.map((e, i) => (edgeId(e, i) === edge.id ? {...e, from: connection.source, to: connection.target} : e)))
    }

    // …and dropped on nothing: the arrow is broken. That is the gesture this
    // whole anchor exists for — cutting a link where the link is, without
    // hunting for a control first.
    const onReconnectEnd = (_e: MouseEvent | TouchEvent, edge: Edge, _type: string, state: {isValid?: boolean | null}) => {
        if (!props.onChange || state?.isValid) {
            return
        }
        removeEdgeById(edge.id)
    }

    const onEdgesDelete = (deleted: Edge[]) => {
        if (!props.onChange) {
            return
        }
        const gone = new Set(deleted.map((e) => e.id))
        props.onChange(props.nodes, props.edges.filter((e, i) => !gone.has(edgeId(e, i))))
    }

    const onNodeClick = ({node}: {node: Node}) => {
        const column = spareById().get(node.id)
        if (column) {
            props.onAddColumn?.(column)
            return
        }
        props.onSelect?.({kind: 'node', id: node.id})
    }

    // The canvas API arrives from inside the canvas (CanvasHook); the drop
    // events arrive on the wrapper. Between them a palette block becomes a
    // stage exactly under the pointer.
    const [flowHandle, setFlowHandle] = createSignal<FlowHandle | null>(null)

    // `fitView` is an init option, and what is on the canvas changes after the
    // canvas is built: the editor opens on the columns, moves to the first route
    // when the routes arrive, and every tab click replaces every box. So the fit
    // is redone rather than configured once.
    //
    // Keyed on the *set* of stages and never on where they are: a fit in the
    // middle of a drag would pull the canvas out from under the pointer. The
    // handle is in the key because it arrives from inside the canvas and may not
    // be there on the first run.
    //
    // And it is the *only* fit there is. The canvas has one of its own on init
    // — fired when its boxes are first measured and its pane has a size,
    // whichever comes last, and fitted to every box on the canvas, the shelf
    // of spare columns included — and the two raced: whichever landed second
    // won, and which that was depended on whether the dialog had finished
    // laying out before the boxes were measured. So the editor opened centred
    // on the route one time and on the route-plus-shelf the next, which is a
    // short route pushed into a corner. `fitView={false}` below takes the
    // canvas' own out; until this one has landed the picture is not drawn
    // (`FlowDiagram--unfitted`), or the boxes would stand a frame at the origin.
    const shape = createMemo(() => drawnNodes.map((n) => n.id).join('|'))
    const [paneSize, setPaneSize] = createSignal('')
    const [fitted, setFitted] = createSignal(false)
    createEffect(() => {
        // What the fit is *about* is the route; what it is *keyed on* is every
        // box, so joining a faded column to the route re-fits to the new shape.
        shape()
        paneSize()
        const ids = props.nodes.map((n) => n.id)
        const handle = flowHandle()
        if (handle?.fitView && ids.length > 0) {
            let live = true
            onCleanup(() => {
                live = false
            })
            fitWhenMeasured(handle, ids, () => live, () => setFitted(true))
        }
    })

    // …and when the canvas itself changes size. A window resized, or a dialog
    // that gives the canvas a different share of it, leaves the picture where
    // it was: correct for the box it was fitted to and off centre in the new
    // one. Rounded to whole pixels so a sub-pixel reflow is not a re-fit, and
    // it costs somebody who has zoomed in by hand their zoom — which is the
    // right trade at the moment they resized the window, and does not happen
    // while they are working inside it.
    const watchPane = (pane: HTMLDivElement) => {
        if (typeof ResizeObserver !== 'function') {
            return
        }
        const observer = new ResizeObserver(([entry]) => {
            const box = entry.contentRect
            setPaneSize(`${Math.round(box.width)}x${Math.round(box.height)}`)
        })
        observer.observe(pane)
        onCleanup(() => observer.disconnect())
    }

    const dropPoint = (e: DragEvent) => {
        const at = {x: e.clientX, y: e.clientY}
        const handle = flowHandle()

        // Without the canvas API (jsdom, or a not-yet-mounted canvas) the drop
        // still works — the block lands at a default spot instead of the exact
        // pointer position.
        return handle ? handle.screenToFlowPosition(at) : {x: 0, y: 0}
    }

    const onDragOver = (e: DragEvent) => {
        if (props.onDropBlock && e.dataTransfer?.types.includes(BLOCK_DRAG_TYPE)) {
            e.preventDefault()
            e.dataTransfer.dropEffect = 'copy'
        }
    }

    const onDrop = (e: DragEvent) => {
        const kind = e.dataTransfer?.getData(BLOCK_DRAG_TYPE)
        if (!kind || !props.onDropBlock) {
            return
        }
        e.preventDefault()
        const at = dropPoint(e)
        props.onDropBlock(kind, {x: at.x - (NODE_WIDTH / 2), y: at.y - (NODE_HEIGHT / 2)})
    }

    return (
        <Show when={props.nodes.length > 0 || spare().length > 0}>
            <div
                ref={watchPane}
                class={`FlowDiagram${editable() ? ' FlowDiagram--editable' : ''}${fitted() ? '' : ' FlowDiagram--unfitted'}`}
                data-testid='flow-diagram'
                onDragOver={onDragOver}
                onDrop={onDrop}
            >
                <SolidFlow
                    nodes={drawnNodes}
                    edges={drawnEdges}
                    onConnect={onConnect}
                    onNodeDragStop={onNodeDragStop}
                    onNodesDelete={onNodesDelete}
                    onEdgesDelete={onEdgesDelete}
                    onNodeClick={onNodeClick}
                    onEdgeClick={({edge}: {edge: Edge}) => props.onSelect?.({kind: 'edge', id: edge.id})}
                    onReconnect={onReconnect}
                    onReconnectEnd={onReconnectEnd}
                    onPaneClick={() => props.onSelect?.(null)}
                    nodeTypes={nodeTypes}
                    edgeTypes={edgeTypes}
                    nodesConnectable={editable()}
                    elementsSelectable={editable()}
                    edgesFocusable={editable()}
                    deleteKey={editable() ? ['Backspace', 'Delete'] : null}

                    // The fit is ours (the effect above), not the canvas' own:
                    // two fits to two different sets of boxes raced, and the
                    // picture opened wherever the loser left it.
                    fitView={false}
                    minZoom={0.3}

                    // Solid Flow is MIT and its own attribution says to feel free to
                    // remove it; the canvas is small and the plate sits over the cards.
                    proOptions={{hideAttribution: true}}
                >
                    <Background/>
                    <Controls
                        showLock={false}

                        // The button fits what the opening fit fitted, the
                        // route, or pressing it would be the one way to get the
                        // shelf into the picture — which nobody has asked for.
                        fitViewOptions={{...FIT_VIEW, nodes: props.nodes.map((n) => ({id: n.id}))}}
                    />
                    <CanvasHook onReady={setFlowHandle}/>
                </SolidFlow>
            </div>
        </Show>
    )
}

export default FlowDiagram
