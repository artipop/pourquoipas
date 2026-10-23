import {
    addAssigneesFilter,
    addBoardUsersSource,
    addCardPropertiesFilter,
    addCardPropertyRow,
    addCardSidePanel,
    addNewCardDefaults,
    addSettingsSection,
    addToSlot,
} from '../../extensions'
import {aiEnabled} from '../../aiSwitch'
import {UserSettings} from '../../userSettings'

import FirstRunStrip from './firstRunStrip'

import AIPanel, {isAIPanelAvailable} from './aiPanel'
import {agentBoardUsers, assignableFilter} from './agentAssignees'
import {withoutAgentProperties} from './agentProperties'
import {BoardAgentMenuItems, PlanningMenuItem} from './agentMenus'
import AttentionNotifications from './attentionNotifications'
import CardAgentBadges from './cardAgentBadges'
import CardAgentHeader from './cardAgentHeader'
import {cardChangesPanel} from './cardChanges'
import {FolderRowToAdd, assigneeRow, branchRow, folderRow} from './cardAgentProperties'
import {soleWorkdirOption} from './workdirSync'
import {SHOW_GIT} from './gitShown'
import MachineMiscPanel from './machineMiscPanel'

// Everything the agents add to the board, plugged in through extensions.tsx.
// Called once from main.tsx; the board does not import this, and without it
// the board is Focalboard's own (docs/architecture.md, «The board knows
// nothing of the agents»).
export function installAgentFeatures(): void {
    // Who a card can be given to: the person's agents, on any board.
    addBoardUsersSource(agentBoardUsers)
    addAssigneesFilter(assignableFilter())

    // The folder and the branch are the agents' fields, hidden when they are
    // off or on a server.
    addCardPropertiesFilter(withoutAgentProperties)

    // A board with one folder gives it to every new card — unless the card was
    // dropped into a group of the folder field itself, where the empty group
    // means «no folder yet» and filling it in would make it impossible to add a
    // card there.
    addNewCardDefaults((board, given, groupBy) => {
        const folder = aiEnabled() && UserSettings.prefillCardFolder ? soleWorkdirOption(board) : undefined
        if (!folder || folder.propertyId === groupBy?.id || given[folder.propertyId]) {
            return {}
        }
        return {[folder.propertyId]: folder.optionId}
    })

    addToSlot('app', AttentionNotifications)
    addToSlot('cardBadges', CardAgentBadges)
    addToSlot('cardDetailTop', CardAgentHeader)
    if (SHOW_GIT) {
        addCardSidePanel(cardChangesPanel)
    }
    addCardPropertyRow(assigneeRow)
    addCardPropertyRow(branchRow)
    addCardPropertyRow(folderRow)
    addToSlot('cardPropertiesEnd', FolderRowToAdd)
    addToSlot('newCardMenu', PlanningMenuItem)
    addToSlot('boardMenu', BoardAgentMenuItems)
    addToSlot('boardHeader', FirstRunStrip)

    // The AI's own section, right after the app itself; «Other», whose every
    // setting is about agents, last and only with them on.
    addSettingsSection({
        id: 'ai',
        name: (intl) => intl.formatMessage({id: 'Settings.section-ai', defaultMessage: 'AI'}),
        when: isAIPanelAvailable,
        body: AIPanel,
        first: true,
    })
    addSettingsSection({
        id: 'misc',
        name: (intl) => intl.formatMessage({id: 'Machine.section-misc', defaultMessage: 'Other'}),
        when: aiEnabled,
        body: MachineMiscPanel,
    })
}
