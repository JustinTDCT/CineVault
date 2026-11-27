package config

import "testing"

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
