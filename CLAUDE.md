# CLAUDE.md

## Build & tooling

- **Go version**: 1.26+
- **Format**: run `gofumpt -w .` after writing or editing any `.go` file
- **Lint (Go)**: run `golangci-lint run` (v2) before finishing a task; fix all reported issues
- **Lint (C)**: run `./lint-c.sh` after editing any `.c`/`.h` file (containerised clang-tidy; `LINT_C_NATIVE=1` to use a local clang-tidy). Config in `.clang-tidy` — its `Checks:` block is a YAML folded scalar and must not contain `#` comments
- **Build**: `go build ./...`
- **CGO**: required — the player package links against libasound (`-lasound`)

## Development approach: tech-lead

The primary session works as a **tech-lead**, following the `tech-lead` skill vendored at `.claude/skills/tech-lead/SKILL.md` (from [akunzai/agent-skills](https://github.com/akunzai/agent-skills/blob/main/skills/tech-lead/SKILL.md) @ `50117a94d9d7`, MIT, see its `LICENSE`). Read it, and its `references/brief-elements.md`, before starting any non-trivial dev or fix:

- Stay in the primary session for a few-line or single-file mechanical edit and for architecture decisions; delegate anything larger to implementer subagents
- Slice the work (files each slice may touch, parallel/serial, durability), confirm parallelism and its cap with the user, isolate each slice in its own git worktree and branch, brief one implementer per slice, then accept, integrate and clean up in the primary session
- Tests, builds and lint run in the primary session, which reads their output itself

It combines with SDD + TDD below as follows: the spec (step 1 of SDD) is written or approved in the primary session before slicing, and is quoted in every brief; each brief's acceptance requires the red → green → refactor sequence, the failing-test evidence, and the tooling checks from "Build & tooling"; at acceptance the tech-lead verifies the test was written first and fails without the change, and that the spec and code agree.

## Development workflow: SDD + TDD

Every feature and every bug fix, however small, follows spec-driven development (SDD) and then test-driven development (TDD), in this order:

1. **Spec first (SDD)** — before touching code, write or update the spec that states the intended behaviour:
   - Features: add or update the relevant page in `docs/` (or create a new one) describing the behaviour, inputs/outputs, key bindings/config, and edge cases. Link new pages from `docs/architecture.md` or `README.md` where appropriate
   - Bug fixes: state the expected vs. actual behaviour and the root cause, and correct the spec in `docs/` if it was wrong or silent on the case
   - Keep the "Package overview" below in sync when a change alters what a package does
2. **Red (TDD)** — write a failing `_test.go` test derived from the spec (for a bug: a test that reproduces it). Run `go test ./<pkg>/...` and confirm it fails for the expected reason
3. **Green** — write the minimal code that makes the test pass
4. **Refactor** — clean up with the tests green, then run `gofumpt -w .`, `golangci-lint run`, `go build ./...` and `go test ./...`

Tests are table-driven where it fits and live next to the code they cover. Code that can't be unit-tested directly (ALSA/FFmpeg CGO, D-Bus, live Tidal API) is tested through a seam — extract the pure logic (format choice, fallback decisions, parsing, URL building) into a testable function, as `shared.go`/`alsa_fallback_test.go` and `api_test.go` do. Never skip, disable or weaken a test to get green; if a spec change makes a test obsolete, update the test to the new spec.

## Package overview

### `cmd/tidalt`
Entry point. Handles signal setup, session load/restore from the secrets store, OAuth2 device-flow login on first run, and launches the BubbleTea TUI program.

### `internal/tidal`
Tidal API client.
- `client.go` — OAuth2 device-flow authentication, token refresh, authenticated HTTP client
- `api.go` — REST calls: favorites, search, track lookup, stream URL (quality ladder: HI_RES_LOSSLESS → LOSSLESS → HIGH → LOW), mixes, mix tracks, artist albums/top-tracks/all-tracks
- Daily Mixes use the v1 API: `GET /v1/pages/my_collection_my_mixes` for the list, `GET /v1/mixes/{mixId}/items` for a mix's tracks (fully populated, one request). The v2 `openapi.tidal.com/v2/userRecommendations` resource was removed by Tidal and now 404s — do not reintroduce it. Video mixes and non-`track` items are filtered out, since the player cannot decode video

### `internal/player`
Bit-perfect FLAC playback via CGO + libasound.
- The hand-written C lives in real translation units, not cgo preamble comments: `alsa.h`/`alsa.c` (PCM open + format negotiation, used by `mpv.go`) and `avcodec.h`/`avcodec.c` (FFmpeg decode pipeline, used by `avcodec.go`). The Go files keep only a minimal preamble that `#include`s the header plus the `#cgo` linker directives. `avio_read_cb` is implemented in Go via `//export` and declared in `avcodec.h`
- Opens ALSA `hw:` devices directly, bypassing PipeWire/PulseAudio
- Negotiates the best PCM format the DAC supports using `snd_pcm_hw_params`. Resampling policy is stated explicitly per device via `snd_pcm_hw_params_set_rate_resample`: `0` on `hw:` (a no-op there, but it keeps the bit-perfect contract in the code instead of in an ALSA default), `1` on plug-layer devices
- Shared output mode: selecting `default` in the device picker plays through the system mixer instead of claiming the card. `IsSharedDevice`/`isPlugDevice`/`alsaTuning` in `shared.go` drive the three things that differ — the ReserveDevice1 handshake is skipped (`reserveUnlessShared`), the ring buffer widens from 4 to 8 periods, and `bitPerfect` is false so the UI badge reads `(shared)`. Shared mode is never reached by a fallback; it only happens when the user picks it
- Falls back to `plughw:` only when format negotiation itself is refused — i.e. `configure_hw_pcm` fails, tagged with the `errFormatRefused` sentinel — as on fixed-format USB interfaces such as the Focusrite Vocaster. A busy device fails at `open_hw_device` instead, which carries the existing `-EBUSY` retry against `hw:` and is never downgraded. The fallback is memoised per device, and `alsaHandle.bitPerfect` / `Player.AudioPath` propagate the downgrade up to the UI so the quality badge and device readout stop claiming untouched output
- Format preference for 16-bit sources: S32_LE → S16_LE → S24_3LE → S24_LE (S32_LE first because some DACs, e.g. CS43198-based Hidizs S9 Pro Plus, have a broken S16_LE USB endpoint)
- Format preference for 24-bit sources: S24_3LE → S24_LE → S32_LE
- Acquires `org.freedesktop.ReserveDevice1.Audio{N}` on D-Bus before opening the device, asking PipeWire to release if it holds the reservation
- Demuxes and decodes the HTTP stream in-flight via FFmpeg (libavformat/libavcodec/libswresample, CGO) — FLAC, AAC/mp4, and ALAC — resampling to S32LE. A custom AVIO callback feeds bytes straight from the HTTP response. FFmpeg is linked dynamically for local/dev/CI builds (needs the distro's libav*-dev headers); the official distro packages (`packaging/`) bundle a minimal static FFmpeg built from source, selected with the `staticav` build tag
- Volume, pause, and position tracking via atomics
- Auto-detects known DACs (Hidizs S9 Pro, Hidizs S9 Pro Plus "Martha", Focusrite Scarlett Solo) from `/proc/asound/cards`

### `internal/store`
Persistent storage.
- OAuth2 session stored securely via `docker/secrets-engine` (system keychain, falling back to age-encrypted file at `~/.config/tidalt/secrets`)
- Volume, selected device, and track metadata cache stored in a bbolt database at `~/.local/share/tidalt/tidal-cache.db`

### `internal/ui`
BubbleTea TUI model (Model/Update/View).
- Five states: `StateBrowse`, `StateMixes`, `StateSearch`, `StateDeviceSelect`, `StateArtistAlbums`
- Scrollable track and mix lists with a visible window helper
- Artist view (`StateArtistAlbums`): opened with `a` on any track; lists the artist's albums plus "Play all tracks" / "Top tracks" entries, loading the chosen tracks into the browse queue
- Progress bar with playback position, volume display, and device label
- Key bindings go through `keymap.go`: an `Action` per command (names follow spotify-player's), two presets (`tidalt`, the historical keys and the default; `spotify-player`), and optional overrides from `~/.config/tidalt/keymap.toml` (`preset = …`, falling back to app.toml's `[layout] preset`, plus spotify-player-style `[[keymaps]] command / key_sequence` blocks; `command = "None"` unbinds). Sequences are stored as space-joined `tea.KeyMsg` strings (`"g y"`, `"ctrl+s"`). `handleKey` collects chords in `pendingKeys` (the footer lists the continuations) and runs page/app actions, and playback actions on non-structural keys, globally; a completed chord is routed to the section as one key whose `String()` is the whole sequence, so `updateListMotion` and `commonKeys` resolve chords and single keys the same way. Cursor/focus keys (`j/k/h/l`, arrows, `Enter`, `Esc`) stay hard-coded in the section handlers. `?` opens the help overlay (`help.go`), built from the active keymap
- Layout comes from `~/.config/tidalt/app.toml` (`appconfig.go`): `[layout] preset` (`tidalt` | `spotify-player`) plus `playback_window_position` (`Top`/`Bottom`), `sidebar`, `track_table`. The zero `Layout` is tidalt's own arrangement, so models built without config (tests, daemon) keep it. With `PlaybackTop` the now-playing bar renders first and keeps its full height when idle; `bodyTop()` gives the offset that `coverBoxRect` and the action-sheet anchor add. With `HideSidebar`, `layoutDims` returns a zero sidebar and `routeKey` pins `focusMain`. `TrackTable` switches the track panes (all drawn by `renderTrackList`) to `renderTrackTableRow` under a pinned `renderTrackTableHeader`
- Library page (`library_page.go`, `SecLibrary`, `g l` / `LibraryPage`): playlists, favorite artists and favorite albums in 40/20/40 columns, each with its own cursor (`libCursor`) and one focused column (`libFocus`, `Tab`/`h`/`l` wrap), both kept across visits; below 60 columns only the focused column shows. No sidebar entry; the palette lists "Go to Library". Row builders shared with the single-list pages live in `library_sections.go`. Spec: `docs/layout.md` "Library page"
- Page history: `selectSection` and `jumpToQueue` push the section being left onto `pageHistory`; `PreviousPage` (`Backspace`) closes the artist drill-down first, then pops it via `showSection`, which switches without recording
- Vim-style list keys in every main-pane list (`listfind.go`): `/` opens an incremental, case-insensitive find over the active list (tracks match title or artist), `Enter` keeps the match, `Esc` restores the cursor; `n`/`N` repeat the last find with wrap-around; `gg`/`G` jump to top/bottom. `activeList` maps each section/sub-view to its labels and cursor; the open prompt is checked before global keys so `q`/`t`/`d`/`:` type literally
- Auto-advances to the next track in the queue when playback finishes
- Queue follow: on the 1s `tickMsg`, `followPlaying` (`queue.go`) moves the Queue cursor onto the playing track once no key has arrived for `followIdle` (app.toml `[queue] follow_idle_sec`, default 10, `0` = off; the zero `Model` leaves it off). `lastKeyAt` is stamped on every `tea.KeyMsg`. It holds off while an overlay, the find prompt, a pending chord or the artist drill-down is open. Spec: `docs/ui.md` "Following the playing track"
