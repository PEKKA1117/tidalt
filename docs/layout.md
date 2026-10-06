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
```

| Setting | `tidalt` | `spotify-player` |
| --- | --- | --- |
| `playback_window_position` | `Bottom` | `Top` |
| `sidebar` | `true` | `false` |
| `track_table` | `false` | `true` |

Without the sidebar, use the page keys to move between sections (`z` for the
queue, `g y`, `g r`, `g m`, `g s`, `u p`, `u a`, `u A`) and `Backspace` to go
back. You can also open the command palette (`:`) and pick "Go to …".

The track table applies to the Queue, Favorite songs, Recently played, the
open playlist, and an album opened from the artist view. Its column header stays
at the top while the list scrolls.

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
