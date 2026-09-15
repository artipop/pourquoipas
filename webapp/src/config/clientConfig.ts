export type ClientConfig = {
    enablePublicSharedBoards: boolean
    featureFlags: Record<string, string>
    teammateNameDisplay: string
    maxFileSize: number

    // Whether this install has accounts on it, and therefore whether a comment
    // is addressed to anybody (docs/teamwork.md). Set by the Go side from the
    // same answer the board server was started with.
    teamMode: boolean

    // Whether the next person to register claims this install. True only on a
    // hosted server nobody has registered on yet, and what sends the first
    // visitor to /register instead of a login they have no account for.
    registrationOpen: boolean
}
