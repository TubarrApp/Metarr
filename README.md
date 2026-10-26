# Metarr

Metarr is a Go-based command line tool for pairing video files with their JSON/NFO metadata, enriching the metadata, and optionally transcoding/renaming the underlying media. It automates the workflow of preparing large video libraries for media servers (e.g. Plex, Jellyfin, Emby) by combining metadata normalization, FFmpeg/FFprobe driven transforms, and a rich set of batch-friendly file operations.

## Highlights

- **Batch-oriented pairing** – match individual files or entire directories of videos/metadata via `--batch-pairs`, `--video-directory`, `--meta-directory`, or explicit file lists.
- **Metadata enrichment** – normalize or infer titles/descriptions, copy/paste fields, insert date tags, and rewrite values using `--meta-ops`.
- **Filename automation** – add prefixes/suffixes, enforce date tags, and repair names with `--filename-ops` or the built-in rename styles.
- **Smart transcoding** – drive FFmpeg with per-codec remap rules, GPU acceleration (`--transcode-gpu`, `--transcode-gpu-node`), CRF quality, filters, and thumbnail embedding/removal.
- **Deterministic filtering** – constrain work by file extension, prefix/suffix/contains rules, or skip videos entirely (`--skip-videos`) to do metadata-only edits.
- **Resource aware** – honor concurrency, CPU, and minimum free RAM thresholds before spinning up workers.
- **Observability** – structured logging to `~/.metarr/metarr.log`, optional profiling output, and log forwarding to a running Tubarr instance.

## Requirements

- FFmpeg and FFprobe available on your `PATH`
- (Optional) GPU drivers/device nodes when using hardware acceleration

## Install

```bash
git clone https://github.com/TubarrApp/Metarr.git
cd Metarr
go build -o metarr ./cmd/metarr
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
2. **Config file** – point to JSON/YAML/TOML/etc. via `--config-file path/to/config.yaml`. Keys match the flag names.

Example YAML config:

```yaml
video-directory:
  - /srv/media/videos
meta-directory:
  - /srv/media/meta
output-ext: mp4
rename-style: underscores
transcode-gpu: vaapi
transcode-gpu-node: /dev/dri/renderD128
transcode-video-codecs:
  - av1:h265
transcode-audio-codecs:
  - flac:aac
meta-ops:
  - "title:prefix:[Archive] "
  - "description:suffix: (archived)"
filename-ops:
  - "prefix:[ARCHIVE] "
  - date-tag:suffix:ymd
purge-metafile: json
debug: 2
```

Quote values with leading or trailing spaces, since YAML trims them otherwise.

## Inputs, Filtering, and Safety Nets

- `--video-directory`, `--video-file`, `--meta-directory`, `--meta-file` – sources that can be mixed and matched. Only the top level of each directory is read (subdirectories are not searched).
- `--batch-pairs "/videos:/meta"` – pin a video path to a metadata path (file or directory).
- `--input-video-exts` / `--input-meta-exts` – limit processing to certain extensions (`all`, `mkv`, `mp4`, `webm` / `all`, `json`, `nfo`).
- `--filter-prefix`, `--filter-suffix`, `--filter-contains`, `--filter-omits` – lightweight filename filters applied before work begins.
- `--no-file-overwrite` – keep originals around by renaming them before writing outputs.
- `--purge-original-video` – delete the original video once a new one is written from it. Without it, the original is kept where it is, under its own name; only the new video goes to the output directory. If the new video ends up with exactly the original's path (same folder, name and extension), the original is renamed `name_mbackup.ext` instead.
- `--output-directory` – place finished video/metadata pairs somewhere else.
- `--purge-metafile` – delete matching metadata files after successful processing (`json`, `nfo`, or `all`).
- `--write-nfo` – write a Kodi/Jellyfin-style `.nfo` sidecar next to the final video.
- `--meta-overwrite` / `--meta-preserve` – when adding metadata fields, overwrite or keep fields that already exist.

## Metadata Operations (`--meta-ops`)

Each entry follows `field:operation:value[:value]`. The last part is everything after the previous `:`, so `title:set:HELLO: MY TEXT` needs no escaping. A `:` in a middle part (e.g. the find text of `replace`) is written `\:`. A `regex-replace` pattern is wrapped in `[[ ]]` and passed to the regex engine untouched. Repeat the flag once per operation.

| Operation        | Example                                       | Effect |
| ---------------- | --------------------------------------------- | ------ |
| `set`            | `title:set:New Name`                          | Hard overwrite a field (including bulk credits via `all-credits`). |
| `prefix`         | `description:prefix:Draft - `                 | Add text to the start of the field if it exists. |
| `suffix`         | `title:suffix: (Remaster)`                    | Add text to the end of the field if it exists. |
| `replace`        | `summary:replace:foo:bar`                     | Replace substrings inside a field. |
| `replace-prefix` | `title:replace-prefix:[OLD] :[NEW] `          | Swap a matching prefix. |
| `replace-suffix` | `title:replace-suffix: DVD:: UHD`             | Swap a matching suffix. |
| `regex-replace`  | `title:regex-replace:[[^Show \d+ \| ]]:`      | Replace every regex match ([RE2 syntax](https://github.com/google/re2/wiki/Syntax)); `$1` inserts a capture group, `(?i)` ignores case, an empty replacement deletes. |
| `keep-after`     | `title:keep-after:\| `                       | Keep only the text after the first `\| ` (the `\| ` is dropped too). |
| `keep-before`    | `title:keep-before: \|`                      | Keep only the text before the first ` \|`. |
| `copy-to`        | `actors:copy-to:tags`                         | Copy a field’s contents into another field. |
| `paste-from`     | `title:paste-from:original-title`             | Reverse direction of `copy-to`. |
| `date-tag`       | `title:date-tag:prefix:ymd`                   | Insert a date tag at the `prefix` or `suffix`. |
| `delete-date-tag`| `title:delete-date-tag:prefix:ymd`            | Strip date tags from the `prefix`, `suffix`, or `all`. |

Date formats are `Ymd`, `ymd`, `Ydm`, `ydm`, `dmY`, `dmy`, `mdY`, `mdy`, `md`, and `dm` (capital `Y` is a four-digit year, lowercase `y` two digits).

Metarr will also attempt to infer missing descriptions from sibling fields. To fill fields missing from a site's metadata, download through Tubarr, whose scraper writes them into the metadata file.

## Filename Operations (`--filename-ops`)

Syntax mirrors metadata operations but targets the physical filename (without extension). `copy-to` and `paste-from` are metadata-only.

| Operation        | Example                           | Effect |
| ---------------- | --------------------------------- | ------ |
| `prefix`         | `prefix:[MOVIE] `                  | Add a static prefix. |
| `suffix`         | `suffix: (Remaster)`               | Add a static suffix. |
| `set`            | `set:Exact File Name`             | Replace the entire basename (one per run). |
| `replace`        | `replace:_ : `                     | Search/replace substrings. |
| `replace-prefix` | `replace-prefix:[OLD] :[NEW] `     | Swap prefixes. |
| `replace-suffix` | `replace-suffix: _v1:_final`       | Swap suffixes. |
| `regex-replace`  | `regex-replace:[[^(.*) - (\d{4})$]]:$2 $1` | Replace every regex match. |
| `keep-after`     | `keep-after:\| `                  | Keep only the text after the first `\| `. |
| `keep-before`    | `keep-before: \|`                 | Keep only the text before the first ` \|`. |
| `date-tag`       | `date-tag:prefix:ymd`             | Attach a formatted date. |
| `delete-date-tag`| `delete-date-tag:all:ymd`         | Remove matching tags. |

## Filtered Operations (`--filtered-meta-ops`, `--filtered-filename-ops`)

Each entry is one or more filters and one operation divided by `||`, applied only when every filter matches the file's metadata: `title:contains:Ep 5 | Show: Finale||title:keep-after:| `. Join several filters with `&&` to require all of them: `title:contains:cat&&duration:more-than:3600||title:prefix:[LONG CAT VIDEO] `. Neither side needs escaping, except a `||` or `&&` in a filter's value. For several operations under the same filters, repeat the filters on each entry.

Additionally, `--rename-style` quickly enforces common conventions: `spaces`, `underscores`, `fixes-only`, or `skip` (the default).

## Video and Audio Pipeline

- `--output-ext` – change the container/extension (`mp4`, `mkv`, `webm`, etc.). If a requested codec isn't allowed in the chosen container, Metarr falls back to copying the stream instead.
- `--transcode-video-codecs` / `--transcode-audio-codecs` – remap codecs via `input:output` pairs (e.g. `av1:h265`, `flac:aac`). A single value applies to every input.
- `--transcode-gpu` – pick `auto`, `cuda`, `vaapi`, `qsv`, `amf`, or `videotoolbox`. Supply device paths via `--transcode-gpu-node` when required.
- `--transcode-quality` – CRF value from `0` (best) to `51` (worst); out-of-range values are clamped.
- `--transcode-video-filter` – inject arbitrary `-vf` expressions.
- `--extra-ffmpeg-args` – append custom switches to the generated command.
- `--force-write-thumbnail` – run FFmpeg to write the thumbnail even when all metadata already matches.
- `--strip-thumbnail` – remove embedded artwork.
- `--thumbnail` – embed this image (a file path or http(s) URL) instead of the metadata's thumbnail, always writing it. A local file is used where it is. Can't be combined with `--strip-thumbnail`.
- `--skip-videos` – stop after metadata and filename updates.

Under the hood Metarr introspects the current codecs via FFprobe, caches available FFmpeg codecs, and only transcodes when needed. Thumbnail support handles both downloading remote artwork and copying embedded cover art when the container allows it.

## Resource & Execution Controls

- `--concurrency` – size of the worker pool (defaults to 5).
- `--max-cpu` – percentage cap that throttles work creation (default `101`, i.e. no cap).
- `--min-free-mem` – minimum free RAM required to start/continue batches (supports suffixes like `4GB`).
- `--debug` – log verbosity (0–5).
- `--benchmark` – writes Go profiling output (`cpu.prof`, `mem.prof`, `trace.out`) into `~/.metarr/benchmark`.

## Logging, Metrics, and Troubleshooting

- Timestamps go to both stderr and `~/.metarr/metarr.log`.
- When Tubarr is running on the same machine, Metarr also sends its logs to it (`http://127.0.0.1:8827/metarr-logs`), so they show on Tubarr's Logs page.
- Errors from multiple goroutines are collected and summarized once processing stops.
- Combine `--debug 5` with `--benchmark` to capture the sequences that led to a problematic file.

## Development Notes

- Run the test suite with `go test ./...`.
- Run `golangci-lint` or your preferred tooling before submitting patches.
- Avoid committing generated binaries (the repository already includes a prebuilt `metarr` binary at its root for convenience; rebuild it with `go build -o metarr ./cmd/metarr`).

---

Metarr is built to be composable: start with simple metadata normalization, then gradually add filename rules, GPU transcoding, and benchmarking as your workflow matures.
