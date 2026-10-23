import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './assign_agent.scss'

import {CardTourSteps, TOUR_CARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

// The first thing on a card worth knowing: the field that decides whether an
// agent works it at all.
const AssignAgentTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.AssignAgent.Title'
            defaultMessage='Who works on the card'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.AssignAgent.Body'
            defaultMessage='Put an agent in this field and the card is the agent’s to work. Nothing starts yet: the agent takes the card when the card is moved into another column. Agents are connected in Settings → AI.'
        />
    )

    const punchout = useMeasurePunchouts(['.octo-propertyrow--assignee'])

    return (
        <TourTipRenderer
            requireCard={true}
            category={TOUR_CARD}
            step={CardTourSteps.ASSIGN_AGENT}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='AssignAgentTourStep'
            placement={'right-end'}
            hideBackdrop={true}
        />
    )
}

export default AssignAgentTourStep
