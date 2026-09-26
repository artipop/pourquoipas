import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './columns.scss'

import {BoardTourSteps, TOUR_BOARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const ColumnsTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.Columns.Title'
            defaultMessage='Columns'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.Columns.Body'
            defaultMessage='Moving a card into a column starts the agent the card is assigned to. The agent reads the column’s name with the task, and when it is done it moves the card on itself — into the column whose name says what happened. Rename the columns to tell agents what they mean.'
        />
    )

    const punchout = useMeasurePunchouts(['.octo-board-header'])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_BOARD}
            step={BoardTourSteps.COLUMNS}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='ColumnsTourStep'
            placement={'bottom-start'}
            hideBackdrop={true}
        />
    )
}

export default ColumnsTourStep
