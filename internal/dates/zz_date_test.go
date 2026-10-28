package dates_test

import (
	"io"
	"metarr/internal/dates"
	"metarr/internal/domain/logger"
	"metarr/internal/models"
	"os"
	"testing"

	"github.com/TubarrApp/gocommon/sharedenums"
)

// TestMain sets up the test environment for the app package.
func TestMain(m *testing.M) {
	logger.Pl.Console = io.Discard
	os.Exit(m.Run())
}

// TestMakeDateTag tests date tag generation.
func TestMakeDateTag(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		fd       *models.FileData
		dateFmt  sharedenums.DateFormat
		wantTag  string
		wantErr  bool
	}{
		{
			name:     "Skip format returns empty string",
			metadata: nil,
			fd:       &models.FileData{OriginalVideoPath: "test.mp4"},
			dateFmt:  sharedenums.DateFmtSkip,
			wantTag:  "",
			wantErr:  false,
		},
		{
			name:     "Uses FormattedDate from FileData when present",
			metadata: nil,
			fd: &models.FileData{
				OriginalVideoPath: "test.mp4",
				MDates: &models.MetadataDates{
					FormattedDate: "2026-06-05",
				},
			},
			dateFmt: sharedenums.DateYyMmDd,
			wantTag: "[26-06-05]",
			wantErr: false,
		},
		{
			name: "Extracts date from metadata when FormattedDate is empty",
			metadata: map[string]any{
				"date": "20260605", // 'date' metafield.
			},
			fd: &models.FileData{
				OriginalVideoPath: "test.mp4",
				MDates: &models.MetadataDates{
					FormattedDate: "",
				},
			},
			dateFmt: sharedenums.DateYyyyMmDd,
			wantTag: "[2026-06-05]",
			wantErr: false,
		},
		{
			name: "Guess 'release_date' field is 'yyyy *DD* mm' instead of 'yyyymmdd'",
			metadata: map[string]any{
				"release_date": "20262905", // Impossible month in 'yyyymmdd'.
			},
			fd: &models.FileData{
				OriginalVideoPath: "test.mp4",
				MDates: &models.MetadataDates{
					FormattedDate: "",
				},
			},
			dateFmt: sharedenums.DateYyyyMmDd,
			wantTag: "[2026-05-29]",
			wantErr: false,
		},
		{
			name:     "No dates found returns empty tag without error",
			metadata: map[string]any{},
			fd: &models.FileData{
				OriginalVideoPath: "test.mp4",
				MDates: &models.MetadataDates{
					FormattedDate: "",
				},
			},
			dateFmt: sharedenums.DateYyyyMmDd,
			wantTag: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTag, err := dates.MakeDateTag(tt.metadata, tt.fd, tt.dateFmt)

			if (err != nil) != tt.wantErr {
				t.Fatalf("MakeDateTag() error = %v, wantErr %v", err, tt.wantErr)
			}

			if gotTag != tt.wantTag {
				t.Errorf("MakeDateTag() = %q, want %q", gotTag, tt.wantTag)
			}
		})
	}
}
