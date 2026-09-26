import {Show} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {type Placement} from '@floating-ui/dom'

import {ClientConfig} from '../../../config/clientConfig'
import {getClientConfig} from '../../../store/clientConfig'

import {useAppSelector} from '../../../store/hooks'
import {getCurrentCard} from '../../../store/cards'
import TourTip from '../../tutorial_tour_tip/tutorial_tour_tip'
import {TutorialTourTipPunchout} from '../../tutorial_tour_tip/tutorial_tour_tip_backdrop'
import {tourStepNumber} from '../index'
import {useOnTourBoard, useTourAt, useTourFlags} from '../tourState'

type Props = {
    requireCard: boolean
    category: string

    // The step's name (index.ts); its number depends on what is switched on.
    step: string
    screen: JSX.Element
    title: JSX.Element
    punchout: TutorialTourTipPunchout | null | undefined
    classname: string
    placement: Placement | undefined
    hideBackdrop: boolean
    imageURL?: string
    singleTip?: boolean
    hideNavButtons?: boolean
    showForce?: boolean
}

const TourTipRenderer = (props: Props): JSX.Element | null => {
    const clientConfig = useAppSelector<ClientConfig>(getClientConfig)
    const flags = useTourFlags()
    const onTourBoard = useOnTourBoard()
    const at = useTourAt(props.category, props.step)
    const currentCard = useAppSelector((state) => (props.requireCard ? getCurrentCard(state) : null))

    const disableTour = () => clientConfig()?.featureFlags?.disableTour || false

    // A card tip is drawn over whichever card is open on the tour's board: the
    // base tip points at the board's first card, but nothing asks what it is
    // called.
    const showTourTip = () => {
        let show = !disableTour() && (props.showForce || onTourBoard()) && at()
        if (props.requireCard) {
            show = show && Boolean(currentCard())
        }
        return show
    }

    return (
        <Show when={showTourTip()}>
            <TourTip
                screen={props.screen}
                title={props.title}
                punchOut={props.punchout}
                step={tourStepNumber(props.category, props.step, flags())}
                tutorialCategory={props.category}
                placement={props.placement}
                class={props.classname}
                imageURL={props.imageURL}
                skipCategoryFromBackdrop={true}
                autoTour={true}
                hideBackdrop={props.hideBackdrop}
                singleTip={props.singleTip}
                hideNavButtons={props.hideNavButtons}
            />
        </Show>
    )
}

export default TourTipRenderer
