import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './add_comments.scss'

import {CardTourSteps, TOUR_CARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const AddCommentTourStep = (): JSX.Element | null => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.AddComments.Title'
            defaultMessage='Comments'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.AddComments.Body'
            defaultMessage='Comments are where people talk about the card. When the agent finishes, it writes here what it did.'
        />
    )

    const punchout = useMeasurePunchouts(['.cardFollowBtn--comments'])

    return (
        <TourTipRenderer
            requireCard={true}
            category={TOUR_CARD}
            step={CardTourSteps.ADD_COMMENTS}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='AddCommentTourStep'
            placement={'bottom-end'}
            hideBackdrop={true}
        />
    )
}

export default AddCommentTourStep
