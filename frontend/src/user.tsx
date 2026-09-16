interface IUser {
    id: string
    username: string
    email: string
    nickname: string
    firstname: string
    lastname: string
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    props: Record<string, any>
    create_at: number
    update_at: number
    is_bot: boolean
    is_guest: boolean
    permissions?: string[]

    // Who administers this install: 'admin', or absent and 'member' reading the
    // same. A different question from what somebody may do on one board, which
    // is board membership.
    role?: string

    // person, agent or source — the accounts this app makes under their own
    // names are accounts like any other, and there was no way to tell them
    // apart.
    kind?: string

    // delete_at is a soft delete: an account switched off by an admin. Only the
    // admin's own list carries deactivated accounts at all; every other read
    // filters them out.
    delete_at?: number
}

interface UserWorkspace {
    id: string
    title: string
    boardCount: number
}

interface UserConfigPatch {
    updatedFields?: Record<string, string>
    deletedFields?: string[]
}

function parseUserProps(props: UserPreference[]): Record<string, UserPreference> {
    const processedProps: Record<string, UserPreference> = {}

    props.forEach((prop) => {
        const processedProp = prop
        if (prop.name === 'hiddenBoardIDs') {
            const hiddenBoardIDs = JSON.parse(processedProp.value)
            processedProp.value = {}
            hiddenBoardIDs.forEach((boardID: string) => {
                processedProp.value[boardID] = true
            })
        }
        processedProps[processedProp.name] = processedProp
    })

    return processedProps
}

interface UserPreference {
    user_id: string
    category: string
    name: string
    value: any
}

export {
    type IUser,
    type UserWorkspace,
    type UserConfigPatch,
    parseUserProps,
    type UserPreference,
}
