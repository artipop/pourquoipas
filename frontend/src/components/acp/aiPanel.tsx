import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import Switch from '../../widgets/switch'
import {aiEnabled, setAIEnabled} from '../../aiSwitch'
import {goApp} from '../../goBridge'

import AgentsPanel from './agentsPanel'
import ProxiesPanel, {isProxiesAvailable} from './proxiesPanel'
import {agentNotificationsOn, setAgentNotifications} from './attention'

import '../settings/appPanel.scss'
import './aiPanel.scss'

// Everything about the AI, in the one place it is set up: whether agents work
// at all, the agents of both kinds — a program on this computer or a model on
// the person's API key — and the proxies a program may be started behind. An
// agent is the person's, not a board's: any of them may be assigned a card on
// any board, so no board has a screen of its own for them.

export function isAIPanelAvailable(): boolean {
    return Boolean(goApp()?.ListAgents)
}

const AIPanel = () => {
    const intl = useIntl()

    return (
        <div class='AppPanel AIPanel'>
            {/* The one switch for everything the app does with agents
                (aiSwitch.ts), whatever they are. */}
            <section class='AppPanel__group'>
                <div class='AppPanel__action'>
                    <div class='AppPanel__actionText'>
                        <span class='AppPanel__actionName'>
                            {intl.formatMessage({id: 'Settings.ai-enabled', defaultMessage: 'Agents work on the cards'})}
                        </span>
                        <span class='AppPanel__actionHint'>
                            {intl.formatMessage({
                                id: 'Settings.ai-enabled-hint',
                                defaultMessage: 'When this is off, the app is a plain kanban board: no agents and no agent settings, and cards moved between columns start nothing. The agents already connected are kept and come back when this is turned on again.',
                            })}
                        </span>
                    </div>
                    <Switch
                        isOn={aiEnabled()}
                        onChanged={setAIEnabled}
                    />
                </div>
            </section>

            <Show when={aiEnabled()}>
                {/* Only about interrupting: the card's own badge and the dot
                    on the tray icon are part of the card and stay. One switch
                    for the page's notification and the system's, which the Go
                    side reads too (alerts.go, machinesettings.go). */}
                <section class='AppPanel__group'>
                    <div class='AppPanel__action'>
                        <div class='AppPanel__actionText'>
                            <span class='AppPanel__actionName'>
                                {intl.formatMessage({id: 'Sidebar.agent-notifications', defaultMessage: 'Notify me when an agent is waiting'})}
                            </span>
                            <span class='AppPanel__actionHint'>
                                {intl.formatMessage({
                                    id: 'Machine.agent-notifications-hint',
                                    defaultMessage: 'The notification shows the question itself, and the agent can be answered right in it; when the app is in the background it comes from the system instead. The dot on the card and the dot on the tray icon stay even with this off.',
                                })}
                            </span>
                        </div>
                        <Switch
                            isOn={agentNotificationsOn()}
                            onChanged={setAgentNotifications}
                        />
                    </div>
                </section>

                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'Machine.section-agents', defaultMessage: 'Agents'})}
                    </h4>
                    <span class='AppPanel__actionHint'>
                        {intl.formatMessage({id: 'Agents.subtitle', defaultMessage: 'An agent works the cards it is the assignee of, on any board. Moving such a card into another column starts it.'})}
                    </span>
                    <AgentsPanel/>
                </section>

                <Show when={isProxiesAvailable()}>
                    <section class='AppPanel__group'>
                        <h4 class='AppPanel__groupTitle'>
                            {intl.formatMessage({id: 'Proxies.title', defaultMessage: 'Proxy configurations'})}
                        </h4>
                        <ProxiesPanel/>
                    </section>
                </Show>
            </Show>
        </div>
    )
}

export default AIPanel
