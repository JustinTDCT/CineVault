package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := load(env(map[string]string{
		"CINEVAULT_DATABASE_URL": "postgres://cinevault:secret@127.0.0.1/cinevault",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != DefaultListen {
		t.Errorf("Listen = %q, want %q", c.Listen, DefaultListen)
	}
	if c.FFmpegPath != "ffmpeg" || c.FFprobePath != "ffprobe" {
		t.Errorf("FFmpegPath = %q, FFprobePath = %q", c.FFmpegPath, c.FFprobePath)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	if _, err := load(env(nil)); err == nil {
		t.Fatal("expected an error when CINEVAULT_DATABASE_URL is missing")
	}
}

func TestDatabaseAddress(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@127.0.0.1/db":       "127.0.0.1:5432",
		"postgresql://u:p@db.local:6543/db": "db.local:6543",
		"postgres://u:p@[::1]:5432/db":      "[::1]:5432",
	}
	for in, want := range cases {
		got, err := DatabaseAddress(in)
		if err != nil || got != want {
			t.Errorf("DatabaseAddress(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"mysql://u@h/db", "postgres:///db", "::"} {
		if _, err := DatabaseAddress(bad); err == nil {
			t.Errorf("DatabaseAddress(%q): expected error", bad)
		}
	}
}

func TestThrottleSettings(t *testing.T) {
	base := map[string]string{"CINEVAULT_DATABASE_URL": "postgres://u:p@127.0.0.1/db"}
	with := func(kv ...string) map[string]string {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	c, err := load(env(with("CINEVAULT_TRANSCODE_PAUSE_AHEAD", "5m", "CINEVAULT_TRANSCODE_RESUME_AHEAD", "120", "CINEVAULT_TRANSCODE_KEEP_BEHIND", "1m30s")))
	if err != nil || c.PauseAhead != 5*time.Minute || c.ResumeAhead != 2*time.Minute || c.KeepBehind != 90*time.Second {
		t.Errorf("got %v %v %v, %v", c.PauseAhead, c.ResumeAhead, c.KeepBehind, err)
	}
	if c, _ := load(env(base)); c.PauseAhead != 0 || c.ResumeAhead != 0 || c.KeepBehind != 0 {
		t.Error("unset settings should be zero, meaning defaults")
	}
	for _, bad := range [][]string{
		{"CINEVAULT_TRANSCODE_PAUSE_AHEAD", "soon"},
		{"CINEVAULT_TRANSCODE_KEEP_BEHIND", "-5s"},
		{"CINEVAULT_TRANSCODE_PAUSE_AHEAD", "1m", "CINEVAULT_TRANSCODE_RESUME_AHEAD", "2m"},
	} {
		if _, err := load(env(with(bad...))); err == nil {
			t.Errorf("%v: expected an error", bad)
		}
	}
}
