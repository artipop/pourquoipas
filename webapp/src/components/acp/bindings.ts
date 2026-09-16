// agentBindings returns the Wails-injected ACP bindings, or undefined in
// browser/plugin deployments (mirrors the webSocketBaseURL guard pattern).
//
// It lives in a module of its own because every ACP component needs it and none
// of them needs the others: it used to be exported from the folders dialog,
// which made that file a dependency of the whole feature for one line.
export function agentBindings() {
    return (window as unknown as import('../../types').IAppWindow).go?.main?.App
}

// Whether this build has the route editor at all. Here rather than beside the
// dialog for the reason above, one size larger: the view menu asks the question
// on every render, and asking it in the dialog's own module meant every board
// loaded the route canvas — solid-flow, xyflow and d3 — to find out whether to
// draw one menu item.
export function isAutomationAvailable(): boolean {
    return Boolean(agentBindings()?.ListFlows)
}
