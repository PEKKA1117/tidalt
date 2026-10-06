package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpRows lists every bound action, grouped, with its keys in
// spotify-player's notation. Unbound actions are left out.
func (m *Model) helpRows() []string {
	t := m.activeTheme()
	km := m.keys()
	const keyCol = 18
	var rows []string
	lastGroup := actionGroup(-1)
	for _, a := range actions {
		seqs := km.Keys(a.act)
		if len(seqs) == 0 {
			continue
		}
		if a.group != lastGroup {
			if lastGroup >= 0 {
				rows = append(rows, "")
			}
			rows = append(rows, t.CmdGroup.Render(groupNames[a.group]))
			lastGroup = a.group
		}
		labels := make([]string, len(seqs))
		for i, s := range seqs {
			labels[i] = keyLabel(s)
		}
		keys := strings.Join(labels, ", ")
		pad := strings.Repeat(" ", max(keyCol-lipgloss.Width(keys), 1))
		rows = append(rows, " "+t.KeyBarKey.Render(keys)+pad+a.desc)
	}
	return rows
}

// helpBodyHeight is how many help rows fit in the popup.
func (m *Model) helpBodyHeight() int {
	return max(m.height-6, 3)
}

// updateHelp scrolls the help overlay; Esc or ? closes it.
func (m Model) updateHelp(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxScroll := max(len(m.helpRows())-m.helpBodyHeight(), 0)
	switch k.String() {
	case keyEsc, "?":
		m.overlay = OverlayNone
	case keyDown, "j":
		m.helpScroll = min(m.helpScroll+1, maxScroll)
	case keyUp, "k":
		m.helpScroll = max(m.helpScroll-1, 0)
	case "pgdown", "ctrl+f":
		m.helpScroll = min(m.helpScroll+m.helpBodyHeight(), maxScroll)
	case "pgup", "ctrl+b":
		m.helpScroll = max(m.helpScroll-m.helpBodyHeight(), 0)
	}
	return m, nil
}

// renderHelp renders the key-binding help popup.
func (m *Model) renderHelp(t Theme) string {
	rows := m.helpRows()
	bodyH := m.helpBodyHeight()
	start := min(m.helpScroll, max(len(rows)-bodyH, 0))
	end := min(start+bodyH, len(rows))
	visible := rows[start:end]

	w := min(max(m.width*2/3, 44), 64)
	h := min(len(visible)+2, m.height-2)
	return renderPanel(t, "KEYS · j/k scroll · Esc close", true, w, max(h, 4), strings.Join(visible, "\n"))
}

// keyHint returns the label of the first key bound to a, or "" when a is
// unbound, for the footer.
func (m *Model) keyHint(a Action) string {
	if seqs := m.keys().Keys(a); len(seqs) > 0 {
		return keyLabel(seqs[0])
	}
	return ""
}

// chordHint is the footer shown while a chord is pending: the typed prefix,
// then each key that completes it and what it does.
func (m *Model) chordHint() [][2]string {
	prefix := strings.Join(m.pendingKeys, " ")
	next := m.keys().continuations(prefix)
	items := make([][2]string, 0, len(next)+1)
	items = append(items, [2]string{keyLabel(prefix) + "-", ""})
	for _, b := range next {
		items = append(items, [2]string{keyLabel(b.seq), actionIndex[b.act].desc})
	}
	return items
}
