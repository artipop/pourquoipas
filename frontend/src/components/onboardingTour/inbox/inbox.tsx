import {createEffect} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../../../intl'

import {useMeasurePunchouts} from '../../tutorial_tour_tip/hooks'
import TourTipRenderer from '../tourTipRenderer/tourTipRenderer'
import {FINISHED, SidebarTourSteps, TOUR_BOARD, TOUR_SIDEBAR} from '../index'
import {patchPreferences} from '../tourState'
import {useAppSelector, useAppStore} from '../../../store/hooks'
import {getMe, getOnboardingTourCategory, getOnboardingTourStep} from '../../../store/users'

import './inbox.scss'

// The sidebar's part of the tour opens on the inbox: the one board every source
// writes to, and the half of the product a person has no other way to find out
// about. Mounted beside the inbox row, which is always drawn, so it is also
// what moves the tour on from the board's part.
const InboxTourStep = (): JSX.Element => {
    const title = (
        <FormattedMessage
            id='SidebarTour.Inbox.Title'
            defaultMessage='Inbox'
        />
    )
    const screen = (
        <FormattedMessage
            id='SidebarTour.Inbox.Body'
            defaultMessage='Cards from outside the app arrive here: from mail, a tracker, the share menu, a script. Each one waits until you move it to the board it belongs on.'
        />
    )

    const punchout = useMeasurePunchouts(['.SidebarInboxItem'])

    const {actions} = useAppStore()
    const me = useAppSelector(getMe)
    const category = useAppSelector(getOnboardingTourCategory)
    const step = useAppSelector(getOnboardingTourStep)

    createEffect(() => (category() === TOUR_BOARD && step() === FINISHED.toString() ? me()?.id : undefined), (userId) => {
        if (userId) {
            patchPreferences(userId, {tourCategory: TOUR_SIDEBAR, onboardingTourStep: '0'}, actions.users.patchProps)
        }
    })

    return (
        <TourTipRenderer
            requireCard={false}
            category={TOUR_SIDEBAR}
            step={SidebarTourSteps.INBOX}
            screen={screen}
            title={title}
            punchout={punchout()}
            classname='InboxTourStep'
            placement={'right'}
            hideBackdrop={false}
            showForce={true}
        />
    )
}

export default InboxTourStep
