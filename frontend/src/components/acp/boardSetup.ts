// The Wails-generated Go bindings are PascalCase methods, not constructors.
/* eslint-disable new-cap */
import type {Accessor} from 'solid-js'

import {createLoader} from '../../hooks/loader'

import {Board} from '../../blocks/board'

import {goApp} from '../../goBridge'
import {aiEnabled} from '../../aiSwitch'

// What a board needs answered before its automation can run is resolved on the
// Go side (internal/acp/setup.go) out of what the board asks for, what its
// columns imply and what this machine already has. The page reads that plan and
// renders it; it works nothing out for itself, because a second opinion here
// would be a second answer to drift from the first.

export type SetupStepKind = 'name' | 'agent' | 'deploy' | 'browser' | 'done'
export type SetupStepStatus = 'pending' | 'done' | 'skipped'

export type SetupStep = {
    kind: SetupStepKind
    optional: boolean
    hint?: string
    status: SetupStepStatus

    // The machine can already answer this step — there is an agent.
    // Not the same as this board having answered it: the registries are the
    // machine's, and every board is asked for itself.
    ready?: boolean
}

export type SetupPlan = {
    boardId: string
    steps: SetupStep[]

    // What this board calls the column a card is dragged into to be worked on.

    // What this board calls the column that tests. The QA step names an agent
    // for it, so the question can say which column it is about.

    // Who already works this board's cards: the crew of its agent stages, when
    // they agree on one. The agent step opens on it, so walking the wizard
    // again shows the answer already given rather than nobody chosen.

    // The board named these steps itself, rather than them being worked out
    // from the automation it carries.
    declared: boolean

    // The board brings columns or routes of its own. One that brings none is
    // not a board the wizard should open itself for.
    automated: boolean

    // The wizard has already opened itself for this board once.
    offered?: boolean
}

// The plan a board with nothing to say gets: no steps at all, so nothing is
// offered and nothing is asked. It is what a browser or plugin build sees,
// where there is no machine to set up in the first place.
export const NO_SETUP: SetupPlan = {boardId: '', steps: [], declared: false, automated: false, offered: false}

// The setup is agents' questions — where to work, who works, where it goes —
// so with the AI switched off it is not offered at all (the plan below keeps
// only the board's name).
export function isBoardSetupAvailable(): boolean {
    return aiEnabled() && Boolean(goApp()?.BoardSetupPlan)
}

export async function readSetupPlan(boardId: string): Promise<SetupPlan> {
    const bindings = goApp()
    if (!bindings?.BoardSetupPlan || !boardId) {
        return NO_SETUP
    }
    const plan = JSON.parse(await bindings.BoardSetupPlan(boardId)) as SetupPlan
    return {...plan, steps: plan.steps || []}
}

// NO_SETUP stands in until Go answers: a board that asks nothing and a board
// not read yet look the same on screen, and every caller reads the plan
// straight through.
export function createSetupPlan(board: () => Board | undefined): [Accessor<SetupPlan>, () => void] {
    const [plan, reload] = createLoader((boardId: string) => readSetupPlan(boardId).catch(() => NO_SETUP), NO_SETUP, () => board()?.id ?? '')
    return [() => (aiEnabled() ? plan() : withoutAgents(plan())), reload]
}

// withoutAgents is the plan a board has while the AI is switched off: its name
// is still a question about the board, everything else is a question about
// agents. Not automated, so the wizard never opens itself either.
export function withoutAgents(plan: SetupPlan): SetupPlan {
    return {...plan, steps: plan.steps.filter((step) => step.kind === 'name'), automated: false}
}

// Recording is for the answer no registry can be read for: a step deliberately
// passed over. Everything else is answered by the registry the step fills.
export async function recordSetupStep(boardId: string, step: SetupStepKind, status: SetupStepStatus): Promise<void> {
    const bindings = goApp()
    if (!bindings?.RecordBoardSetupStep || !boardId) {
        return
    }
    await bindings.RecordBoardSetupStep(boardId, step, status)
}

export function planHasStep(plan: SetupPlan | undefined, kind: SetupStepKind): boolean {
    return Boolean(plan?.steps.some((step) => step.kind === kind))
}

// setupNeeded says this board runs something this machine cannot run yet: a
// step nobody may skip is still waiting for an answer.
export function setupNeeded(plan: SetupPlan | undefined): boolean {
    return Boolean(plan?.automated && plan.steps.some((step) => !step.optional && step.status === 'pending'))
}

// shouldOfferSetup is the rule for opening the wizard by itself, and it fires
// once per board — on the first board you open after making it. It used to fire
// on every launch until the setup was finished or refused, which meant the app
// greeted you with a modal every morning for as long as you had not got round
// to it. A thing you have already seen and closed is a reminder, not a dialog.
export function shouldOfferSetup(plan: SetupPlan | undefined): boolean {
    return setupNeeded(plan) && !plan?.offered
}

// Remembered by Go, in the store, because the page cannot: the desktop app
// serves itself on a fresh port every launch, so localStorage is keyed by an
// origin that does not survive a restart — which made "do not show this again"
// last exactly one run.
export async function markSetupOffered(boardId: string): Promise<void> {
    const bindings = goApp()
    if (!bindings?.MarkBoardSetupOffered || !boardId) {
        return
    }
    await bindings.MarkBoardSetupOffered(boardId)
}
