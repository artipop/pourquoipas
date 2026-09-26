import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './add_view.scss'

import {BoardTourSteps, TOUR_BOARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const AddViewTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.AddView.Title'
            defaultMessage='Add a view'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.AddView.Body'
            defaultMessage='A view is a way of looking at the same cards: a board, a table, a calendar. A board can have as many as you like.'
        />
    )

    const punchout = useMeasurePunchouts(['.viewSelector'])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_BOARD}
            step={BoardTourSteps.ADD_VIEW}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='AddViewTourStep'
            placement={'bottom-start'}
            hideBackdrop={false}
        />
    )
}

export default AddViewTourStep
