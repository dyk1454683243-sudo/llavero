package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name:    "stamped release matches the journal example",
			version: "v0.1.0",
			commit:  "230caa9",
			date:    "2026-09-17",
			want:    "llavero v0.1.0 (230caa9, 2026-09-17)",
		},
		{
			name:    "ldflags without a v prefix still print one",
			version: "0.1.0",
			commit:  "230caa9",
			date:    "2026-09-17",
			want:    "llavero v0.1.0 (230caa9, 2026-09-17)",
		},
		{
			name:    "dev default is not rewritten to vdev",
			version: "dev",
			commit:  "none",
			date:    "unknown",
			want:    "llavero dev (none, unknown)",
		},
		{
			name:    "go install pseudo-version is left intact",
			version: "v0.0.0-20260917120000-230caa9abcdef",
			commit:  "230caa9",
			date:    "2026-09-17",
			want:    "llavero v0.0.0-20260917120000-230caa9abcdef (230caa9, 2026-09-17)",
		},
		{
			name:    "empty version falls back to dev",
			version: "",
			commit:  "deadbee",
			date:    "2026-01-02",
			want:    "llavero dev (deadbee, 2026-01-02)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVersion(tt.version, tt.commit, tt.date)
			if got != tt.want {
				t.Fatalf("formatVersion(%q, %q, %q) = %q, want %q",
					tt.version, tt.commit, tt.date, got, tt.want)
			}
		})
	}
}

func TestApplyBuildSettingsFillsDefaults(t *testing.T) {
	version, commit, date := "dev", "none", "unknown"
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.2.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "230caa9abcdef0123456789"},
			{Key: "vcs.time", Value: "2026-09-17T21:12:23Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}

	applyBuildSettings(&version, &commit, &date, info)

	if version != "v0.2.0" {
		t.Errorf("version = %q, want v0.2.0 from Main.Version", version)
	}
	if commit != "230caa9" {
		t.Errorf("commit = %q, want the short vcs.revision", commit)
	}
	if date != "2026-09-17" {
		t.Errorf("date = %q, want 2026-09-17 from vcs.time", date)
	}
}

func TestApplyBuildSettingsMarksDirtyCommit(t *testing.T) {
	version, commit, date := "dev", "none", "unknown"
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	applyBuildSettings(&version, &commit, &date, info)

	if version != "dev" {
		t.Errorf("version = %q, want dev; (devel) is not a real version", version)
	}
	if commit != "abcdef1-dirty" {
		t.Errorf("commit = %q, want abcdef1-dirty", commit)
	}
}

func TestApplyBuildSettingsDoesNotOverrideLdflags(t *testing.T) {
	version, commit, date := "v0.1.0", "230caa9", "2026-09-17"
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "v9.9.9"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "ffffffff"},
			{Key: "vcs.time", Value: "1999-01-01T00:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}

	applyBuildSettings(&version, &commit, &date, info)

	if version != "v0.1.0" || commit != "230caa9" || date != "2026-09-17" {
		t.Fatalf("ldflags were overwritten: version=%q commit=%q date=%q",
			version, commit, date)
	}
}

func TestApplyBuildSettingsKeepsUnknownDateWithoutVCS(t *testing.T) {
	version, commit, date := "dev", "none", "unknown"
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	}

	applyBuildSettings(&version, &commit, &date, info)

	if version != "dev" || commit != "none" || date != "unknown" {
		t.Fatalf("defaults changed without VCS metadata: version=%q commit=%q date=%q",
			version, commit, date)
	}
}

func TestParseFirmwareVersion(t *testing.T) {
	tests := []struct {
		in   string
		want uint32
	}{
		{"dev", 0},
		{"", 0},
		{"none", 0},
		{"v0.1.0", 0<<16 | 1<<8 | 0},
		{"0.1.0", 0<<16 | 1<<8 | 0},
		{"v1.2.3", 1<<16 | 2<<8 | 3},
		{"v2.0", 2 << 16},
		{"v3", 3 << 16},
		{"v0.1.0-rc.1", 0<<16 | 1<<8 | 0},
		{"v0.0.0-20260917120000-230caa9abcdef", 0},
	}
	for _, tt := range tests {
		if got := parseFirmwareVersion(tt.in); got != tt.want {
			t.Errorf("parseFirmwareVersion(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// TestVersionFlagWithLdflags builds a real binary the way a release would, so
// a mistake in the -X paths or the flag name cannot hide behind a unit test
// that never reaches main().
func TestVersionFlagWithLdflags(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "llavero")
	ldflags := "-X main.version=v0.1.0 -X main.commit=230caa9 -X main.date=2026-09-17"
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", bin, ".")
	build.Env = append(os.Environ(), "GOFLAGS=")
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "-version")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("llavero -version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	want := "llavero v0.1.0 (230caa9, 2026-09-17)"
	if got != want {
		t.Fatalf("llavero -version printed %q, want %q", got, want)
	}
}
