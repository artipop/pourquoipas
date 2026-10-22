import {defineMessages} from '@formatjs/intl'

import type {IntlShape} from './intl'

// What Go says to a person without saying it in a language (internal/apperr):
// a code, its values, and English for a code this page does not know. The
// sentences are here with every other string; TestEveryCodeHasASentence holds
// this list to the codes Go uses.

export type GoMessage = {
    code?: string
    args?: Record<string, string>
    text?: string
}

const messages: Record<string, {id: string, defaultMessage: string}> = defineMessages({
    'adapter.missing': {id: 'Go.adapter.missing', defaultMessage: 'Neither {bin} nor npx was found: install Node.js and run `npm install -g {package}`'},
    'adapter.no-npm': {id: 'Go.adapter.no-npm', defaultMessage: 'npm was not found: install Node.js'},
    'adapter.not-found': {id: 'Go.adapter.not-found', defaultMessage: '{bin} was not found: give the agent a binPath or a command'},
    'adapter.not-installed': {id: 'Go.adapter.not-installed', defaultMessage: '{bin} was not found: install it with `npm install -g {package}` (or install Node.js, and the adapter will run through npx)'},
    'adapter.via-npx': {id: 'Go.adapter.via-npx', defaultMessage: '{bin} is not installed, so it will be run through npx (the first run takes longer)'},
    'agents.account-left': {id: 'Go.agents.account-left', defaultMessage: 'Agent {name} was removed from the registry, but its account is still a member of the board'},
    'agents.ambiguous': {id: 'Go.agents.ambiguous', defaultMessage: 'Could not choose an agent: assign one to the card (there are: {agents})'},
    'agents.busy': {id: 'Go.agents.busy', defaultMessage: 'The agent is busy with another card'},
    'agents.cli-args-flag': {id: 'Go.agents.cli-args-flag', defaultMessage: 'CLI arguments have to start with a flag, for example --remote-control (got {got})'},
    'agents.cli-args-kind': {id: 'Go.agents.cli-args-kind', defaultMessage: 'An agent of kind {kind} cannot be handed CLI arguments: its adapter has no way to pass them. Use the extra CLI arguments, which go into the launch command itself'},
    'agents.cli-not-here': {id: 'Go.agents.cli-not-here', defaultMessage: 'Agent {name} runs a program on a machine, and this server runs none: give the card an agent that is a model'},
    'agents.duplicate': {id: 'Go.agents.duplicate', defaultMessage: 'An agent named {name} already exists'},
    'agents.no-command': {id: 'Go.agents.no-command', defaultMessage: 'An agent of kind {kind} needs a launch command (the ACP agent\'s argv)'},
    'agents.no-launch': {id: 'Go.agents.no-launch', defaultMessage: 'Agent {name} ({kind}) has no launch command'},
    'agents.no-model': {id: 'Go.agents.no-model', defaultMessage: 'An agent that is a model needs the model\'s name'},
    'agents.no-name': {id: 'Go.agents.no-name', defaultMessage: 'The agent needs a name'},
    'agents.none': {id: 'Go.agents.none', defaultMessage: 'No agents are registered (the board\'s menu, Agents…)'},
    'agents.not-found': {id: 'Go.agents.not-found', defaultMessage: 'Agent {name} was not found'},
    'agents.not-yours': {id: 'Go.agents.not-yours', defaultMessage: 'Agent {name} is somebody else\'s'},
    'agents.off': {id: 'Go.agents.off', defaultMessage: 'Agents are off in this install'},
    'agents.unknown-kind': {id: 'Go.agents.unknown-kind', defaultMessage: 'Unknown agent kind {kind} (one of: {kinds})'},
    'agents.unknown-provider': {id: 'Go.agents.unknown-provider', defaultMessage: 'Unknown model provider {provider} (one of: {providers})'},
    'agents.which': {id: 'Go.agents.which', defaultMessage: 'Say which agent: several are registered'},
    'ai.cut-off': {id: 'Go.ai.cut-off', defaultMessage: 'The model\'s reply was cut off at its length limit'},
    'ai.key-empty': {id: 'Go.ai.key-empty', defaultMessage: 'The key is empty'},
    'ai.key-refused': {id: 'Go.ai.key-refused', defaultMessage: 'The provider did not accept the key: {reason}'},
    'ai.no-key': {id: 'Go.ai.no-key', defaultMessage: 'There is no API key for {provider}: add one in Settings → AI'},
    'ai.nobody': {id: 'Go.ai.nobody', defaultMessage: 'Sign in first'},
    'ai.off': {id: 'Go.ai.off', defaultMessage: 'AI is switched off in the app'},
    'ai.refused': {id: 'Go.ai.refused', defaultMessage: 'The model declined the task'},
    'ai.too-many-rounds': {id: 'Go.ai.too-many-rounds', defaultMessage: 'The model called tools {n} times without finishing'},
    'ai.unknown-provider': {id: 'Go.ai.unknown-provider', defaultMessage: 'Unknown provider {provider}'},
    'attach.bad-address': {id: 'Go.attach.bad-address', defaultMessage: '{address} is not an address'},
    'attach.bad-scheme': {id: 'Go.attach.bad-scheme', defaultMessage: '{address}: the address has to start with http or https'},
    'attach.login-failed': {id: 'Go.attach.login-failed', defaultMessage: 'Could not log in to {server}'},
    'attach.no-address': {id: 'Go.attach.no-address', defaultMessage: 'Enter the server\'s address'},
    'attach.no-host': {id: 'Go.attach.no-host', defaultMessage: '{address}: the address has no host'},
    'attach.no-session': {id: 'Go.attach.no-session', defaultMessage: '{server} did not give a session'},
    'attach.no-username': {id: 'Go.attach.no-username', defaultMessage: 'Enter a username'},
    'attach.no-vault': {id: 'Go.attach.no-vault', defaultMessage: 'There is nowhere on this machine to keep the session'},
    'attach.not-attached': {id: 'Go.attach.not-attached', defaultMessage: 'This desktop is not connected to {server}'},
    'attach.save-session': {id: 'Go.attach.save-session', defaultMessage: 'Could not save the session'},
    'attach.unavailable': {id: 'Go.attach.unavailable', defaultMessage: 'Connecting to a server is not available in this build'},
    'board.unavailable': {id: 'Go.board.unavailable', defaultMessage: 'The board is not available'},
    'carddiff.elsewhere': {id: 'Go.carddiff.elsewhere', defaultMessage: 'Work on this card started on another machine, and its working copy is not here'},
    'carddiff.folder-gone': {id: 'Go.carddiff.folder-gone', defaultMessage: 'The folder {dir} is no longer on the disk'},
    'carddiff.not-git': {id: 'Go.carddiff.not-git', defaultMessage: 'The folder {dir} is not a git repository, so there are no changes to show'},
    'carddiff.not-started': {id: 'Go.carddiff.not-started', defaultMessage: 'Nobody has worked on this card yet: there is no branch and no working copy'},
    'carddiff.nothing-changed': {id: 'Go.carddiff.nothing-changed', defaultMessage: 'Nothing has been changed on this card yet'},
    'folders.added-elsewhere': {id: 'Go.folders.added-elsewhere', defaultMessage: 'The folder is already added as {name}: mark it shared to have it on this board too'},
    'folders.busy-branch': {id: 'Go.folders.busy-branch', defaultMessage: 'The folder is held by another card ({card}). It is let go when that card\'s branch is merged into the main one; move this card again after that'},
    'folders.busy-session': {id: 'Go.folders.busy-session', defaultMessage: 'Another card\'s session ({card}) is already working in the folder {dir}: wait for it to finish'},
    'folders.dirty': {id: 'Go.folders.dirty', defaultMessage: 'The folder {dir} has unsaved changes'},
    'folders.gone': {id: 'Go.folders.gone', defaultMessage: 'Folder {name} points at {path}, which does not exist'},
    'folders.missing': {id: 'Go.folders.missing', defaultMessage: 'There is no folder at {path}'},
    'folders.name-taken': {id: 'Go.folders.name-taken', defaultMessage: 'The name {name} is taken ({path})'},
    'folders.no-picker': {id: 'Go.folders.no-picker', defaultMessage: 'A server has no folder picker: type the path'},
    'folders.none': {id: 'Go.folders.none', defaultMessage: 'The card names no folder, and there are no folders in the registry (the board\'s menu, Folders…)'},
    'folders.not-absolute': {id: 'Go.folders.not-absolute', defaultMessage: 'The path has to be absolute: {path}'},
    'folders.not-found': {id: 'Go.folders.not-found', defaultMessage: 'Folder {name} was not found'},
    'folders.not-in-registry': {id: 'Go.folders.not-in-registry', defaultMessage: 'Folder {name} is not in the registry ({folders})'},
    'folders.owned': {id: 'Go.folders.owned', defaultMessage: 'Folder {name} already belongs to a board'},
    'folders.unchosen': {id: 'Go.folders.unchosen', defaultMessage: 'The card\'s folder field names none of the registry\'s folders ({folders})'},
    'inbox.local-only': {id: 'Go.inbox.local-only', defaultMessage: 'The inbox is this machine\'s board, not the server\'s'},
    'inboxsort.bad-answer': {id: 'Go.inboxsort.bad-answer', defaultMessage: 'The agent\'s answer could not be read'},
    'inboxsort.move-failed': {id: 'Go.inboxsort.move-failed', defaultMessage: 'The move failed: {error}'},
    'inboxsort.no-agent': {id: 'Go.inboxsort.no-agent', defaultMessage: 'No agent is chosen to sort the inbox'},
    'inboxsort.no-agents': {id: 'Go.inboxsort.no-agents', defaultMessage: 'Agents are not available, so nobody can sort the inbox'},
    'inboxsort.no-answer': {id: 'Go.inboxsort.no-answer', defaultMessage: 'The agent said nothing about this card'},
    'inboxsort.no-boards': {id: 'Go.inboxsort.no-boards', defaultMessage: 'There are no boards besides the inbox, so there is nowhere to carry cards'},
    'inboxsort.no-inbox': {id: 'Go.inboxsort.no-inbox', defaultMessage: 'There is no inbox board'},
    'inboxsort.unavailable': {id: 'Go.inboxsort.unavailable', defaultMessage: 'Sorting the inbox is not available: sources are off'},
    'mcp.bad-name': {id: 'Go.mcp.bad-name', defaultMessage: 'The MCP server name {name} may contain only Latin letters, digits, hyphens and underscores'},
    'mcp.duplicate': {id: 'Go.mcp.duplicate', defaultMessage: 'There is already an MCP server named {name}'},
    'mcp.name-builtin': {id: 'Go.mcp.name-builtin', defaultMessage: 'The name {name} belongs to a built-in server'},
    'mcp.no-command': {id: 'Go.mcp.no-command', defaultMessage: 'The MCP server {name} has no launch command (command)'},
    'mcp.no-name': {id: 'Go.mcp.no-name', defaultMessage: 'An MCP server has no name'},
    'mcp.no-processes-here': {id: 'Go.mcp.no-processes-here', defaultMessage: 'The MCP server {name} is a program to start, and this server starts none: give its address (url) instead'},
    'mcp.remote': {id: 'Go.mcp.remote', defaultMessage: 'The server {name} is remote (url/type), and only local servers are supported so far: command + args'},
    'oauth.no-browser': {id: 'Go.oauth.no-browser', defaultMessage: 'Could not open the browser'},
    'oauth.no-refresh': {id: 'Go.oauth.no-refresh', defaultMessage: 'There is no refresh token: log in again'},
    'oauth.timeout': {id: 'Go.oauth.timeout', defaultMessage: 'The login was not finished within {timeout}'},
    'planning.busy': {id: 'Go.planning.busy', defaultMessage: 'The agent has not answered the earlier messages yet'},
    'planning.empty': {id: 'Go.planning.empty', defaultMessage: 'The message is empty'},
    'planning.over': {id: 'Go.planning.over', defaultMessage: 'The conversation is over'},
    'proxies.agent-cannot': {id: 'Go.proxies.agent-cannot', defaultMessage: 'Agent {name} ({kind}) could not use this configuration'},
    'proxies.agent-missing': {id: 'Go.proxies.agent-missing', defaultMessage: 'The proxy configuration of agent {name} is not in the registry (there are: {proxies})'},
    'proxies.agent-missing-none': {id: 'Go.proxies.agent-missing-none', defaultMessage: 'The proxy configuration of agent {name} is not in the registry, and the registry is empty'},
    'proxies.bad-address': {id: 'Go.proxies.bad-address', defaultMessage: 'Could not read the proxy address {address}'},
    'proxies.credentials-alone': {id: 'Go.proxies.credentials-alone', defaultMessage: 'A login and a password are given without a proxy address'},
    'proxies.duplicate': {id: 'Go.proxies.duplicate', defaultMessage: 'A configuration named {name} already exists'},
    'proxies.empty': {id: 'Go.proxies.empty', defaultMessage: 'The configuration {name} is empty: give it a proxy, a list of exceptions or a CA certificate'},
    'proxies.in-use': {id: 'Go.proxies.in-use', defaultMessage: 'Agents use the configuration {name}: {agents}. Switch them to another one first'},
    'proxies.no-name': {id: 'Go.proxies.no-name', defaultMessage: 'The configuration needs a name'},
    'proxies.no-scheme': {id: 'Go.proxies.no-scheme', defaultMessage: 'The proxy address {address} lacks a scheme or a host, for example http://{address}'},
    'proxies.not-found': {id: 'Go.proxies.not-found', defaultMessage: 'Configuration {name} was not found'},
    'proxies.socks-claude': {id: 'Go.proxies.socks-claude', defaultMessage: 'Claude Code does not support SOCKS proxies: give an http(s):// address, or wrap the CLI in a launch command (command)'},
    'question.allow': {id: 'Go.question.allow', defaultMessage: 'Allow the agent\'s action?'},
    'question.allow-command': {id: 'Go.question.allow-command', defaultMessage: 'Run the command: {command}?'},
    'question.allow-title': {id: 'Go.question.allow-title', defaultMessage: 'Allow: {title}?'},
    'question.allow-tool': {id: 'Go.question.allow-tool', defaultMessage: 'Allow {tool}?'},
    'question.allow-tool-title': {id: 'Go.question.allow-tool-title', defaultMessage: 'Allow {tool}: {title}?'},
    'question.answering': {id: 'Go.question.answering', defaultMessage: 'This question is already being answered'},
    'question.gone': {id: 'Go.question.gone', defaultMessage: 'This question is no longer open'},
    'secrets.env-readonly': {id: 'Go.secrets.env-readonly', defaultMessage: 'A secret from the environment cannot be changed from the app'},
    'secrets.env-undeletable': {id: 'Go.secrets.env-undeletable', defaultMessage: 'A secret from the environment cannot be deleted from the app'},
    'secrets.nowhere': {id: 'Go.secrets.nowhere', defaultMessage: 'There is nowhere on this machine to keep the secret'},
    'setup.needs-git': {id: 'Go.setup.needs-git', defaultMessage: 'The folder {dir} has no git, and this board needs a repository: it publishes a branch or waits for one. Run `git init` or choose another folder'},
    'share.empty': {id: 'Go.share.empty', defaultMessage: 'Nothing to save: there is neither a link nor a title'},
    'sources.agents-off': {id: 'Go.sources.agents-off', defaultMessage: 'Agents are off, and source {name} is read by an agent'},
    'sources.bad-expression': {id: 'Go.sources.bad-expression', defaultMessage: 'Could not read the expression {expression}'},
    'sources.bad-item': {id: 'Go.sources.bad-item', defaultMessage: 'The plugin sent an item that could not be read: {error}'},
    'sources.comment-no-card': {id: 'Go.sources.comment-no-card', defaultMessage: 'The rule comments, but there is no card yet'},
    'sources.connected': {id: 'Go.sources.connected', defaultMessage: 'The source is connected to the service'},
    'sources.disconnected': {id: 'Go.sources.disconnected', defaultMessage: 'The source is disconnected from the service'},
    'sources.duplicate': {id: 'Go.sources.duplicate', defaultMessage: 'There is already a source named {name}'},
    'sources.empty-token': {id: 'Go.sources.empty-token', defaultMessage: 'The token is empty'},
    'sources.no-agent': {id: 'Go.sources.no-agent', defaultMessage: 'No agent is chosen to read the source'},
    'sources.no-login': {id: 'Go.sources.no-login', defaultMessage: 'Plugin {name} needs no login to a service'},
    'sources.no-name': {id: 'Go.sources.no-name', defaultMessage: 'The source needs a name'},
    'sources.no-task': {id: 'Go.sources.no-task', defaultMessage: 'The source does not say what to ask the service'},
    'sources.no-vault': {id: 'Go.sources.no-vault', defaultMessage: 'There is nowhere on this machine to keep the token'},
    'sources.not-found': {id: 'Go.sources.not-found', defaultMessage: 'Source {name} was not found'},
    'sources.off': {id: 'Go.sources.off', defaultMessage: 'Source {name} is switched off'},
    'sources.plugin-no-command': {id: 'Go.sources.plugin-no-command', defaultMessage: 'Plugin {name} does not say how to run it (command)'},
    'sources.plugin-no-name': {id: 'Go.sources.plugin-no-name', defaultMessage: 'The plugin has no name'},
    'sources.plugin-not-found': {id: 'Go.sources.plugin-not-found', defaultMessage: 'Plugin {name} was not found'},
    'sources.plugin-unknown': {id: 'Go.sources.plugin-unknown', defaultMessage: 'Plugin {name} is not registered'},
    'sources.protocol-newer': {id: 'Go.sources.protocol-newer', defaultMessage: 'The plugin speaks protocol version {plugin}, and this app understands {app}: update the app'},
    'sources.token-refresh': {id: 'Go.sources.token-refresh', defaultMessage: 'Could not refresh the token: {error}'},
    'sources.unavailable': {id: 'Go.sources.unavailable', defaultMessage: 'Sources are not available'},
    'stall.assigned-to-person': {id: 'Go.stall.assigned-to-person', defaultMessage: 'The card is assigned to {who}, so no agent starts. Remove the assignee, or assign an agent if one should take the work'},
    'updates.nothing-to-skip': {id: 'Go.updates.nothing-to-skip', defaultMessage: 'There is no update to skip'},
    'updates.restart-failed': {id: 'Go.updates.restart-failed', defaultMessage: 'Could not restart'},
    'updates.unavailable': {id: 'Go.updates.unavailable', defaultMessage: 'This build does not update itself'},
})

// goMessage words what Go said in the page's language.
export function goMessage(intl: IntlShape, said: GoMessage | null | undefined): string {
    if (!said) {
        return ''
    }
    const message = said.code ? messages[said.code] : undefined
    return message ? intl.formatMessage(message, said.args) : said.text || ''
}

// goError is what a failed binding call says: the code and values Go sent as
// the error's cause, else the error's own text.
export function goError(intl: IntlShape, e: unknown): string {
    const cause = (e as {cause?: {code?: string, args?: Record<string, string>, message?: string}} | null)?.cause
    if (cause && typeof cause === 'object' && cause.code) {
        return goMessage(intl, {code: cause.code, args: cause.args, text: cause.message})
    }
    return (e as Error | null)?.message || String(e)
}

// goSentences are the sentences Go has to word by itself, with their slots
// left in, for the codes that reach the OS notification without this page
// (gowords.go).
export function goSentences(intl: IntlShape): Record<string, string> {
    const slots: Record<string, string[]> = {
        'question.allow': [],
        'question.allow-title': ['title'],
        'question.allow-tool': ['tool'],
        'question.allow-tool-title': ['tool', 'title'],
    }
    const out: Record<string, string> = {}
    for (const [code, names] of Object.entries(slots)) {
        out[code] = intl.formatMessage(messages[code], Object.fromEntries(names.map((name) => [name, `{${name}}`])))
    }
    return out
}
