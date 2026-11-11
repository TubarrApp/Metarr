package file

import (
	"context"
	"fmt"
	"io"
	"metarr/internal/domain/logger"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// thumbnailClient downloads thumbnails, giving up on a server that stops answering.
var thumbnailClient = &http.Client{Timeout: 30 * time.Second}

// DownloadThumbnail downloads a thumbnail from a URL to a temporary file.
//
// Returns the local file path to use with FFmpeg -attach.
func DownloadThumbnail(ctx context.Context, urlStr, videoBaseName string) (string, error) {
	if urlStr == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}
	resp, err := thumbnailClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("got nil resp in DownloadThumbnail()")
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Pl.E("Failed to close response body due to error: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download thumbnail: %s", resp.Status)
	}

	// Remove query parameters.
	base, _, _ := strings.Cut(urlStr, "?")
	base, _, _ = strings.Cut(base, "#")

	// Remove illegal characters
	cleanBase := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, filepath.Base(base))

	// Limit length to stay below filesystem limits.
	if len(cleanBase) > 50 {
		cleanBase = cleanBase[:50]
	}

	tmpPath := filepath.Join(os.TempDir(), "thumb_"+videoBaseName+"_"+cleanBase)

	file, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Pl.E("Failed to close file %q due to error: %v", file.Name(), closeErr)
		}
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}

	return tmpPath, nil
}
