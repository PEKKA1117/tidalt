package ui

import (
	"testing"

	"github.com/Benehiko/tidalt/v4/internal/store"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// Spec: docs/layout.md "Settings" (start_page) and "Library page".

func TestParseAppConfigStartPage(t *testing.T) {
	cases := []struct {
		name, toml string
		want       bool
	}{
		{"tidalt default", "", false},
		{"spotify-player default", "[layout]\npreset = \"spotify-player\"\n", true},
		{"explicit Library", "[layout]\nstart_page = \"Library\"\n", true},
		{"case-insensitive", "[layout]\nstart_page = \"library\"\n", true},
		{"override preset", "[layout]\npreset = \"spotify-player\"\nstart_page = \"Queue\"\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := ParseAppConfig([]byte(c.toml))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Layout.StartLibrary != c.want {
				t.Fatalf("StartLibrary = %v, want %v", cfg.Layout.StartLibrary, c.want)
			}
		})
	}
	if _, err := ParseAppConfig([]byte("[layout]\nstart_page = \"Search\"\n")); err == nil {
		t.Fatal("start_page = \"Search\": want an error")
	}
}

func TestStartSection(t *testing.T) {
	if got := startSection(Layout{}); got != SecQueue {
		t.Fatalf("tidalt layout starts on %v, want SecQueue", got)
	}
	if got := startSection(Layout{StartLibrary: true}); got != SecLibrary {
		t.Fatalf("StartLibrary starts on %v, want SecLibrary", got)
	}
}

// Filling the startup queue from favorites must not pull the user off the
// page they are on (the Library start page, or any page they moved to).
func TestFavoritesLoadKeepsPage(t *testing.T) {
	m := Model{
		section:   SecLibrary,
		focusMain: true,
		store:     &store.SecretsStore{},
		favorites: map[int]bool{},
		libFocus:  libAlbums,
	}
	next, _ := m.Update(favoritesLoadedMsg([]tidal.Track{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}}))
	got := asModel(t, next)
	if got.section != SecLibrary {
		t.Fatalf("section = %v, want SecLibrary", got.section)
	}
	if len(got.tracks) != 2 {
		t.Fatalf("queue has %d tracks, want 2", len(got.tracks))
	}
	if len(got.pageHistory) != 0 {
		t.Fatalf("pageHistory = %v, want empty", got.pageHistory)
	}
}
