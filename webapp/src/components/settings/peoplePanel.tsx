import {For, Show, createResource, createSignal} from 'solid-js'

import {useIntl} from '../../intl'

import octoClient from '../../octoClient'
import {Utils} from '../../utils'
import {useAppSelector, useAppStore} from '../../store/hooks'
import {getMe} from '../../store/users'
import {getCurrentTeam, Team} from '../../store/teams'
import Button from '../../widgets/buttons/button'
import {sendFlashMessage} from '../flashMessages'

import './peoplePanel.scss'

// Who is on this install, and who runs it. Not who may do what on one board —
// that is board membership, edited on the board itself.
//
// It lives here rather than in the account menu, where "Invite users" used to
// be, because the people of an install are the install's: the same rule that
// puts the agent registry and the archive in this dialog. One door, and it is
// the one reachable with no board open.

const PeoplePanel = () => {
    const intl = useIntl()
    const me = useAppSelector(getMe)
    const team = useAppSelector<Team|null>(getCurrentTeam)
    const {actions} = useAppStore()

    const [reload, setReload] = createSignal(0)
    const [people] = createResource(reload, () => octoClient.getAdminUsers())
    const [wasCopied, setWasCopied] = createSignal(false)

    const inviteLink = () => {
        const token = team()?.signupToken
        return token ? `${Utils.getBaseURL(true).replace(/\/$/, '')}/register?t=${token}` : ''
    }

    const say = (message: string) => sendFlashMessage({content: message, severity: 'low'})

    const copyInvite = () => {
        Utils.copyTextToClipboard(inviteLink())
        setWasCopied(true)
    }

    const regenerate = async () => {
        await actions.teams.regenerateSignupToken()
        await actions.teams.refreshCurrentTeam()
        setWasCopied(false)
        say(intl.formatMessage({id: 'People.invite-regenerated', defaultMessage: 'A new link is ready. The previous one no longer registers anybody.'}))
    }

    // The list is re-read rather than patched in place: these are four
    // operations on somebody else's account, and what the server did is the
    // only answer worth drawing.
    const after = async (ok: boolean, done: string) => {
        if (ok) {
            say(done)
        } else {
            say(intl.formatMessage({id: 'People.refused', defaultMessage: 'The server refused that. The last admin cannot be removed or switched off.'}))
        }
        setReload((n) => n + 1)
    }

    const isAdmin = (person: {role?: string}) => person.role === 'admin'
    const isActive = (person: {delete_at?: number}) => !person.delete_at

    return (
        <div class='PeoplePanel'>
            <div class='PeoplePanel__subtitle'>
                {intl.formatMessage({
                    id: 'People.subtitle',
                    defaultMessage: 'Everybody with an account on this server. Who may open which board is decided on the board itself.',
                })}
            </div>

            <div class='PeoplePanel__invite'>
                <div class='PeoplePanel__inviteName'>
                    {intl.formatMessage({id: 'People.invite', defaultMessage: 'Send this to whoever is joining'})}
                </div>
                <Show
                    when={inviteLink()}
                    fallback={
                        <div class='PeoplePanel__hint'>
                            {intl.formatMessage({id: 'People.no-invite', defaultMessage: 'There is no link yet.'})}
                        </div>
                    }
                >
                    <div class='PeoplePanel__inviteLink'>{inviteLink()}</div>
                    <div class='PeoplePanel__inviteButtons'>
                        <Button
                            filled={true}
                            onClick={copyInvite}
                        >
                            {wasCopied() ? intl.formatMessage({id: 'People.copied', defaultMessage: 'Copied'}) : intl.formatMessage({id: 'People.copy', defaultMessage: 'Copy'})}
                        </Button>
                        <Button onClick={regenerate}>
                            {intl.formatMessage({id: 'People.regenerate', defaultMessage: 'New link'})}
                        </Button>
                    </div>
                    <div class='PeoplePanel__hint'>
                        {intl.formatMessage({
                            id: 'People.invite-hint',
                            defaultMessage: 'The link works until you ask for a new one. A new one retires the old.',
                        })}
                    </div>
                </Show>
            </div>

            <div class='PeoplePanel__list'>
                <For each={people() ?? []}>
                    {(person) => (
                        <div
                            class='PeoplePanel__person'
                            classList={{'PeoplePanel__person--off': !isActive(person)}}
                        >
                            <div class='PeoplePanel__who'>
                                <span class='PeoplePanel__name'>{person.username}</span>
                                <span class='PeoplePanel__what'>
                                    <Show when={isAdmin(person)}>
                                        {intl.formatMessage({id: 'People.role-admin', defaultMessage: 'administrator'})}
                                    </Show>
                                    <Show when={person.kind && person.kind !== 'person'}>
                                        {' '}
                                        {person.kind === 'agent' ? intl.formatMessage({id: 'People.kind-agent', defaultMessage: 'agent'}) : intl.formatMessage({id: 'People.kind-source', defaultMessage: 'source'})}
                                    </Show>
                                    <Show when={!isActive(person)}>
                                        {' '}
                                        {intl.formatMessage({id: 'People.switched-off', defaultMessage: 'switched off'})}
                                    </Show>
                                </span>
                            </div>

                            {/* Nothing is offered against your own account: the
                                server refuses the last admin anyway, and a
                                button whose only outcome is a refusal is a
                                button that lies. */}
                            <Show when={person.id !== me()?.id}>
                                <div class='PeoplePanel__actions'>
                                    <Show when={isActive(person)}>
                                        <Button
                                            onClick={async () => after(
                                                await octoClient.setUserRole(person.id, isAdmin(person) ? 'member' : 'admin'),
                                                isAdmin(person) ? intl.formatMessage({id: 'People.now-member', defaultMessage: 'No longer an administrator'}) : intl.formatMessage({id: 'People.now-admin', defaultMessage: 'Now an administrator'}),
                                            )}
                                        >
                                            {isAdmin(person) ? intl.formatMessage({id: 'People.demote', defaultMessage: 'Take away administration'}) : intl.formatMessage({id: 'People.promote', defaultMessage: 'Make an administrator'})}
                                        </Button>
                                    </Show>
                                    <Button
                                        onClick={async () => after(
                                            await octoClient.setUserActive(person.id, !isActive(person)),
                                            isActive(person) ? intl.formatMessage({id: 'People.turned-off', defaultMessage: 'The account is switched off'}) : intl.formatMessage({id: 'People.turned-on', defaultMessage: 'The account works again'}),
                                        )}
                                    >
                                        {isActive(person) ? intl.formatMessage({id: 'People.deactivate', defaultMessage: 'Switch off'}) : intl.formatMessage({id: 'People.activate', defaultMessage: 'Switch on'})}
                                    </Button>
                                </div>
                            </Show>
                        </div>
                    )}
                </For>
            </div>
        </div>
    )
}

export default PeoplePanel
