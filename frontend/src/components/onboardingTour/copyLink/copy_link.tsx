import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import './copy_link.scss'

import {BoardTourSteps, OnboardingCardClassName, TOUR_BOARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const CopyLinkTourStep = (): JSX.Element | null => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.CopyLink.Title'
            defaultMessage='Copy the link'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.CopyLink.Body'
            defaultMessage='The link leads to one card and opens it wherever the board is available: in the window, in a browser, on a phone.'
        />
    )

    const punchout = useMeasurePunchouts([`.${OnboardingCardClassName} .optionsMenu`])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_BOARD}
            step={BoardTourSteps.COPY_LINK}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='CopyLinkTourStep'
            placement={'right-start'}
            hideBackdrop={true}
        />
    )
}

export default CopyLinkTourStep
