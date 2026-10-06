package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/BurntSushi/toml"
)

// Action is a named command a key sequence can be bound to. The names double
// as the `command` values in keymap.toml and follow spotify-player's command
// names wherever tidalt has the same command, so its keymap files carry over.
type Action string

const (
	ActNone Action = "None" // unbinds a key sequence in keymap.toml

	// Playback.
	ActResumePause   Action = "ResumePause"
	ActNextTrack     Action = "NextTrack"
	ActPreviousTrack Action = "PreviousTrack"
	ActSeekForward   Action = "SeekForward"
	ActSeekBackward  Action = "SeekBackward"
	ActSeekStart     Action = "SeekStart"
	ActVolumeUp      Action = "VolumeUp"
	ActVolumeDown    Action = "VolumeDown"
	ActMute          Action = "Mute"
	ActShuffle       Action = "Shuffle"

	// List navigation.
	ActSearch             Action = "Search" // find in the current list
	ActFindNext           Action = "FindNext"
	ActFindPrevious       Action = "FindPrevious"
	ActSelectFirst        Action = "SelectFirstOrScrollToTop"
	ActSelectLast         Action = "SelectLastOrScrollToBottom"
	ActPageSelectNext     Action = "PageSelectNextOrScrollDown"
	ActPageSelectPrevious Action = "PageSelectPreviousOrScrollUp"

	// Pages.
	ActQueue                     Action = "Queue"
	ActLikedTrackPage            Action = "LikedTrackPage"
	ActRecentlyPlayedTrackPage   Action = "RecentlyPlayedTrackPage"
	ActSearchPage                Action = "SearchPage"
	ActMixesPage                 Action = "MixesPage"
	ActBrowseUserPlaylists       Action = "BrowseUserPlaylists"
	ActBrowseUserFollowedArtists Action = "BrowseUserFollowedArtists"
	ActBrowseUserSavedAlbums     Action = "BrowseUserSavedAlbums"
	ActLibraryPage               Action = "LibraryPage"
	ActSwitchTheme               Action = "SwitchTheme"
	ActPreviousPage              Action = "PreviousPage"

	// Track actions.
	ActShowActionsOnSelectedItem Action = "ShowActionsOnSelectedItem"
	ActShowActionsOnCurrentTrack Action = "ShowActionsOnCurrentTrack"
	ActAddSelectedItemToQueue    Action = "AddSelectedItemToQueue"
	ActGoToRadio                 Action = "GoToRadio"
	ActToggleLiked               Action = "ToggleLiked"
	ActGoToArtist                Action = "GoToArtist"
	ActCopyLink                  Action = "CopyLink"
	ActSaveQueueAsPlaylist       Action = "SaveQueueAsPlaylist"

	// Application.
	ActOpenCommandHelp    Action = "OpenCommandHelp"
	ActOpenCommandPalette Action = "OpenCommandPalette"
	ActSwitchDevice       Action = "SwitchDevice"
	ActCycleTheme         Action = "CycleTheme"
	ActQuit               Action = "Quit"
)

// actionGroup orders the help page and decides where an action is handled.
type actionGroup int

const (
	groupPlayback actionGroup = iota
	groupList
	groupPages
	groupTrack
	groupApp
)

var groupNames = map[actionGroup]string{
	groupPlayback: "PLAYBACK",
	groupList:     "LIST",
	groupPages:    "PAGES",
	groupTrack:    "TRACK",
	groupApp:      "APP",
}

type actionInfo struct {
	group actionGroup
	desc  string
}

// actions lists every bindable action in help-page order.
var actions = []struct {
	act   Action
	group actionGroup
	desc  string
}{
	{ActResumePause, groupPlayback, "Resume/pause"},
	{ActNextTrack, groupPlayback, "Next track"},
	{ActPreviousTrack, groupPlayback, "Previous track"},
	{ActSeekForward, groupPlayback, "Seek forward 10s"},
	{ActSeekBackward, groupPlayback, "Seek backward 10s"},
	{ActSeekStart, groupPlayback, "Seek to start"},
	{ActVolumeUp, groupPlayback, "Volume up"},
	{ActVolumeDown, groupPlayback, "Volume down"},
	{ActMute, groupPlayback, "Mute/unmute"},
	{ActShuffle, groupPlayback, "Cycle shuffle"},

	{ActSearch, groupList, "Find in list"},
	{ActFindNext, groupList, "Next match"},
	{ActFindPrevious, groupList, "Previous match"},
	{ActSelectFirst, groupList, "Go to top"},
	{ActSelectLast, groupList, "Go to bottom"},
	{ActPageSelectNext, groupList, "Page down"},
	{ActPageSelectPrevious, groupList, "Page up"},

	{ActQueue, groupPages, "Queue"},
	{ActLikedTrackPage, groupPages, "Favorite songs"},
	{ActRecentlyPlayedTrackPage, groupPages, "Recently played"},
	{ActMixesPage, groupPages, "Daily mixes"},
	{ActSearchPage, groupPages, "Search"},
	{ActBrowseUserPlaylists, groupPages, "Playlists"},
	{ActBrowseUserFollowedArtists, groupPages, "Favorite artists"},
	{ActBrowseUserSavedAlbums, groupPages, "Favorite albums"},
	{ActLibraryPage, groupPages, "Library"},
	{ActSwitchTheme, groupPages, "Theme picker"},
	{ActPreviousPage, groupPages, "Previous page"},

	{ActShowActionsOnSelectedItem, groupTrack, "Actions on selected track"},
	{ActShowActionsOnCurrentTrack, groupTrack, "Actions on playing track"},
	{ActAddSelectedItemToQueue, groupTrack, "Add to queue"},
	{ActGoToRadio, groupTrack, "Start radio"},
	{ActToggleLiked, groupTrack, "Favorite/unfavorite"},
	{ActGoToArtist, groupTrack, "Go to artist"},
	{ActCopyLink, groupTrack, "Copy link"},
	{ActSaveQueueAsPlaylist, groupTrack, "Save queue as playlist"},

	{ActOpenCommandHelp, groupApp, "Key bindings"},
	{ActOpenCommandPalette, groupApp, "Command palette"},
	{ActSwitchDevice, groupApp, "Switch device"},
	{ActCycleTheme, groupApp, "Cycle theme"},
	{ActQuit, groupApp, "Quit"},
}

var actionIndex = func() map[Action]actionInfo {
	out := make(map[Action]actionInfo, len(actions))
	for _, a := range actions {
		out[a.act] = actionInfo{group: a.group, desc: a.desc}
	}
	return out
}()

// Preset names accepted by the `preset` key in keymap.toml.
const (
	PresetTidalt        = "tidalt"
	PresetSpotifyPlayer = "spotify-player"
)

// binding ties one normalized key sequence (space-separated tea.KeyMsg
// strings, e.g. "g y" or "ctrl+s") to an action.
type binding struct {
	seq string
	act Action
}

var presets = map[string][]binding{
	// The bindings tidalt has always had, plus a few that collide with nothing.
	PresetTidalt: {
		{" ", ActResumePause},
		{">", ActNextTrack},
		{".", ActNextTrack},
		{"<", ActPreviousTrack},
		{",", ActPreviousTrack},
		{"right", ActSeekForward},
		{"left", ActSeekBackward},
		{"0", ActVolumeUp},
		{"9", ActVolumeDown},
		{"s", ActShuffle},

		{"/", ActSearch},
		{"n", ActFindNext},
		{"N", ActFindPrevious},
		{"g g", ActSelectFirst},
		{"home", ActSelectFirst},
		{"G", ActSelectLast},
		{"end", ActSelectLast},
		{"pgdown", ActPageSelectNext},
		{"pgup", ActPageSelectPrevious},

		// spotify-player's page keys: needed when the sidebar is hidden.
		{"z", ActQueue},
		{"g y", ActLikedTrackPage},
		{"g r", ActRecentlyPlayedTrackPage},
		{"g s", ActSearchPage},
		{"g m", ActMixesPage},
		{"u p", ActBrowseUserPlaylists},
		{"u a", ActBrowseUserFollowedArtists},
		{"u A", ActBrowseUserSavedAlbums},
		{"g l", ActLibraryPage},
		{"backspace", ActPreviousPage},

		{"o", ActShowActionsOnSelectedItem},
		{"r", ActGoToRadio},
		{"f", ActToggleLiked},
		{"a", ActGoToArtist},
		{"c", ActCopyLink},
		{"S", ActSaveQueueAsPlaylist},

		{"?", ActOpenCommandHelp},
		{":", ActOpenCommandPalette},
		{"ctrl+p", ActOpenCommandPalette},
		{"d", ActSwitchDevice},
		{"t", ActCycleTheme},
		{"q", ActQuit},
	},
	// spotify-player's defaults (github.com/aome510/spotify-player), plus
	// tidalt-only commands on keys spotify-player leaves free.
	PresetSpotifyPlayer: {
		{"n", ActNextTrack},
		{"p", ActPreviousTrack},
		{" ", ActResumePause},
		{"ctrl+s", ActShuffle},
		{"+", ActVolumeUp},
		{"-", ActVolumeDown},
		{"_", ActMute},
		{"^", ActSeekStart},
		{">", ActSeekForward},
		{"<", ActSeekBackward},

		{"/", ActSearch},
		{"g g", ActSelectFirst},
		{"home", ActSelectFirst},
		{"G", ActSelectLast},
		{"end", ActSelectLast},
		{"pgdown", ActPageSelectNext},
		{"ctrl+f", ActPageSelectNext},
		{"pgup", ActPageSelectPrevious},
		{"ctrl+b", ActPageSelectPrevious},

		{"z", ActQueue},
		{"g y", ActLikedTrackPage},
		{"g r", ActRecentlyPlayedTrackPage},
		{"g s", ActSearchPage},
		{"g m", ActMixesPage},
		{"u p", ActBrowseUserPlaylists},
		{"u a", ActBrowseUserFollowedArtists},
		{"u A", ActBrowseUserSavedAlbums},
		{"g l", ActLibraryPage},
		{"T", ActSwitchTheme},
		{"backspace", ActPreviousPage},
		{"ctrl+q", ActPreviousPage},

		{"g a", ActShowActionsOnSelectedItem},
		{"ctrl+@", ActShowActionsOnSelectedItem},
		{"a", ActShowActionsOnCurrentTrack},
		{"Z", ActAddSelectedItemToQueue},
		{"ctrl+z", ActAddSelectedItemToQueue},
		{"N", ActSaveQueueAsPlaylist},

		{"?", ActOpenCommandHelp},
		{"ctrl+h", ActOpenCommandHelp},
		{":", ActOpenCommandPalette},
		{"D", ActSwitchDevice},
		{"q", ActQuit},
	},
}

// Keymap resolves key sequences to actions.
type Keymap struct {
	bindings []binding // in definition order; drives Keys and the help page
	index    map[string]Action
	prefixes map[string]bool // every proper prefix of a multi-key sequence
}

func newKeymap(bs []binding) (*Keymap, error) {
	km := &Keymap{
		bindings: bs,
		index:    make(map[string]Action, len(bs)),
		prefixes: make(map[string]bool),
	}
	for _, b := range bs {
		km.index[b.seq] = b.act
		parts := strings.Split(b.seq, " ")
		for i := 1; i < len(parts); i++ {
			km.prefixes[strings.Join(parts[:i], " ")] = true
		}
	}
	for _, b := range bs {
		if km.prefixes[b.seq] {
			return nil, fmt.Errorf("key sequence %q is bound to %s but is also the start of a longer sequence", b.seq, b.act)
		}
	}
	return km, nil
}

// Preset returns the named built-in keymap.
func Preset(name string) (*Keymap, error) {
	bs, ok := presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown keymap preset %q (want %q or %q)", name, PresetTidalt, PresetSpotifyPlayer)
	}
	return newKeymap(slices.Clone(bs))
}

// defaultKeymap is used by models built without a keymap (and when
// keymap.toml cannot be read).
var defaultKeymap = func() *Keymap {
	km, err := Preset(PresetTidalt)
	if err != nil {
		panic(err)
	}
	return km
}()

// Lookup returns the action bound to a normalized key sequence.
func (km *Keymap) Lookup(seq string) (Action, bool) {
	a, ok := km.index[seq]
	return a, ok
}

// IsPrefix reports whether seq is the start of a longer bound sequence, i.e.
// more keys are needed before it means anything.
func (km *Keymap) IsPrefix(seq string) bool {
	return km.prefixes[seq]
}

// Keys returns every sequence bound to a, in definition order.
func (km *Keymap) Keys(a Action) []string {
	var out []string
	for _, b := range km.bindings {
		if b.act == a {
			out = append(out, b.seq)
		}
	}
	return out
}

// continuations returns the bindings that extend the pending prefix, with the
// prefix stripped, for the footer's chord hint.
func (km *Keymap) continuations(prefix string) []binding {
	var out []binding
	for _, b := range km.bindings {
		if rest, ok := strings.CutPrefix(b.seq, prefix+" "); ok {
			out = append(out, binding{seq: rest, act: b.act})
		}
	}
	return out
}

// keymapFile is the on-disk shape of keymap.toml. It mirrors spotify-player's
// keymap.toml so a [[keymaps]] block can be copied across.
type keymapFile struct {
	Preset  string `toml:"preset"`
	Keymaps []struct {
		Command     string `toml:"command"`
		KeySequence string `toml:"key_sequence"`
	} `toml:"keymaps"`
}

// ParseKeymap builds a keymap from keymap.toml contents: the chosen preset
// (tidalt when unset) with each [[keymaps]] entry layered on top. An entry
// replaces whatever its key sequence meant before; command = "None" unbinds it.
func ParseKeymap(data []byte) (*Keymap, error) {
	return parseKeymap(data, PresetTidalt)
}

// parseKeymap is ParseKeymap with the preset to use when the file names none.
func parseKeymap(data []byte, fallbackPreset string) (*Keymap, error) {
	var f keymapFile
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, err
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("unknown setting %q", und[0].String())
	}
	if f.Preset == "" {
		f.Preset = fallbackPreset
	}
	base, ok := presets[f.Preset]
	if !ok {
		return nil, fmt.Errorf("unknown keymap preset %q (want %q or %q)", f.Preset, PresetTidalt, PresetSpotifyPlayer)
	}
	bs := slices.Clone(base)
	for i, e := range f.Keymaps {
		act := Action(e.Command)
		if _, known := actionIndex[act]; !known && act != ActNone {
			return nil, fmt.Errorf("keymaps[%d]: unknown command %q", i, e.Command)
		}
		seq, err := normalizeKeySequence(e.KeySequence)
		if err != nil {
			return nil, fmt.Errorf("keymaps[%d]: %w", i, err)
		}
		bs = slices.DeleteFunc(bs, func(b binding) bool { return b.seq == seq })
		if act != ActNone {
			bs = append(bs, binding{seq: seq, act: act})
		}
	}
	return newKeymap(bs)
}

// KeymapPath is where tidalt looks for keymap.toml:
// $XDG_CONFIG_HOME/tidalt/keymap.toml, usually ~/.config/tidalt/keymap.toml.
func KeymapPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tidalt", "keymap.toml")
}

// LoadKeymap reads keymap.toml. fallbackPreset (app.toml's layout preset)
// applies when the file names no preset, and when it is missing. A broken
// file returns the fallback preset too, alongside the error, so a typo in
// the file never leaves the UI without keys.
func LoadKeymap(path, fallbackPreset string) (*Keymap, error) {
	fallback, err := Preset(fallbackPreset)
	if err != nil {
		fallback = defaultKeymap
	}
	if path == "" {
		return fallback, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is tidalt's own config file, chosen by the user
	if errors.Is(err, fs.ErrNotExist) {
		return fallback, nil
	}
	if err != nil {
		return fallback, err
	}
	km, err := parseKeymap(data, fallbackPreset)
	if err != nil {
		return fallback, fmt.Errorf("%s: %w", path, err)
	}
	return km, nil
}

// keyAliases maps the key names keymap.toml accepts (spotify-player's and a
// few common spellings) to bubbletea's tea.KeyMsg strings.
var keyAliases = map[string]string{
	"space":     " ",
	"enter":     "enter",
	"return":    "enter",
	"esc":       "esc",
	"escape":    "esc",
	"backspace": "backspace",
	"tab":       "tab",
	"backtab":   "shift+tab",
	"delete":    "delete",
	"insert":    "insert",
	"home":      "home",
	"end":       "end",
	"page_up":   "pgup",
	"pageup":    "pgup",
	"pgup":      "pgup",
	"page_down": "pgdown",
	"pagedown":  "pgdown",
	"pgdown":    "pgdown",
	"up":        "up",
	"down":      "down",
	"left":      "left",
	"right":     "right",
}

// normalizeKeySequence turns a keymap.toml key sequence ("C-s", "g a",
// "M-enter", "space") into the space-joined tea.KeyMsg strings the keymap is
// indexed by.
func normalizeKeySequence(s string) (string, error) {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "", errors.New("empty key sequence")
	}
	out := make([]string, len(parts))
	for i, p := range parts {
		k, err := normalizeKey(p)
		if err != nil {
			return "", err
		}
		out[i] = k
	}
	return strings.Join(out, " "), nil
}

// modifierPrefixes are the modifier spellings normalizeKey strips, in
// spotify-player's (C-, M-, S-) and bubbletea's (ctrl+, alt+, shift+) forms.
var modifierPrefixes = []struct {
	prefix, mod string
	foldCase    bool
}{
	{"C-", "ctrl", false},
	{"M-", "alt", false},
	{"S-", "shift", false},
	{"ctrl+", "ctrl", true},
	{"alt+", "alt", true},
	{"shift+", "shift", true},
}

// cutModifier splits one leading modifier off s.
func cutModifier(s string) (mod, rest string, ok bool) {
	for _, m := range modifierPrefixes {
		if len(s) < len(m.prefix) {
			continue
		}
		head := s[:len(m.prefix)]
		if head == m.prefix || (m.foldCase && strings.EqualFold(head, m.prefix)) {
			return m.mod, s[len(m.prefix):], true
		}
	}
	return "", s, false
}

func normalizeKey(s string) (string, error) {
	var ctrl, alt, shift bool
	rest := s
	for {
		mod, after, ok := cutModifier(rest)
		if !ok {
			break
		}
		if after == "" {
			return "", fmt.Errorf("invalid key %q: modifier without a key", s)
		}
		switch mod {
		case "ctrl":
			ctrl = true
		case "alt":
			alt = true
		default:
			shift = true
		}
		rest = after
	}
	key, ok := keyAliases[strings.ToLower(rest)]
	if !ok {
		if utf8.RuneCountInString(rest) != 1 {
			return "", fmt.Errorf("invalid key %q", s)
		}
		key = rest
	}
	if shift {
		switch {
		case utf8.RuneCountInString(key) == 1 && key != " ":
			key = strings.ToUpper(key)
		case key == "tab" || key == "up" || key == "down" || key == "left" || key == "right" || key == "home" || key == "end":
			key = "shift+" + key
		default:
			return "", fmt.Errorf("invalid key %q: shift is not supported with %q", s, rest)
		}
	}
	if ctrl {
		if key == " " {
			key = "@" // terminals send NUL for ctrl+space; bubbletea calls it ctrl+@
		}
		key = "ctrl+" + key
	}
	if alt {
		key = "alt+" + key
	}
	return key, nil
}

// keyLabels shortens tea.KeyMsg strings for the footer and help page.
var keyLabels = map[string]string{
	" ":         "Space",
	"enter":     "↵",
	"esc":       "Esc",
	"backspace": "⌫",
	"tab":       "Tab",
	"shift+tab": "S-Tab",
	"left":      "←",
	"right":     "→",
	"up":        "↑",
	"down":      "↓",
	"pgup":      "PgUp",
	"pgdown":    "PgDn",
	"home":      "Home",
	"end":       "End",
	"ctrl+@":    "C-Space",
}

// keyLabel renders a normalized key sequence in spotify-player's notation
// (C-s, M-↵, g a).
func keyLabel(seq string) string {
	parts := strings.Split(seq, " ")
	if seq == " " {
		parts = []string{" "}
	}
	for i, p := range parts {
		parts[i] = singleKeyLabel(p)
	}
	return strings.Join(parts, " ")
}

func singleKeyLabel(k string) string {
	if l, ok := keyLabels[k]; ok {
		return l
	}
	if rest, ok := strings.CutPrefix(k, "alt+"); ok && rest != "" {
		return "M-" + singleKeyLabel(rest)
	}
	if rest, ok := strings.CutPrefix(k, "ctrl+"); ok && rest != "" {
		return "C-" + singleKeyLabel(rest)
	}
	return k
}
