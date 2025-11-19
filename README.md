# Metarr

Metarr is a Go-based command line tool for pairing video files with their JSON/NFO metadata, enriching the metadata, and optionally transcoding/renaming the underlying media. It automates the workflow of preparing large video libraries for media servers (e.g. Plex, Jellyfin, Emby) by combining metadata scraping, FFmpeg/FFprobe driven transforms, and a rich set of batch-friendly file operations.

## Highlights

- **Batch-oriented pairing** – match individual files or entire directories of videos/metadata via `--batch-pairs`, `--video-directory`, `--meta-directory`, or explicit file lists.
- **Metadata enrichment** – normalize, infer, or scrape titles/descriptions, copy/paste fields, insert date tags, and purge stale keys using `--meta-ops`.
- **Filename automation** – add prefixes/suffixes, enforce date tags, and repair names with `--filename-ops` or the built-in rename styles.
- **Smart transcoding** – drive FFmpeg with per-codec remap rules, GPU acceleration (`--transcode-gpu`, `--transcode-gpu-directory`), quality presets, filters, and thumbnail embedding/removal.
- **Deterministic filtering** – constrain work by file extension, prefix/suffix/contains rules, or skip videos entirely (`--skip-videos`) to do metadata-only edits.
- **Resource aware** – honor concurrency, CPU, and minimum free RAM thresholds before spinning up workers.
- **Observability** – structured logging to `~/.metarr/metarr.log`, optional benchmarking artifacts, and an in-process HTTP endpoint (`127.0.0.1:6387/logs`) for live log tails.

## Requirements

- FFmpeg and FFprobe available on your `PATH`
- (Optional) GPU drivers/device nodes when using hardware acceleration
- (Optional) Browser cookie stores or a cookie file for authenticated metadata scraping (`--cookie-dir`)

## Install

```bash
# Clone or unpack the repository, then:
git clone https://github.com/TubarrApp/Metarr.git
cd Metarr
sudo mv metarr /usr/local/bin/metarr
metarr --help
```

The binary keeps its runtime artifacts under `~/.metarr/`:

- `metarr.log` – rolling program log
- `benchmark/` – created when `--benchmark` is enabled

## Quick Start

```bash
metarr \
  --video-directory /srv/media/videos \
  --meta-directory  /srv/media/meta \
  --output-ext mp4 \
  --transcode-video-codecs h265 \
  --transcode-audio-codecs aac \
  --filename-ops "date-tag:prefix:ymd" \
  --concurrency 4
```

For explicit source pairs (video and metadata file pair):

```bash
metarr --batch-pairs "/videos/show:/metadata/show"
```

Pass `--skip-videos` to only touch metadata/filenames.

## Configuration Sources

1. **Flags** – every option is exposed as a Cobra/Viper flag.
2. **Environment variables** – automatically derived by uppercasing and replacing `_` with `-` (e.g. `VIDEO_DIRECTORY=/srv/media`).
3. **Config file** – point to JSON/YAML/TOML/etc. via `--config-file path/to/config.yaml`.

Example YAML config:

```yaml
video-directory:
  - /srv/media/videos
meta-directory:
  - /srv/media/meta
output-ext: mp4
rename-style: underscores
transcode-gpu: vaapi
transcode-gpu-directory: /dev/dri/renderD128
transcode-video-codecs:
  - av1:h265
transcode-audio-codecs:
  - flac:aac
meta-ops:
  - title:prefix:[Archive] 
  - tags:append:new-release
filename-ops:
  - prefix:[ARCHIVE] 
  - date-tag:suffix:ymd
purge-metafile: json
debug: 2
```

## Inputs, Filtering, and Safety Nets

- `--video-directory`, `--video-file`, `--meta-directory`, `--meta-file` – sources that can be mixed and matched. Directories are processed recursively.
- `--batch-pairs "/videos:/meta"` – pin a video path to a metadata path (file or directory).
- `--input-video-exts` / `--input-meta-exts` – limit processing to certain extensions (`all`, `mkv`, `mp4`, `json`, `nfo`, etc.).
- `--filter-prefix`, `--filter-suffix`, `--filter-contains`, `--filter-omits` – lightweight string filters applied before work begins.
- `--no-file-overwrite` – keep originals around by renaming them before writing outputs.
- `--output-directory` – place finished video/metadata pairs somewhere else.
- `--purge-metafile` – delete matching metadata files after successful processing (e.g. `json`, `nfo`, `all`).

## Metadata Operations (`--meta-ops`)

Each entry follows `field:operation:value[:value]`. Values are colon-escaped internally, so literal `:` can be written as `\:`.

| Operation        | Example                                       | Effect |
| ---------------- | --------------------------------------------- | ------ |
| `set`            | `title:set:New Name`                          | Hard overwrite a field (including bulk credits via `all-credits`). |
| `append`         | `tags:append:new-tag`                         | Append to existing list/string values. |
| `prefix`         | `description:prefix:Draft - `                 | Prepend text if the field exists. |
| `replace`        | `summary:replace:foo:bar`                     | Replace substrings inside a field. |
| `replace-prefix` | `title:replace-prefix:[OLD] :[NEW] `          | Swap a matching prefix. |
| `replace-suffix` | `title:replace-suffix: DVD:: UHD`             | Swap a matching suffix. |
| `copy-to`        | `actors:copy-to:tags`                         | Copy a field’s contents into another field. |
| `paste-from`     | `title:paste-from:original-title`             | Reverse direction of `copy-to`. |
| `date-tag`       | `title:date-tag:prefix:ymd`                   | Insert a date tag using one of the supported `ymd`/`Ymd` styles. |
| `delete-date-tag`| `title:delete-date-tag:prefix:ymd`            | Strip generated date tags. |

Metarr will also attempt to infer missing descriptions from sibling fields and, if allowed, scrape metadata from the source website using browser cookies (`--cookie-dir` or auto-discovered Chrome/Firefox/Safari stores).

## Filename Operations (`--filename-ops`)

Syntax mirrors metadata operations but targets the physical filename (without extension):

| Operation        | Example                           | Effect |
| ---------------- | --------------------------------- | ------ |
| `prefix`         | `prefix:[MOVIE] `                  | Add a static prefix. |
| `append`         | `append: (Remaster)`               | Add a suffix. |
| `set`            | `set:Exact File Name`             | Replace the entire basename (one per run). |
| `replace`        | `replace:_ : `                     | Search/replace substrings. |
| `replace-prefix` | `replace-prefix:[OLD] :[NEW] `     | Swap prefixes. |
| `replace-suffix` | `replace-suffix: _v1:_final`       | Swap suffixes. |
| `date-tag`       | `date-tag:prefix:ymd`             | Attach a formatted date. |
| `delete-date-tag`| `delete-date-tag:all:ymd`         | Remove matching tags. |

Additionally, `--rename-style` quickly enforces common conventions: `spaces`, `underscores`, `fixes-only`, or `skip`.

## Video and Audio Pipeline

- `--output-ext` – change the container/extension (`mp4`, `mkv`, `webm`, etc.). Metarr protects you from illegal codec/container combos.
- `--transcode-video-codecs` / `--transcode-audio-codecs` – remap codecs via `input:output` pairs (e.g. `av1:h265`, `flac:aac`). A single value applies to every input.
- `--transcode-gpu` – pick `auto`, `cuda`, `vaapi`, `qsv`, or `amf`. Supply device paths via `--transcode-gpu-directory` when required.
- `--transcode-quality` – use FFmpeg preset-like quality buckets (`p1`..`p7`, respecting selected accelerator).
- `--transcode-video-filter` – inject arbitrary `-vf` expressions.
- `--extra-ffmpeg-args` – append custom switches to the generated command.
- `--force-write-thumbnail` – always regenerate thumbnails even if metadata matches.
- `--strip-thumbnail` – remove embedded artwork.
- `--skip-videos` – stop after metadata and filename updates.

Under the hood Metarr introspects the current codecs via FFprobe, caches available FFmpeg codecs, and only transcodes when needed. Thumbnail support handles both downloading remote artwork and copying embedded cover art when the container allows it.

## Resource & Execution Controls

- `--concurrency` – size of the worker pool (defaults to 5).
- `--max-cpu` – percentage cap that throttles work creation.
- `--min-free-mem` – minimum free RAM required to start/continue batches (supports suffixes like `4GB`).
- `--debug` – log verbosity (0–5).
- `--benchmark` – writes per-stage benchmark CSV files into `~/.metarr/benchmark`.

## Logging, Metrics, and Troubleshooting

- Timestamps go to both stderr and `~/.metarr/metarr.log`.
- `curl 127.0.0.1:6387/logs` while Metarr is running to view the in-memory log ring buffer.
- Errors from multiple goroutines are collected and summarized once processing stops.
- Combine `--debug 5` with `--benchmark` to capture the sequences that led to a problematic file.

## Development Notes

- Run the test suite with `go test ./...`.
- Run `golangci-lint` or your preferred tooling before submitting patches.
- Avoid committing generated binaries (the repository already includes `metarr/metarr` for convenience; delete or overwrite as needed).

---

Metarr is built to be composable: start with simple metadata normalization, then gradually add filename rules, GPU transcoding, cookie-backed scraping, and benchmarking as your workflow matures.
