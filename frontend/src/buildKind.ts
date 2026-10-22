// What this install is, as the Go side said before the first render
// (GetMachineSettings): a hosted server has no machine of anybody's to work
// on, so nothing that needs one — a folder, a CLI agent, a proxy — is offered
// there. Not a setting: it is the build.
let hosted = false

export function isHosted(): boolean {
    return hosted
}

export function setHosted(value: boolean): void {
    hosted = value
}
