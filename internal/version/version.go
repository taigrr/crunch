// Package version exposes the crunch build version.
package version

import "runtime/debug"

// Version is the crunch version. It defaults to the module version embedded by
// the Go toolchain and can be overridden at build time via
// -ldflags "-X github.com/taigrr/crunch/internal/version.Version=...".
var Version = "devel"

func init() {
	if Version != "devel" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = v
		}
	}
}
