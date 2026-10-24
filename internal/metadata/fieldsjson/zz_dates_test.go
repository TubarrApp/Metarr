package fieldsjson

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/domain/logger"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"
)

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestYearPrecedence tests that the release year wins over the year, a worked-out year prefers the release date,
// and the creation time stays a real date.
func TestYearPrecedence(t *testing.T) {
	for _, c := range []struct{ name, json, want, wantCreation string }{
		{"release year wins over year", `{"year": "2020", "release_year": 2019}`, "2019", ""},
		{"year used without a release year", `{"year": "2020"}`, "2020", ""},
		{"release date wins over date", `{"date": "2020-03-04", "release_date": "2019-01-02"}`, "2019", ""},
		{"upload date used last", `{"upload_date": "20210506"}`, "2021", ""},
		{"creation time isn't moved to the release year", `{"release_year": 2019, "upload_date": "20210506"}`, "2019", "2021-05-06"},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Write the metafile and decode it as processing does.
			path := filepath.Join(t.TempDir(), "video.json")
			if err := os.WriteFile(path, []byte(c.json), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := os.OpenFile(path, os.O_RDWR, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			rw := metawriters.NewJSONFileRW(context.Background(), f)
			data, err := rw.DecodeJSON(f)
			if err != nil {
				t.Fatal(err)
			}

			fd := models.NewFileData()
			fd.MetaFilePath = path
			FillTimestamps(fd, data, rw)
			if fd.MDates.Year != c.want {
				t.Errorf("year = %q, want %q", fd.MDates.Year, c.want)
			}
			if !strings.HasPrefix(fd.MDates.CreationTime, c.wantCreation) {
				t.Errorf("creation time = %q, want it to start %q", fd.MDates.CreationTime, c.wantCreation)
			}
		})
	}
}
