# Key bindings

tidalt comes with two keymap presets, and you can change any binding in
`~/.config/tidalt/keymap.toml` (`$XDG_CONFIG_HOME/tidalt/keymap.toml`). Press
`?` in the app to see the bindings in effect.

- **`tidalt`** (the default): the keys tidalt has always used.
- **`spotify-player`**: the defaults of
  [spotify-player](https://github.com/aome510/spotify-player), with
  tidalt-only commands put on keys spotify-player leaves free.

Both presets have the page keys (`z`, `g y`, `u p`, …) and `Backspace` for the
previous page, so every section can be reached without the sidebar. The
screen layout is set separately, in [app.toml](layout.md).

Cursor and focus keys are the same in both presets and can't be rebound yet:
`j`/`k` (or `↓`/`↑`) move, `h`/`l` (or `←`/`→`) switch between the sidebar and
the main pane, `Enter` plays or opens, and `Esc` backs out. In the `tidalt`
preset `Space`, `←` and `→` still play/pause and seek wherever the selected
list doesn't use them itself.

## keymap.toml

```toml
# Pick a preset. When unset, the keys follow the layout preset in
# app.toml, and are "tidalt" without one.
preset = "spotify-player"

# Then override single bindings. The format matches spotify-player's
# keymap.toml, so its [[keymaps]] blocks can be copied across.
[[keymaps]]
command = "NextTrack"
key_sequence = "g n"

[[keymaps]]
command = "ResumePause"
key_sequence = "M-enter"

# Unbind a key.
[[keymaps]]
command = "None"
key_sequence = "q"
```

A binding replaces whatever its key sequence meant in the preset. The preset's
other keys for that command stay bound.

**Key syntax:** separate the keys of a sequence with spaces (`g y`). Modifiers
are `C-` (ctrl), `M-` (alt) and `S-` (shift). `ctrl+`, `alt+` and `shift+`
work too. Named keys are `space`, `enter`, `esc`, `backspace`, `tab`,
`backtab`, `delete`, `insert`, `home`, `end`, `page_up`, `page_down`, `up`,
`down`, `left` and `right`.

A sequence can't be both bound and the start of a longer one. For example,
binding `g` alone while `g y` exists is an error. If the file has an error,
tidalt shows it in the status line and falls back to the default keymap.

When you type the first key of a chord, the footer shows that key followed by
`-` and lists each key that completes it.

## Commands

| Command | Description | `tidalt` | `spotify-player` |
| --- | --- | --- | --- |
| `ResumePause` | Resume/pause | `Space` | `Space` |
| `NextTrack` | Next track | `>`, `.` | `n` |
| `PreviousTrack` | Previous track | `<`, `,` | `p` |
| `SeekForward` | Seek forward 10s | `→` | `>` |
| `SeekBackward` | Seek backward 10s | `←` | `<` |
| `SeekStart` | Seek to start | | `^` |
| `VolumeUp` | Volume up 5% | `0` | `+` |
| `VolumeDown` | Volume down 5% | `9` | `-` |
| `Mute` | Mute/unmute | | `_` |
| `Shuffle` | Cycle shuffle mode | `s` | `C-s` |
| `Search` | Find in the current list | `/` | `/` |
| `FindNext` | Next match | `n` | |
| `FindPrevious` | Previous match | `N` | |
| `SelectFirstOrScrollToTop` | Go to top | `g g`, `Home` | `g g`, `Home` |
| `SelectLastOrScrollToBottom` | Go to bottom | `G`, `End` | `G`, `End` |
| `PageSelectNextOrScrollDown` | Page down | `PgDn` | `PgDn`, `C-f` |
| `PageSelectPreviousOrScrollUp` | Page up | `PgUp` | `PgUp`, `C-b` |
| `Queue` | Queue page | `z` | `z` |
| `LikedTrackPage` | Favorite songs | `g y` | `g y` |
| `RecentlyPlayedTrackPage` | Recently played | `g r` | `g r` |
| `MixesPage` | Daily mixes | `g m` | `g m` |
| `SearchPage` | Search | `g s` | `g s` |
| `BrowseUserPlaylists` | Playlists | `u p` | `u p` |
| `BrowseUserFollowedArtists` | Favorite artists | `u a` | `u a` |
| `BrowseUserSavedAlbums` | Favorite albums | `u A` | `u A` |
| `SwitchTheme` | Theme picker | | `T` |
| `PreviousPage` | Back to the previous page, or out of the artist view | `Backspace` | `Backspace`, `C-q` |
| `ShowActionsOnSelectedItem` | Actions on the selected track | `o` | `g a`, `C-Space` |
| `ShowActionsOnCurrentTrack` | Actions on the playing track | | `a` |
| `AddSelectedItemToQueue` | Add the selected track to the queue | | `Z`, `C-z` |
| `GoToRadio` | Start radio from the selected track | `r` | |
| `ToggleLiked` | Favorite/unfavorite | `f` | |
| `GoToArtist` | Go to the artist | `a` | |
| `CopyLink` | Copy the track link | `c` | |
| `SaveQueueAsPlaylist` | Save the queue as a playlist | `S` | `N` |
| `OpenCommandHelp` | Show key bindings | `?` | `?`, `C-h` |
| `OpenCommandPalette` | Command palette | `:`, `C-p` | `:` |
| `SwitchDevice` | Output device picker | `d` | `D` |
| `CycleTheme` | Cycle the color theme | `t` | |
| `Quit` | Quit (`C-c` always quits) | `q` | `q` |

In the `spotify-player` preset, radio, favorite, go-to-artist and copy-link are
in the actions popup (`g a` / `a`), the same as in spotify-player.
