package db

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

type fakeStore struct {
	applied map[int]bool
	ran     []int
	failOn  int
	initErr error
}

func (f *fakeStore) Init(context.Context) error { return f.initErr }
func (f *fakeStore) Applied(context.Context) (map[int]bool, error) {
	out := map[int]bool{}
	for k, v := range f.applied {
		out[k] = v
	}
	return out, nil
}
func (f *fakeStore) Apply(_ context.Context, m Migration) error {
	if m.Version == f.failOn {
		return errors.New("syntax error")
	}
	f.applied[m.Version] = true
	f.ran = append(f.ran, m.Version)
	return nil
}

func three() []Migration {
	return []Migration{{1, "a", "x"}, {2, "b", "y"}, {3, "c", "z"}}
}

func TestMigrateAppliesPendingInOrder(t *testing.T) {
	s := &fakeStore{applied: map[int]bool{1: true}}
	res, err := migrate(context.Background(), s, three())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ran) != 2 || s.ran[0] != 2 || s.ran[1] != 3 || res.Current != 3 || len(res.Applied) != 2 {
		t.Errorf("ran=%v res=%+v", s.ran, res)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := &fakeStore{applied: map[int]bool{}}
	if _, err := migrate(context.Background(), s, three()); err != nil {
		t.Fatal(err)
	}
	s.ran = nil
	res, err := migrate(context.Background(), s, three())
	if err != nil || len(s.ran) != 0 || res.Current != 3 {
		t.Errorf("second run: ran=%v res=%+v err=%v", s.ran, res, err)
	}
}

func TestMigrateStopsOnFailure(t *testing.T) {
	s := &fakeStore{applied: map[int]bool{}, failOn: 2}
	res, err := migrate(context.Background(), s, three())
	if err == nil || !strings.Contains(err.Error(), "0002_b") {
		t.Fatalf("err = %v", err)
	}
	if res.Current != 1 || s.applied[3] {
		t.Errorf("should stop after 1: res=%+v applied=%v", res, s.applied)
	}
}

func TestMigrateRejectsUnknownAppliedVersion(t *testing.T) {
	s := &fakeStore{applied: map[int]bool{1: true, 2: true, 3: true, 4: true}}
	if _, err := migrate(context.Background(), s, three()); err == nil || !strings.Contains(err.Error(), "migration 4") {
		t.Errorf("err = %v", err)
	}
}

func TestLoadMigrationsValidation(t *testing.T) {
	ok := fstest.MapFS{
		"m/0002_second.sql": {Data: []byte("B")},
		"m/0001_first.sql":  {Data: []byte("A")},
		"m/README.md":       {Data: []byte("ignored")},
	}
	ms, err := loadMigrations(ok, "m")
	if err != nil || len(ms) != 2 || ms[0].Name != "first" || ms[1].SQL != "B" {
		t.Fatalf("ms=%+v err=%v", ms, err)
	}
	bad := map[string]fstest.MapFS{
		"gap":       {"m/0001_a.sql": {}, "m/0003_c.sql": {}},
		"duplicate": {"m/0001_a.sql": {}, "m/1_b.sql": {}},
		"bad name":  {"m/first.sql": {}},
		"zero":      {"m/0000_a.sql": {}},
	}
	for name, fsys := range bad {
		if _, err := loadMigrations(fsys, "m"); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestEmbeddedMigrationsLoad(t *testing.T) {
	ms, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 || ms[0].Version != 1 || !strings.Contains(ms[0].SQL, "CREATE TABLE edition_types") {
		t.Errorf("unexpected embedded migrations: %d", len(ms))
	}
}
