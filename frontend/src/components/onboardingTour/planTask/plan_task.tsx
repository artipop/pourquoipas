import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './plan_task.scss'

import {BoardTourSteps, TOUR_BOARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const PlanTaskTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.PlanTask.Title'
            defaultMessage='Talk it over first'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.PlanTask.Body'
            defaultMessage='When the task is not clear yet, open “New” → “Talk it over with an agent…”. The agent only reads and asks; the cards you agree on are made on this board.'
        />
    )

    const punchout = useMeasurePunchouts(['.ViewHeader__newCard'])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_BOARD}
            step={BoardTourSteps.PLAN_TASK}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='PlanTaskTourStep'
            placement={'bottom-end'}
            hideBackdrop={true}
        />
    )
}

export default PlanTaskTourStep
