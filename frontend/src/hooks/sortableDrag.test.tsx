import {Show, createSignal} from 'solid-js'
import type {JSX} from '@solidjs/web'
import {render} from '@solidjs/testing-library'
import {useDragDropManager} from '@dnd-kit/solid'

import {drag} from '../test/drag'

import {SortableProvider, useSortable, useDropZone} from './sortable'

// Laid out one under another: a card at 0, a card at 100, the zone at 200.
const cardStyle = {width: '100px', height: '40px', 'margin-bottom': '60px'}
const zoneStyle = {width: '100px', height: '100px'}

type Item = {id: string}

function Card(props: {item: Item, onDrop: (src: Item, dst: Item) => void}) {
    const [, , ref] = useSortable('card', () => props.item, () => true, (src: Item, dst: Item) => props.onDrop(src, dst))
    return (
        <div
            ref={ref}
            data-testid={props.item.id}
            style={cardStyle}
        />
    )
}

function Zone(props: {onDrop: (src: Item) => void}) {
    const [, ref] = useDropZone<Item>('card', () => true, (src) => props.onDrop(src))
    return (
        <div
            ref={ref}
            data-testid='zone'
            style={zoneStyle}
        />
    )
}

describe('hooks/sortable dragging', () => {
    const a = {id: 'a'}
    const b = {id: 'b'}

    function setup() {
        const onCard = vi.fn()
        const onZone = vi.fn()
        const {getByTestId} = render(() => (
            <SortableProvider>
                <Card
                    item={a}
                    onDrop={onCard}
                />
                <Card
                    item={b}
                    onDrop={onCard}
                />
                <Zone onDrop={onZone}/>
            </SortableProvider>
        ))
        return {getByTestId, onCard, onZone}
    }

    it('drops a card onto another card', async () => {
        const {getByTestId, onCard} = setup()

        await drag(getByTestId('a'), getByTestId('b'))

        expect(onCard).toHaveBeenCalledWith(a, b)
    })

    // The wash on a column while a card is held over it is the only thing that
    // says where the card will land. It went missing once without a test
    // noticing: dnd-kit's Solid bindings subscribe to what the component has
    // already read, and under the compat layer they subscribed before the
    // component had read anything, so `isDropTarget` never changed.
    it('says it is the target while a card is held over it', async () => {
        function LitZone() {
            const [isOver, ref] = useDropZone<Item>('card', () => true, () => undefined)
            return (
                <div
                    ref={ref}
                    data-testid='lit'
                    class={isOver() ? 'dragover' : ''}
                    style={zoneStyle}
                />
            )
        }
        const {getByTestId} = render(() => (
            <SortableProvider>
                <Card
                    item={a}
                    onDrop={vi.fn()}
                />
                <LitZone/>
            </SortableProvider>
        ))

        // Every change of the zone's class while the drag runs, so the hover
        // is seen before the release clears it.
        const zone = getByTestId('lit')
        const lit: boolean[] = []
        const watching = new MutationObserver(() => lit.push(zone.classList.contains('dragover')))
        watching.observe(zone, {attributes: true, attributeFilter: ['class']})
        await drag(getByTestId('a'), zone)
        watching.disconnect()

        expect(lit).toContain(true)
        expect(zone.classList.contains('dragover')).toBe(false)
    })

    it('drops a card into a zone that holds none', async () => {
        const {getByTestId, onZone} = setup()

        await drag(getByTestId('a'), getByTestId('zone'))

        expect(onZone).toHaveBeenCalledWith(a)
    })

    // Moving the mouse quickly is not a mistake the user made. dnd-kit's default
    // pairs a Delay with a Distance under one controller, and the Delay aborts
    // the whole activation if the pointer outruns its tolerance before the timer
    // fires -- so a flick produces a beforedragstart, a canceled dragend, and no
    // drag. Whether dragging worked came down to how fast the hand moved.
    it('drops when the pointer is moved in one quick sweep', async () => {
        const {getByTestId, onZone} = setup()

        await drag(getByTestId('a'), getByTestId('zone'), {steps: 3})

        expect(onZone).toHaveBeenCalledWith(a)
    })

    // Dropping a card moves it, which unmounts the element that was being
    // dragged. dnd-kit resets a finished operation only once that element
    // reports itself idle at the end of its drop animation -- so an element that
    // leaves mid-animation leaves the manager believing a drag is still running,
    // and every press after it is ignored. Two drags in a row is the whole test.
    it('still works after the dragged card has gone away', async () => {
        const onCard = vi.fn()
        const onZone = vi.fn()

        function Board(props: {probe: JSX.Element}) {
            const [gone, setGone] = createSignal(false)
            return (
                <SortableProvider>
                    {props.probe}
                    <Show when={!gone()}>
                        <Card
                            item={a}
                            onDrop={(src, dst) => {
                                onCard(src, dst)
                                setGone(true)
                            }}
                        />
                    </Show>
                    <Card
                        item={b}
                        onDrop={onCard}
                    />
                    <Zone onDrop={onZone}/>
                </SortableProvider>
            )
        }

        let manager: {dragOperation: {status: {idle: boolean}}} | null = null

        function Probe() {
            manager = useDragDropManager() as never
            return null
        }

        const {getByTestId} = render(() => <Board probe={<Probe/>}/>)

        await drag(getByTestId('a'), getByTestId('b'))
        expect(onCard).toHaveBeenCalledWith(a, b)

        // The second drag is only half of it. dnd-kit refuses a press outright
        // unless the operation is idle, and refuses it in silence, so a board
        // that never got back to idle looks like a board nobody pressed. Say
        // which of the two failed.
        expect(manager!.dragOperation.status.idle).toBe(true)

        await drag(getByTestId('b'), getByTestId('zone'))
        expect(onZone).toHaveBeenCalledWith(b)
    })

    // The card being dragged is a drop target too, and it follows the pointer,
    // so without care it wins every collision and the drop is discarded as a
    // drop onto itself -- which is exactly how dragging stopped working.
    it('never lands on the card being dragged', async () => {
        const {getByTestId, onCard} = setup()

        // Moved past the activation distance and released over itself.
        await drag(getByTestId('a'), getByTestId('a'), {at: {x: 50, y: 35}})

        expect(onCard).not.toHaveBeenCalled()
    })
})
