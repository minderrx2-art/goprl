package buildinfo

import (
	"runtime"
	"runtime/debug"
	"time"
)

// Commit and BuildTime can be set at build time using -ldflags -X.
var (
	Commit    string
	BuildTime string
	startedAt = time.Now()
)

type Info struct {
	Commit        string `json:"commit"`
	BuildTime     string `json:"build_time"`
	StartedAt     string `json:"started_at"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	GoVersion     string `json:"go_version"`
}

func Current() Info {
	vcs, _ := debug.ReadBuildInfo()
	commit, buildTime := resolve(Commit, BuildTime, vcs)
	return Info{
		Commit:        commit,
		BuildTime:     buildTime,
		StartedAt:     startedAt.UTC().Format(time.RFC3339),
		UptimeSeconds: int64(time.Since(startedAt) / time.Second),
		GoVersion:     runtime.Version(),
	}
}

func resolve(commit, buildTime string, vcs *debug.BuildInfo) (string, string) {
	if commit == "" && vcs != nil {
		for _, setting := range vcs.Settings {
			if setting.Key == "vcs.revision" {
				commit = setting.Value
				break
			}
		}
	}
	if commit == "" {
		commit = "unknown"
	}
	if timestamp, err := time.Parse(time.RFC3339, buildTime); err == nil {
		buildTime = timestamp.UTC().Format(time.RFC3339)
	} else {
		buildTime = "unknown"
	}
	return commit, buildTime
}
