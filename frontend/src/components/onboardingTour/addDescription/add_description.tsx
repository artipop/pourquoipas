import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './add_description.scss'

import {CardTourSteps, TOUR_CARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const AddDescriptionTourStep = (): JSX.Element | null => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.AddDescription.Title'
            defaultMessage='Description'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.AddDescription.Body'
            defaultMessage='The description is the task itself: it is what the agent gets when it picks the card up, so it is worth writing clearly.'
        />
    )

    const punchout = useMeasurePunchouts(['.octo-content'])

    return (
        <TourTipRenderer
            requireCard={true}
            category={TOUR_CARD}
            step={CardTourSteps.ADD_DESCRIPTION}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='AddDescriptionTourStep'
            placement={'top-start'}
            hideBackdrop={true}
        />
    )
}

export default AddDescriptionTourStep
