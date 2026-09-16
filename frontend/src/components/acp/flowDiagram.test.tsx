import {render, screen} from '@solidjs/testing-library'
import {flush} from 'solid-js'
import '@testing-library/jest-dom'

import {wrapIntl} from '../../testUtils'
import {setupReactFlowEnvironment} from '../../test/reactFlowEnvironment'

import FlowDiagram, {depths, layout, edgeKind, connectEdge, lanePath, longHaul, stageHeight, HANDLE_EVENT, HANDLE_FAILURE, HANDLE_SUCCESS, HANDLE_WRITE, NODE_HEIGHT, NODE_WIDTH, WRITE_ROW} from './flowDiagram'
import {SUCCESS, FAILURE} from './automation'

setupReactFlowEnvironment()

const nodes = [
    {id: 'agent', column: 'To Agent', action: 'agent'},
    {id: 'review', column: 'In Review', action: 'none'},
    {id: 'deploy', column: 'Deploy', action: 'deploy'},
    {id: 'test', column: 'To Test', action: 'test'},
]

const edges = [
    {from: 'agent', to: 'review', on: SUCCESS},
    {from: 'review', to: 'deploy', on: 'branch.merged'},
    {from: 'deploy', to: 'test', on: SUCCESS},

    // A failed check goes back to the agent: the route contains a cycle.
    {from: 'test', to: 'agent', on: FAILURE},
]

const triggers = [
    {kind: 'success', source: 'outcome', label: 'шаг прошёл'},
    {kind: 'failure', source: 'outcome', label: 'шаг упал'},
    {kind: 'branch.merged', source: 'git', label: 'ветка влита в основную'},
]

describe('components/acp/flowDiagram layout', () => {
    test('a stage sits to the right of everything that leads to it', () => {
        const depth = depths(nodes, edges)
        expect(depth.get('agent')).toBe(0)
        expect(depth.get('review')).toBe(1)
        expect(depth.get('deploy')).toBe(2)
        expect(depth.get('test')).toBe(3)

        const positions = layout(nodes, edges)
        expect(positions.get('review')!.x).toBe(positions.get('agent')!.x + NODE_WIDTH + 80)
    })

    test('stages of equal depth are stacked, not overlaid', () => {
        const branching = [
            {id: 'a', column: 'A', action: 'agent'},
            {id: 'ok', column: 'B', action: 'none'},
            {id: 'bad', column: 'C', action: 'none'},
        ]
        const positions = layout(branching, [
            {from: 'a', to: 'ok', on: SUCCESS},
            {from: 'a', to: 'bad', on: FAILURE},
        ])
        expect(positions.get('ok')!.x).toBe(positions.get('bad')!.x)
        expect(positions.get('ok')!.y).not.toBe(positions.get('bad')!.y)
    })

    test('a cycle lays out instead of spinning', () => {
        const loop = [
            {id: 'a', column: 'A', action: 'none'},
            {id: 'b', column: 'B', action: 'none'},
        ]
        const positions = layout(loop, [
            {from: 'a', to: 'b', on: SUCCESS},
            {from: 'b', to: 'a', on: FAILURE},
        ])
        expect(positions.size).toBe(2)
    })

    test('an edge is styled by what produces it', () => {
        expect(edgeKind(SUCCESS)).toBe('success')
        expect(edgeKind(FAILURE)).toBe('failure')
        expect(edgeKind('pr.merged')).toBe('event')
    })
})

describe('components/acp/flowDiagram', () => {
    test('draws every stage and labels the events it waits for', () => {
        render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
            />,
        ))

        for (const node of nodes) {
            expect(screen.getByText(node.column)).toBeInTheDocument()
        }

        // The outcome arrows carry their colour, not a word; only the awaited
        // events are worth a label.
        expect(screen.getByText('ветка влита в основную')).toBeInTheDocument()
        expect(screen.queryByText('шаг прошёл')).not.toBeInTheDocument()
    })

    // The arrows are the route: without them the picture is a row of boxes in
    // no order. They went missing once to a path of NaNs, which a browser draws
    // as nothing at all and reports nowhere, so the geometry is asserted rather
    // than the presence of an element.
    test('every transition is drawn as a line with real coordinates', () => {
        const {container} = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
            />,
        ))

        const paths = [...container.querySelectorAll('.solid-flow__edge-path')]
        expect(paths.length).toBe(edges.length)
        for (const path of paths) {
            expect(path.getAttribute('d')).not.toMatch(/NaN/)
        }
    })

    // The fit is the diagram's own and not the canvas' — the two used to race,
    // and the editor opened centred one time and in a corner the next — so the
    // picture is held back until the diagram's fit has landed. jsdom measures
    // nothing, which is the case the wait is bounded for: after its share of
    // frames the fit lands on whatever there is and the picture is shown.
    test('the picture is hidden until the diagram has fitted it', () => {
        const frames: FrameRequestCallback[] = []
        const raf = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
            frames.push(cb)
            return frames.length
        })
        const {container} = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
            />,
        ))
        const pane = container.querySelector('.FlowDiagram')!
        expect(pane).toHaveClass('FlowDiagram--unfitted')

        for (let i = 0; i < 100 && frames.length > 0; i++) {
            frames.shift()!(i * 16)
        }

        // A frame callback is not an event, so nothing drains the write it made.
        flush()
        expect(pane).not.toHaveClass('FlowDiagram--unfitted')
        raf.mockRestore()
    })

    test('an empty route draws nothing at all', () => {
        const {container} = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={[]}
                edges={[]}
                triggers={triggers}
            />,
        ))
        expect(container).toBeEmptyDOMElement()
    })
})

describe('components/acp/flowDiagram builder', () => {
    const waitTriggers = [
        {kind: 'branch.merged', source: 'git', label: 'ветка влита в основную'},
        {kind: 'pr.merged', source: 'github', label: 'pull request смержен'},
    ]

    test('the handle a connection is pulled from is what the transition means', () => {
        expect(connectEdge([], 'a', 'b', HANDLE_SUCCESS, waitTriggers)).toEqual([{from: 'a', to: 'b', on: SUCCESS}])
        expect(connectEdge([], 'a', 'b', HANDLE_FAILURE, waitTriggers)).toEqual([{from: 'a', to: 'b', on: FAILURE}])

        // An event connection takes the first trigger the stage does not
        // already wait for, so pulling twice does not overwrite the first.
        const first = connectEdge([], 'a', 'b', HANDLE_EVENT, waitTriggers)
        expect(first).toEqual([{from: 'a', to: 'b', on: 'branch.merged'}])
        expect(connectEdge(first, 'a', 'c', HANDLE_EVENT, waitTriggers)).toEqual([
            {from: 'a', to: 'b', on: 'branch.merged'},
            {from: 'a', to: 'c', on: 'pr.merged'},
        ])
    })

    test('a stage may have only one transition per event, and none to itself', () => {
        const existing = [{from: 'a', to: 'b', on: SUCCESS}]
        expect(connectEdge(existing, 'a', 'c', HANDLE_SUCCESS, waitTriggers)).toEqual([{from: 'a', to: 'c', on: SUCCESS}])
        expect(connectEdge(existing, 'a', 'a', HANDLE_SUCCESS, waitTriggers)).toBe(existing)
        expect(connectEdge(existing, '', 'c', HANDLE_SUCCESS, waitTriggers)).toBe(existing)
    })

    test('a stage placed by hand stays where it was put', () => {
        const placed = [
            {id: 'a', column: 'A', action: 'agent', x: 400, y: 40},
            {id: 'b', column: 'B', action: 'none'},
        ]
        const positions = layout(placed, [{from: 'a', to: 'b', on: SUCCESS}])
        expect(positions.get('a')).toEqual({x: 400, y: 40})

        // And one that was never placed is still laid out for the reader.
        expect(positions.get('b')).toBeDefined()
    })

    test('an editable canvas shows the outputs a route is drawn from', () => {
        const readonly = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
            />,
        ))
        expect(readonly.container.querySelector('.FlowDiagram--editable')).toBeNull()
        readonly.unmount()

        const editable = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
                onChange={vi.fn()}
            />,
        ))
        expect(editable.container.querySelector('.FlowDiagram--editable')).not.toBeNull()
        expect(editable.container.querySelectorAll('.FlowDiagram__out--success').length).toBe(nodes.length)
    })

    // The rings on an arrow's ends are the only way to re-point one or to break
    // it where it is, and they went in as a 14-unit target that the zoom then
    // shrank — so what is asserted is that they are there to be grabbed at all,
    // and only where the route can be redrawn.
    test('a selected arrow offers a ring at each end, and only on a canvas being built', () => {
        const readonly = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
                selected={{kind: 'edge', id: 'agent-success-0'}}
            />,
        ))
        expect(readonly.container.querySelectorAll('.FlowDiagram__anchor').length).toBe(0)
        readonly.unmount()

        const editable = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
                selected={{kind: 'edge', id: 'agent-success-0'}}
                onChange={vi.fn()}
            />,
        ))
        expect(editable.container.querySelectorAll('.FlowDiagram__anchor').length).toBe(2)
    })

    test('the map says how many cards stand on a stage', async () => {
        render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
                counts={[{nodeId: 'agent', cards: 3, running: 1, queued: 1}]}
            />,
        ))
        expect(screen.getByText('3')).toBeInTheDocument()
    })

    // The dataflow is drawn on the box, and the fork is pulled from the thing
    // that produces the value: this is the whole point of the row. An arrow
    // dragged from ○Вердикт is «поедет сюда, если Вердикт = …», assembled from
    // the gesture rather than from a panel listing every field on the board.
    test('an arrow pulled from an output branches on that property', () => {
        const next = connectEdge([], 'test', 'agent', HANDLE_WRITE + 'Вердикт', triggers)
        expect(next).toEqual([{from: 'test', to: 'agent', on: SUCCESS, if: {property: 'Вердикт', value: ''}}])

        // The value is the one thing the gesture cannot carry, so it is left
        // empty on purpose and the caption says «Вердикт = ?» until it is
        // chosen. Pulling again makes a second branch rather than replacing the
        // first — which is what a fork is, and what the plain outputs do not do.
        const twice = connectEdge(next, 'test', 'review', HANDLE_WRITE + 'Вердикт', triggers)
        expect(twice.length).toBe(2)
    })

    // A stage with outputs stands taller, and the canvas is told so before it
    // paints — the box is stated, never measured, or the arrows are drawn from
    // the wrong corner on the first frame.
    test('a stage with outputs is taller than one without', () => {
        expect(stageHeight()).toBe(NODE_HEIGHT)
        expect(stageHeight([])).toBe(NODE_HEIGHT)
        expect(stageHeight([{name: 'Вердикт', branchable: true}])).toBe(NODE_HEIGHT + WRITE_ROW)
    })

    // Only a property with a closed set can be branched on, because an arrow
    // asks "is it this value". A text output is drawn — it is what the stages
    // further on will read — but there is nothing to pull from it.
    test('only an output with a set of values carries a port', () => {
        const {container} = render(() => wrapIntl(() =>
            <FlowDiagram
                nodes={nodes}
                edges={edges}
                triggers={triggers}
                onChange={vi.fn()}
                writesOf={(node) => (node.id === 'test' ? [{name: 'Вердикт', branchable: true}, {name: 'Заметка', branchable: false}] : [])}
            />,
        ))
        expect(screen.getByText('Вердикт')).toBeInTheDocument()
        expect(screen.getByText('Заметка')).toBeInTheDocument()
        expect(container.querySelectorAll('.FlowDiagram__out--write').length).toBe(1)
    })

    // Two arrows that cross the picture used to travel it side by side and read
    // as one thick line — which is what «дубли линий» turned out to be. Each
    // gets a corridor of its own under the graph.
    test('an arrow that skips columns gets a lane, a step to the next stage does not', () => {
        const lanes = longHaul(nodes, edges)

        // agent → review → deploy → test are one step each; the failure back to
        // the agent skips three.
        expect(lanes.has('agent-success-0')).toBe(false)
        expect(lanes.has('deploy-success-2')).toBe(false)
        expect(lanes.get('test-failure-3')).toBe(0)
    })

    // And a return of one column is still a return: it travels the width of two
    // boxes to get there, and the library draws it through them — which cuts it
    // into what look like two broken stubs.
    test('an arrow one column back is laned too', () => {
        const back = [
            {id: 'a', column: 'A', action: 'agent'},
            {id: 'b', column: 'B', action: 'deploy'},
        ]
        const lanes = longHaul(back, [
            {from: 'a', to: 'b', on: SUCCESS},
            {from: 'b', to: 'a', on: FAILURE},
        ])
        expect(lanes.has('a-success-0')).toBe(false)
        expect(lanes.get('b-failure-1')).toBe(0)
    })

    test('a lane runs under the lower of the two stages, and two lanes do not share a corridor', () => {
        const first = lanePath(100, 50, 400, 50, 0)
        const second = lanePath(100, 50, 500, 50, 1)
        const corridor = (d: string) => Number(d.split('L')[2].split(',')[1])

        // Under both boxes, and each lane lower than the one before it.
        expect(corridor(first.d)).toBeGreaterThan(50 + (NODE_HEIGHT / 2))
        expect(corridor(second.d)).toBeGreaterThan(corridor(first.d))

        // The caption rides in the corridor rather than over a box.
        expect(first.labelY).toBe(corridor(first.d))
    })
})
