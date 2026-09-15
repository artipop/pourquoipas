import {render, screen, waitFor} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'
import '@testing-library/jest-dom'

import {mockAppStore, wrapIntl} from '../../testUtils'
import {AppStoreProvider} from '../../store'
import octoClient from '../../octoClient'
import {IUser} from '../../user'

import PeoplePanel from './peoplePanel'

describe('components/settings/peoplePanel', () => {
    const admin = {id: 'admin-id', username: 'anna', role: 'admin', kind: 'person', permissions: ['manage_system']} as IUser
    const member = {id: 'member-id', username: 'boris', role: 'member', kind: 'person'} as IUser
    const agent = {id: 'agent-id', username: 'klaus', role: 'member', kind: 'agent'} as IUser

    const store = mockAppStore({
        users: {me: admin},
        teams: {current: {id: 'team-id', signupToken: 'a-token'}},
    })

    let getAdminUsers: ReturnType<typeof vi.spyOn>
    let setUserRole: ReturnType<typeof vi.spyOn>
    let setUserActive: ReturnType<typeof vi.spyOn>

    beforeEach(() => {
        vi.clearAllMocks()
        getAdminUsers = vi.spyOn(octoClient, 'getAdminUsers').mockResolvedValue([admin, member, agent])
        setUserRole = vi.spyOn(octoClient, 'setUserRole').mockResolvedValue(true)
        setUserActive = vi.spyOn(octoClient, 'setUserActive').mockResolvedValue(true)
    })

    const open = () => render(() => wrapIntl(() =>
        <AppStoreProvider store={store}>
            <PeoplePanel/>
        </AppStoreProvider>,
    ))

    // The link is the only way anybody else gets an account, so the panel that
    // is about the people is where it belongs.
    test('shows the invitation link the second person registers with', async () => {
        open()

        await waitFor(() => expect(getAdminUsers).toHaveBeenCalled())
        expect(screen.getByText(/\/register\?t=a-token/)).toBeInTheDocument()
    })

    // The accounts this app makes for agents and sources are accounts like any
    // other, and until `kind` existed nothing could say which was which.
    test('tells a person from an agent', async () => {
        open()

        await waitFor(() => expect(screen.getByText('klaus')).toBeInTheDocument())
        expect(screen.getByText(/agent/)).toBeInTheDocument()
    })

    test('hands administration to somebody else', async () => {
        open()

        await waitFor(() => expect(screen.getByText('boris')).toBeInTheDocument())
        await userEvent.click(screen.getAllByRole('button', {name: 'Make an administrator'})[0])

        expect(setUserRole).toHaveBeenCalledWith('member-id', 'admin')
    })

    test('switches an account off', async () => {
        open()

        await waitFor(() => expect(screen.getByText('boris')).toBeInTheDocument())
        await userEvent.click(screen.getAllByRole('button', {name: 'Switch off'})[0])

        expect(setUserActive).toHaveBeenCalledWith('member-id', false)
    })

    // A button whose only outcome is a refusal is a button that lies: the
    // server keeps the last admin whatever this panel offers.
    test('offers nothing against your own account', async () => {
        open()

        await waitFor(() => expect(screen.getByText('anna')).toBeInTheDocument())

        const rows = screen.getAllByRole('button', {name: 'Switch off'})
        expect(rows).toHaveLength(2)
    })
})
