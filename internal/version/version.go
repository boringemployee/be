// Package version holds build information injected at link time.
package version

// Version is set with -ldflags "-X github.com/boringemployee/be/internal/version.Version=v1.2.3".
var Version = "dev"

// String returns the human-readable version banner.
func String() string {
	return "be " + Version
}
