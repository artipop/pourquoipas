import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'
import {TOUR_SIDEBAR, SidebarTourSteps} from '../index'

import './sidebarCategories.scss'

const SidebarCategoriesTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='SidebarTour.SidebarCategories.Title'
            defaultMessage='Boards in the sidebar'
        />
    )

    // One message and no link. This step used to explain a Mattermost 7.2
    // upgrade that turned workspaces into sidebar categories, and pointed at
    // release notes on docs.mattermost.com — an announcement about a product
    // this is not, addressed to people who had upgraded something they never had.
    const screen = (
        <FormattedMessage
            id='SidebarTour.SidebarCategories.Body'
            defaultMessage='Every board is listed here. Drag one into a category of your own to keep the list in an order that suits you.'
        />
    )

    const punchout = useMeasurePunchouts(['.SidebarCategory'])

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_SIDEBAR}
            step={SidebarTourSteps.SIDE_BAR}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='SidebarCategories'
            placement={'right'}
            hideBackdrop={false}
            showForce={true}
        />
    )
}

export default SidebarCategoriesTourStep
