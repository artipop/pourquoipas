import {goApp} from '../../goBridge'

// Whether this build has the route editor at all. In a module of its own
// rather than beside the dialog: the view menu asks the question on every
// render, and asking it in the dialog's own module meant every board loaded
// the route canvas — solid-flow, xyflow and d3 — to find out whether to draw
// one menu item.
export function isAutomationAvailable(): boolean {
    return Boolean(goApp()?.ListFlows)
}
