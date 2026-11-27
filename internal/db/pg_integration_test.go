package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

// testDatabase creates a fresh, empty database for one test and drops it
// afterwards. It needs CINEVAULT_TEST_DATABASE_URL (run `make test-db`); the
// user in that URL must be allowed to create databases.
func testDatabase(t *testing.T) *DB {
	t.Helper()
	base := os.Getenv("CINEVAULT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set CINEVAULT_TEST_DATABASE_URL to run database tests (make test-db)")
	}
	ctx := context.Background()

	admin, err := Open(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("cinevault_test_%d", time.Now().UnixNano())
	_, err = admin.Pool.Exec(ctx, "CREATE DATABASE "+name)
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	d, err := Open(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		d.Close()
		a, err := Open(context.Background(), base)
		if err != nil {
			t.Logf("could not drop %s: %v", name, err)
			return
		}
		defer a.Close()
		if _, err := a.Pool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("could not drop %s: %v", name, err)
		}
	})
	return d
}

func TestIntegrationMigrateConcurrentAndIdempotent(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	ms, err := LoadMigrations()
	if err != nil {
		t.Fatal(err)
	}

	// Two servers starting at the same moment must not collide.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = d.Migrate(ctx)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent migrate %d: %v", i, err)
		}
	}

	res, err := d.Migrate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Applied) != 0 || res.Current != len(ms) {
		t.Errorf("third run should change nothing: %+v", res)
	}
	var recorded int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != len(ms) {
		t.Errorf("schema_migrations has %d rows, want %d", recorded, len(ms))
	}
}

func TestIntegrationEditionList(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	types, err := d.EditionTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 38 {
		t.Errorf("got %d edition types, want the 38 approved built-ins", len(types))
	}

	defaults := map[string][]string{}
	var tv []string
	for _, e := range types {
		if !e.Builtin || e.Hidden || len(e.ShortLabel) == 0 {
			t.Errorf("unexpected built-in entry: %+v", e)
		}
		for _, m := range e.MediaTypes {
			if e.IsDefault {
				defaults[m] = append(defaults[m], e.ID)
			}
			if m == "tv" {
				tv = append(tv, e.ID)
			}
		}
	}
	want := map[string]string{"movie": "theatrical", "tv": "broadcast", "music": "original", "audiobook": "unabridged", "book": "original"}
	for media, id := range want {
		if len(defaults[media]) != 1 || defaults[media][0] != id {
			t.Errorf("default for %s = %v, want [%s]", media, defaults[media], id)
		}
	}
	if len(tv) == 0 || tv[0] != "broadcast" {
		t.Errorf("TV editions should start with broadcast: %v", tv)
	}
}

func TestIntegrationAliensExample(t *testing.T) {
	d := testDatabase(t)
	ctx := context.Background()
	if _, err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	var libID, titleID, theatrical, special int64
	q := func(dest *int64, sql string, args ...any) {
		t.Helper()
		if err := d.Pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	q(&libID, `INSERT INTO libraries (name, media_type, paths) VALUES ('Movies', 'movie', '{/media/movies}') RETURNING id`)
	q(&titleID, `INSERT INTO titles (library_id, media_type, name, sort_name, year, tmdb_id)
	             VALUES ($1, 'movie', 'Aliens', 'aliens', 1986, 679) RETURNING id`, libID)
	q(&theatrical, `INSERT INTO editions (title_id, edition_type_id, runtime_seconds) VALUES ($1, 'theatrical', 8220) RETURNING id`, titleID)
	q(&special, `INSERT INTO editions (title_id, edition_type_id, runtime_seconds) VALUES ($1, 'special_edition', 9240) RETURNING id`, titleID)

	for _, v := range []struct {
		edition int64
		path    string
		label   string
	}{
		{theatrical, "/media/movies/Aliens (1986)/Aliens 2160p.mkv", "4K · HDR10 · Atmos"},
		{theatrical, "/media/movies/Aliens (1986)/Aliens 1080p.mkv", "1080p · DTS-HD MA"},
		{special, "/media/movies/Aliens (1986)/Aliens {edition-Special Edition} 1080p.mkv", "1080p · DTS-HD MA"},
	} {
		if _, err := d.Pool.Exec(ctx, `INSERT INTO versions (edition_id, path, size_bytes, label) VALUES ($1, $2, 1, $3)`,
			v.edition, v.path, v.label); err != nil {
			t.Fatal(err)
		}
	}

	var editions, versions int64
	q(&editions, `SELECT count(*) FROM editions WHERE title_id = $1`, titleID)
	q(&versions, `SELECT count(*) FROM versions v JOIN editions e ON e.id = v.edition_id WHERE e.title_id = $1`, titleID)
	if editions != 2 || versions != 3 {
		t.Errorf("editions=%d versions=%d, want 2 and 3", editions, versions)
	}

	// The same edition type can't be added twice to one title.
	if _, err := d.Pool.Exec(ctx, `INSERT INTO editions (title_id, edition_type_id) VALUES ($1, 'theatrical')`, titleID); err == nil {
		t.Error("duplicate edition was accepted")
	}

	// Deleting the title removes its editions and versions.
	if _, err := d.Pool.Exec(ctx, `DELETE FROM titles WHERE id = $1`, titleID); err != nil {
		t.Fatal(err)
	}
	q(&versions, `SELECT count(*) FROM versions`)
	if versions != 0 {
		t.Errorf("%d versions left after deleting the title", versions)
	}
}
