import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './board_menu.scss'

import {BoardTourSteps, TOUR_BOARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const BoardMenuTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.BoardMenu.Title'
            defaultMessage='The board’s menu'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.BoardMenu.Body'
            defaultMessage='Here is what the board tells its agents (its system prompt) and its folders. The agents themselves are yours and are connected in Settings → AI.'
        />
    )

    const punchout = useMeasurePunchouts(['.ViewHeader__actions'])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_BOARD}
            step={BoardTourSteps.BOARD_MENU}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='BoardMenuTourStep'
            placement={'bottom-end'}
            hideBackdrop={true}
        />
    )
}

export default BoardMenuTourStep
