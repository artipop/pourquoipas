import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'

import '../assignAgent/assign_agent.scss'

import {CardTourSteps, TOUR_CARD} from '../index'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'

const PickFolderTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='OnboardingTour.PickFolder.Title'
            defaultMessage='Folder'
        />
    )
    const screen = (
        <FormattedMessage
            id='OnboardingTour.PickFolder.Body'
            defaultMessage='The folder on your machine the agent works in. Add one here and every card of the board can pick it; a board with one folder fills it in by itself. A card without a folder is worked in the board’s drafts. Once the agent has started, the field stays as it is.'
        />
    )

    const punchout = useMeasurePunchouts(['.octo-propertyrow--folder'])

    return (
        <TourTipRenderer
            requireCard={true}
            category={TOUR_CARD}
            step={CardTourSteps.PICK_FOLDER}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='PickFolderTourStep'
            placement={'right-end'}
            hideBackdrop={true}
        />
    )
}

export default PickFolderTourStep
