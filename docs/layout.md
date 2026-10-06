# Layout

The screen layout is set in `~/.config/tidalt/app.toml`
(`$XDG_CONFIG_HOME/tidalt/app.toml`). Without the file you get tidalt's own
layout: the sidebar on the left, the playback bar at the bottom, and plain
track rows.

For a [spotify-player](https://github.com/aome510/spotify-player)-style layout
(the playback bar on top, no sidebar, and track lists as tables), add one line:

```toml
[layout]
preset = "spotify-player"
```

This also switches the keys to the `spotify-player` preset, unless
`keymap.toml` names a preset of its own (see [Key bindings](keymap.md)). So
the layout and the keys can still be set independently.

## Settings

Individual settings override the preset:

```toml
[layout]
preset = "spotify-player"          # or "tidalt" (the default)
playback_window_position = "Top"   # "Top" or "Bottom"
sidebar = false                    # show the section sidebar
track_table = true                 # # · Title · Artist · Album · Time columns
start_page = "Library"             # "Queue" or "Library": the page tidalt opens on
```

| Setting | `tidalt` | `spotify-player` |
| --- | --- | --- |
| `playback_window_position` | `Bottom` | `Top` |
| `sidebar` | `true` | `false` |
| `track_table` | `false` | `true` |
| `start_page` | `Queue` | `Library` |

Without the sidebar, use the page keys to move between sections (`z` for the
queue, `g l` for the [library](#library-page), `g y`, `g r`, `g m`, `g s`,
`u p`, `u a`, `u A`) and `Backspace` to go back. You can also open the command
palette (`:`) and pick "Go to …".

The track table applies to the Queue, Favorite songs, Recently played, the
open playlist, and an album opened from the artist view. Its column header stays
at the top while the list scrolls.

## Library page

`g l` (`LibraryPage`) opens the Library: your playlists, favorite artists and
favorite albums side by side on one page, as in spotify-player.

```
╭─ PLAYLISTS ───────────────╮╭─ ARTISTS ──────╮╭─ ALBUMS ──────────────────╮
│ › Road trip               ││   ◎ Bonobo     ││   ⊞ Migration (2017)      │
│   Focus · 42 tracks       ││   ◎ Floating … ││   ⊞ Promises (2021)       │
╰───────────────────────────╯╰────────────────╯╰───────────────────────────╯
```

- **Columns:** Playlists, Artists and Albums take 40%, 20% and 40% of the width
  (the Albums column absorbs the rounding). Each column scrolls on its own and
  keeps its own cursor. Playlist rows show the title and track count; artist
  and album rows look like the Artists and Albums pages.
- **Focus:** one column at a time has focus (accent border, selection band).
  The page opens on the column focused when you last left it — Playlists the
  first time. `Tab` / `l` / `→` move focus right and `Shift+Tab` / `h` / `←`
  left, wrapping around. `j`/`k`, the arrows, `g g`/`G`, `PgUp`/`PgDn` and `/`
  find (`n`/`N`) work in the focused column.
- **Enter** on a playlist opens it on the Playlists page with its tracks
  focused; on an artist, the artist view; on an album, its tracks in the queue,
  as on the Albums page. `Backspace` comes back to the Library.
- **Loading:** opening the page loads whichever of the three lists has not been
  loaded yet; an empty list says so in its column ("No playlists.", "No
  favorite artists.", "No favorite albums.").
- **Narrow terminals:** when the main pane (the screen minus the sidebar) is
  narrower than 60 columns, the page shows only the focused
  column, full width, with `Tab` still cycling.
- Track actions (`g a`, `Z`, …) have no selected track here and act on the
  playing track, if any.

With `start_page = "Library"` (the `spotify-player` preset's default) tidalt
opens on this page, as spotify-player does. The queue is still restored or
filled from your favorite songs in the background, without switching pages;
`z` shows it. The page history starts empty, so `Backspace` does nothing until
you move to another page.

The page works in either layout. It has no sidebar entry; in the `tidalt`
layout the sidebar items stay the way to reach each list. The command palette
lists it as "Go to Library".

## Following the playing track

The Queue cursor moves back onto the playing track after the keyboard has been
idle for a while (see [The interface](ui.md#following-the-playing-track)):

```toml
[queue]
follow_idle_sec = 10   # seconds without a key press; 0 turns it off
```

The default is 10 seconds; a negative value is an error.

If `app.toml` has an error, tidalt shows it in the status line and uses the
default layout.
