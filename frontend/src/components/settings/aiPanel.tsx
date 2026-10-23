import {Show} from 'solid-js'

import {useIntl} from '../../intl'
import Switch from '../../widgets/switch'
import {aiEnabled, setAIEnabled} from '../../aiSwitch'
import {goApp} from '../../goBridge'
import AgentsPanel from '../acp/agentsPanel'
import ProxiesPanel, {isProxiesAvailable} from '../acp/proxiesPanel'

import './appPanel.scss'
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
                <section class='AppPanel__group'>
                    <h4 class='AppPanel__groupTitle'>
                        {intl.formatMessage({id: 'Machine.section-agents', defaultMessage: 'Agents'})}
                    </h4>
                    <span class='AppPanel__actionHint'>
                        {intl.formatMessage({id: 'Agents.subtitle', defaultMessage: 'An agent works a card it is assigned to in the card’s «Assignee» field, on any board. Moving the card into another column starts it.'})}
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
