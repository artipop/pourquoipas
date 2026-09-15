import {Show} from 'solid-js'

import {useIntl} from '../../intl'

import Dialog from '../dialog'

import AgentsPanel, {isAgentsAvailable} from './agentsPanel'
import ProxiesPanel, {isProxiesAvailable} from './proxiesPanel'

import './agentsDialog.scss'

// Who works this board's cards, on its own screen, reached from the board's ⋯
// menu beside the folders and the deploy hosts.
//
// The registry behind it is the machine's — an agent is a CLI installed here,
// not something a board owns — and it used to be two sections of the settings
// dialog for exactly that reason. What moved it is the same argument the deploy
// targets already answer to: a registry is offered where it means something,
// and an agent means nothing on a board no agent works. A person setting a
// board up is on the board, and the questions it asks — the folder, the deploy
// host, who picks the cards up — are now answered in one place instead of two.
//
// Proxies are under the agents rather than beside them: a proxy exists to be
// named by an agent (AgentEntry.proxyName), so it is the second half of one
// question, and on its own it was a section of the settings list that most
// installs never open.

export function isAgentsDialogAvailable(): boolean {
    return isAgentsAvailable()
}

type Props = {
    onClose: () => void
}

const AgentsDialog = (props: Props) => {
    const intl = useIntl()

    return (
        <Dialog
            class='AgentsDialog'
            title={<span>{intl.formatMessage({id: 'Machine.section-agents', defaultMessage: 'Agents'})}</span>}
            onClose={props.onClose}
        >
            <div class='AgentsDialog__content'>
                <AgentsPanel/>
                <Show when={isProxiesAvailable()}>
                    <section class='AgentsDialog__section'>
                        <h3 class='AgentsDialog__sectionTitle'>
                            {intl.formatMessage({id: 'Proxies.title', defaultMessage: 'Proxy configurations'})}
                        </h3>
                        <ProxiesPanel/>
                    </section>
                </Show>
            </div>
        </Dialog>
    )
}

export default AgentsDialog
