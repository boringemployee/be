package version

import (
	"runtime/debug"
	"testing"
)

func TestString(t *testing.T) {
	oldV, oldRead := Version, readBuildInfo
	t.Cleanup(func() { Version, readBuildInfo = oldV, oldRead })

	info := func(v string, ok bool) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Main: debug.Module{Version: v}}, ok
		}
	}
	tests := []struct {
		name    string
		version string
		read    func() (*debug.BuildInfo, bool)
		want    string
	}{
		{"set by ldflags", "v0.1.0", info("v9.9.9", true), "be v0.1.0"},
		{"go install module@version", "dev", info("v0.2.0", true), "be v0.2.0"},
		{"local build", "dev", info("(devel)", true), "be dev"},
		{"empty module version", "dev", info("", true), "be dev"},
		{"no build info", "dev", info("v1.0.0", false), "be dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, readBuildInfo = tt.version, tt.read
			if got := String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
