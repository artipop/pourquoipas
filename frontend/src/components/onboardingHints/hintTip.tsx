import {Show, createSignal, createUniqueId} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {type Placement} from '@floating-ui/dom'

import {useIntl} from '../../intl'
import Button from '../../widgets/buttons/button'
import PulsatingDot from '../pulsating_dot'
import {TourTipBox} from '../tutorial_tour_tip/tutorial_tour_tip'

import {useHints, useHintSlot} from './hints'

import './hintTip.scss'

type Props = {
    id: string
    wanted: boolean
    title: string
    body: string
    placement?: Placement

    // Where the dot sits against the element the hint is mounted in.
    class?: string
}

// HintTip is one hint: the tour's dot and box (tutorial_tour_tip), with one
// button that puts the hint away for good. It has no steps, so none of the
// tour's machinery — the manager saves tour progress on every click.
const HintTip = (props: Props): JSX.Element => {
    const intl = useIntl()
    const hints = useHints()
    const mine = useHintSlot(() => props.wanted && !hints.shown(props.id))
    const [dot, setDot] = createSignal<HTMLDivElement>()
    const titleId = createUniqueId()

    return (
        <Show when={mine()}>
            <div
                ref={setDot}
                class={`tutorial-tour-tip__pulsating-dot-ctr HintTip ${props.class || ''}`}
            >
                <PulsatingDot/>
            </div>
            <TourTipBox
                anchor={dot}
                placement={props.placement}
                width={320}
                class='HintTip__box'
                labelledBy={titleId}
            >
                <div onClick={(e) => e.stopPropagation()}>
                    <div class='tutorial-tour-tip__header'>
                        <h4
                            id={titleId}
                            class='tutorial-tour-tip__header__title'
                        >
                            {props.title}
                        </h4>
                    </div>
                    <div class='tutorial-tour-tip__body'>
                        {props.body}
                    </div>
                    <div class='tutorial-tour-tip__footer'>
                        <div class='tutorial-tour-tip__footer-buttons'>
                            <div/>
                            <div class='tutorial-tour-tip__btn-ctr'>
                                <Button
                                    class='HintTip__dismiss'
                                    size='small'
                                    filled={true}
                                    onClick={() => hints.markShown(props.id)}
                                >
                                    {intl.formatMessage({id: 'tutorial_tip.got_it', defaultMessage: 'Got it'})}
                                </Button>
                            </div>
                        </div>
                    </div>
                </div>
            </TourTipBox>
        </Show>
    )
}

export default HintTip
