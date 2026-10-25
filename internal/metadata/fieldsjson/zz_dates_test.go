package fieldsjson

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metarr/internal/domain/logger"
	"metarr/internal/ffmpeg"
	"metarr/internal/metadata/metawriters"
	"metarr/internal/models"

	"github.com/TubarrApp/gocommon/sharedtags"
)

// TestMain gives the global logger a console writer, since the zero value has none.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestVideoDateCombinations tests every combination of date fields, each from a different year so the one used shows.
func TestVideoDateCombinations(t *testing.T) {
	// The fields in order of precedence, each with its value and whether it's a full date.
	fields := []struct {
		key, value string
		full       bool
	}{
		{"release_date", `"2015-01-15"`, true},
		{"originally_available_at", `"2016-02-16"`, true},
		{"release_year", `2017`, false},
		{"date", `"2018-03-17"`, true},
		{"year", `"2019"`, false},
		{"upload_date", `"20200418"`, true},
	}

	var table strings.Builder
	for combo := 1; combo < 1<<len(fields); combo++ {
		// Build the metafile from the fields in this combination.
		var parts, names []string
		for i, f := range fields {
			if combo&(1<<i) != 0 {
				parts = append(parts, fmt.Sprintf("%q: %s", f.key, f.value))
				names = append(names, f.key)
			}
		}

		// The first field present gives the year, and the date tag is its date, or the year alone for a year field.
		var wantYear, wantDateTag string
		for i, f := range fields {
			if combo&(1<<i) == 0 {
				continue
			}
			wantYear = strings.Trim(f.value, `"`)[:4]
			wantDateTag = wantYear
			if f.full {
				wantDateTag = strings.Trim(f.value, `"`)
				if len(wantDateTag) == 8 {
					wantDateTag = wantDateTag[:4] + "-" + wantDateTag[4:6] + "-" + wantDateTag[6:]
				}
			}
			break
		}

		fd := fillDatesFrom(t, "{"+strings.Join(parts, ", ")+"}")
		gotDateTag := ffmpeg.MetadataTags(fd)[sharedtags.JDate]
		if fd.MDates.Year != wantYear || gotDateTag != wantDateTag {
			t.Errorf("%v: year %q, date tag %q; want %q, %q", names, fd.MDates.Year, gotDateTag, wantYear, wantDateTag)
		}
		fmt.Fprintf(&table, "%-82s year: %-5s date tag: %-11s creation time: %s\n",
			strings.Join(names, " + "), fd.MDates.Year, gotDateTag, fd.MDates.CreationTime)
	}
	t.Log("\n" + table.String())
}

// TestYearSameOnRerun tests that a worked-out originally available date isn't saved to pass for a release date later.
func TestYearSameOnRerun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(`{"release_year": 2019, "upload_date": "20210506"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Run twice on the same metafile, as reprocessing does.
	for run := 1; run <= 2; run++ {
		if year := fillDatesFromFile(t, path).MDates.Year; year != "2019" {
			t.Errorf("run %d: year = %q, want 2019", run, year)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "originally_available_at") {
		t.Errorf("metafile has a worked-out originally available date: %s", content)
	}
}

// fillDatesFrom fills a file's dates from the given metafile content, as processing does.
func fillDatesFrom(t *testing.T, content string) *models.FileData {
	t.Helper()
	path := filepath.Join(t.TempDir(), "video.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return fillDatesFromFile(t, path)
}

// fillDatesFromFile fills a file's dates from the metafile at path, as processing does.
func fillDatesFromFile(t *testing.T, path string) *models.FileData {
	t.Helper()
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
	return fd
}

// TestVideoDates tests the year and video date tag taken from a metafile's dates, and that the creation time stays
// a real date.
func TestVideoDates(t *testing.T) {
	for _, c := range []struct{ name, json, wantYear, wantDateTag, wantCreation string }{
		{"release date wins over release year", `{"release_date": "2021-05-06", "release_year": 2019}`, "2021", "2021-05-06", "2021-05-06"},
		{"release date wins over year", `{"release_date": "2021-05-06", "year": "2019"}`, "2021", "2021-05-06", "2021-05-06"},
		{"release date wins over date", `{"date": "2021-05-06", "release_date": "2019-05-06"}`, "2019", "2019-05-06", "2019-05-06"},
		{"release year wins over date", `{"date": "2021-05-06", "release_year": 2019}`, "2019", "2019", "2021-05-06"},
		{"release year wins over year", `{"year": "2020", "release_year": 2019}`, "2019", "2019", ""},
		{"date wins over year", `{"date": "2021-05-06", "year": "2019"}`, "2021", "2021-05-06", "2021-05-06"},
		{"year used alone", `{"year": "2020"}`, "2020", "2020", ""},
		{"upload date used last", `{"upload_date": "20210506"}`, "2021", "2021-05-06", "2021-05-06"},
		{"upload date kept for a release year", `{"release_year": 2019, "upload_date": "20210506"}`, "2019", "2019", "2021-05-06"},
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

			// Fill the dates and check what the video gets.
			fd := models.NewFileData()
			fd.MetaFilePath = path
			FillTimestamps(fd, data, rw)
			if fd.MDates.Year != c.wantYear {
				t.Errorf("year = %q, want %q", fd.MDates.Year, c.wantYear)
			}
			if got := ffmpeg.MetadataTags(fd)[sharedtags.JDate]; got != c.wantDateTag {
				t.Errorf("video date tag = %q, want %q", got, c.wantDateTag)
			}
			if !strings.HasPrefix(fd.MDates.CreationTime, c.wantCreation) {
				t.Errorf("creation time = %q, want it to start %q", fd.MDates.CreationTime, c.wantCreation)
			}
		})
	}
}
