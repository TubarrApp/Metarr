package domain

import (
	"os/exec"
)

func GrabLatestCommand(videoDir, targetURL string) *exec.Cmd {
	return exec.Command(
		"yt-dlp",
		"--write-info-json",
		"-P", videoDir,
		"--external-downloader", "aria2c",
		"--external-downloader-args", "-x 16 -s 16",
		"--restrict-filenames",
		"-o", "%(title)s.%(ext)s",
		"--retries", "999",
		"--retry-sleep", "10",
		"--cookies-from-browser", "firefox",
		targetURL,
	)
}
