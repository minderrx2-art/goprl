package buildinfo

import (
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	vcs := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "embedded-sha"}}}
	for _, tc := range []struct {
		name, commit, buildTime, wantCommit, wantTime string
		vcs                                           *debug.BuildInfo
	}{
		{"linker overrides VCS", "linked-sha", "2026-09-27T12:00:00Z", "linked-sha", "2026-09-27T12:00:00Z", vcs},
		{"embedded revision", "", "", "embedded-sha", "unknown", vcs},
		{"missing metadata", "", "", "unknown", "unknown", nil},
		{"missing revision", "", "", "unknown", "unknown", &debug.BuildInfo{}},
		{"UTC normalization", "sha", "2026-09-27T13:00:00+01:00", "sha", "2026-09-27T12:00:00Z", nil},
		{"invalid timestamp", "sha", "invalid", "sha", "unknown", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commit, buildTime := resolve(tc.commit, tc.buildTime, tc.vcs)
			if commit != tc.wantCommit || buildTime != tc.wantTime {
				t.Errorf("got (%q, %q), want (%q, %q)", commit, buildTime, tc.wantCommit, tc.wantTime)
			}
		})
	}
}

func TestCurrent(t *testing.T) {
	info := Current()
	if info.Commit == "" || info.BuildTime == "" || info.GoVersion != runtime.Version() {
		t.Fatalf("unexpected metadata: %+v", info)
	}
	if Commit != "" && info.Commit != Commit {
		t.Errorf("linker commit %q not used: %q", Commit, info.Commit)
	}
	if BuildTime != "" && info.BuildTime != BuildTime {
		t.Errorf("linker build time %q not used: %q", BuildTime, info.BuildTime)
	}
	if info.StartedAt != startedAt.UTC().Format(time.RFC3339) || info.UptimeSeconds < 0 {
		t.Errorf("unexpected startup metadata: %+v", info)
	}
}

func TestUptimeUsesStartupTime(t *testing.T) {
	original := startedAt
	startedAt = time.Now().Add(-10 * time.Second)
	t.Cleanup(func() { startedAt = original })
	if got := Current().UptimeSeconds; got < 10 || got > 11 {
		t.Errorf("expected approximately 10 seconds uptime, got %d", got)
	}
}
