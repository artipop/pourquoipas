import '@testing-library/jest-dom'
import {render} from '@solidjs/testing-library'
import userEvent from '@testing-library/user-event'

import {MemberRole} from '../blocks/board'

import {wrapDNDIntl} from '../testUtils'
import {IUser} from '../user'

import ConfirmAddUserForNotifications from './confirmAddUserForNotifications'

describe('/components/confirmAddUserForNotifications', () => {
    it('should match snapshot', async () => {
        const result = render(() =>
            wrapDNDIntl(() =>
                <ConfirmAddUserForNotifications
                    allowManageBoardRoles={true}
                    minimumRole={MemberRole.Editor}
                    user={{id: 'fake-user-id', username: 'fake-username'} as IUser}
                    onConfirm={vi.fn()}
                    onClose={vi.fn()}
                />,
            ),
        )
        expect(result.container).toMatchSnapshot()
    })

    it('confirm button click, run onConfirm Function once', () => {
        const onConfirm = vi.fn()

        const result = render(() =>
            wrapDNDIntl(() =>
                <ConfirmAddUserForNotifications
                    allowManageBoardRoles={true}
                    minimumRole={MemberRole.Editor}
                    user={{id: 'fake-user-id', username: 'fake-username'} as IUser}
                    onConfirm={onConfirm}
                    onClose={vi.fn()}
                />,
            ),
        )
        userEvent.click(result.getByTitle('Add to board'))
        expect(onConfirm).toHaveBeenCalledTimes(1)
    })

    it('cancel button click runs onClose function', () => {
        const onClose = vi.fn()

        const result = render(() =>
            wrapDNDIntl(() =>
                <ConfirmAddUserForNotifications
                    allowManageBoardRoles={true}
                    minimumRole={MemberRole.Editor}
                    user={{id: 'fake-user-id', username: 'fake-username'} as IUser}
                    onConfirm={vi.fn()}
                    onClose={onClose}
                />,
            ),
        )
        userEvent.click(result.getByTitle('Cancel'))
        expect(onClose).toHaveBeenCalledTimes(1)
    })

    // A person is asked which permission to give, which is the whole reason
    // the dialog exists.
    it('asks which permission to give', async () => {
        const result = render(() =>
            wrapDNDIntl(() =>
                <ConfirmAddUserForNotifications
                    allowManageBoardRoles={true}
                    minimumRole={MemberRole.Viewer}
                    user={{id: 'fake-user-id', username: 'fake-username'} as IUser}
                    onConfirm={vi.fn()}
                    onClose={vi.fn()}
                />,
            ),
        )

        expect(result.getByText('Permissions')).toBeInTheDocument()
    })
})
