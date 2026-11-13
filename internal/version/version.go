// Package version holds build information.
package version

import "runtime/debug"

// Version is set with -ldflags "-X github.com/boringemployee/be/internal/version.Version=v1.2.3"
// by the Makefile and release builds. "go install module@v1.2.3" does not set
// it, so String falls back to the module version Go records in the binary.
var Version = "dev"

// readBuildInfo is replaced in tests.
var readBuildInfo = debug.ReadBuildInfo

// String returns the human-readable version banner.
func String() string {
	v := Version
	if v == "dev" {
		if info, ok := readBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	return "be " + v
}
