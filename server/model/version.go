package model

// CurrentVersion is what this application calls itself.
//
// It is a constant rather than an -ldflags injection for the reason version.go
// in the root gives: threading a value through four Taskfiles makes the version
// a property of how the binary was built, which is exactly what `wails3 dev`
// then gets wrong. This file is one of the places internal/buildversion keeps
// in step, and version_test.go fails when they disagree.
//
// What was here was Focalboard's own list of twenty-odd released versions, with
// CurrentVersion set to the newest of them — so /ping answered "8.0.0" for an
// application whose own version is elsewhere and different. A version list of a
// product this one is not is not history, it is a wrong answer with a
// changelog.
const CurrentVersion = "1.0.0"
