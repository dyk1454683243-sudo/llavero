package main

// Build identity. These three strings are the single source for the -version
// flag, the startup journal line, and (later) the CTAP 2.1 firmwareVersion
// field. Release builds stamp them with -ldflags:
//
//	-X main.version=v0.1.0
//	-X main.commit=$(git rev-parse --short HEAD)
//	-X main.date=$(date -u +%Y-%m-%d)
//
// A plain `go build` or `go install` leaves the defaults; applyBuildSettings
// then fills what it can from the VCS metadata Go embeds automatically, so
// both paths report something a bug report can use.

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if ok {
		applyBuildSettings(&version, &commit, &date, info)
	}
}

// applyBuildSettings fills any still-default field from Go's embedded build
// info. Values already set by -ldflags are left alone, so a release binary
// cannot be overwritten by whatever the linker happened to embed.
func applyBuildSettings(version, commit, date *string, info *debug.BuildInfo) {
	if isUnset(*version, "dev") {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			*version = v
		}
	}

	var (
		rev      string
		vcsTime  string
		modified bool
	)
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if isUnset(*commit, "none") && rev != "" {
		if len(rev) > 7 {
			rev = rev[:7]
		}
		if modified {
			rev += "-dirty"
		}
		*commit = rev
	}

	if isUnset(*date, "unknown") && vcsTime != "" {
		if t, err := time.Parse(time.RFC3339, vcsTime); err == nil {
			*date = t.UTC().Format("2006-01-02")
		} else {
			*date = vcsTime
		}
	}
}

func isUnset(val, fallback string) bool {
	return val == "" || val == fallback
}

// versionString is what -version prints and what the journal logs:
//
//	llavero v0.1.0 (230caa9, 2026-09-17)
func versionString() string {
	return formatVersion(version, commit, date)
}

func formatVersion(version, commit, date string) string {
	return fmt.Sprintf("llavero %s (%s, %s)", displayVersion(version), commit, date)
}

// displayVersion keeps a leading v on stamped releases so the journal matches
// the issue example, without turning the "dev" default into "vdev".
func displayVersion(v string) string {
	if v == "" {
		return "dev"
	}
	if v == "dev" || strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

// firmwareVersion is the unsigned integer CTAP 2.1 authenticatorGetInfo
// reports as key 0x0E. Packed as major<<16 | minor<<8 | patch so a later
// getInfo change can use this instead of inventing a second constant.
// Unparseable strings (including "dev") return 0.
func firmwareVersion() uint32 {
	return parseFirmwareVersion(version)
}

func parseFirmwareVersion(v string) uint32 {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var major, minor, patch uint32
	n, _ := fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	switch n {
	case 1:
		return major << 16
	case 2:
		return major<<16 | minor<<8
	case 3:
		return major<<16 | minor<<8 | patch
	default:
		return 0
	}
}
