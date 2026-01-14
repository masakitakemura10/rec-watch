package tui

import (
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mt4110/rec-watch/internal/config"
	"github.com/mt4110/rec-watch/internal/watcher"
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0"))
)

type tickMsg time.Time

type Model struct {
	cfg *config.Config
	// State variables (Queue, Recent, Stats)
	// State variables (Queue, Recent, Stats)
	queue   []string
	paths   []string // Parallel to queue to store full paths
	history []string

	cursor int // Cursor position in queue

	sub chan interface{} // Subscription to watcher events
}

func NewModel(cfg *config.Config, sub chan interface{}) Model {
	return Model{
		cfg:     cfg,
		queue:   []string{},
		paths:   []string{},
		history: []string{},
		sub:     sub,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		waitForActivity(m.sub),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.queue)-1 {
				m.cursor++
			}
		case " ":
			// QuickLook
			if len(m.queue) > 0 && m.cursor < len(m.paths) {
				path := m.paths[m.cursor]
				return m, tea.ExecProcess(exec.Command("qlmanage", "-p", path), func(err error) tea.Msg {
					return nil
				})
			}
		}
	case tickMsg:
		return m, tickCmd()

	// Watcher Events
	case watcher.FileFoundEvent:
		m.queue = append(m.queue, msg.Name) // Store just name or path?
		// Store path in separate slice? Or upgrade queue type.
		// Let's assume queue items line up with a paths slice.
		m.paths = append(m.paths, msg.Path)
		return m, waitForActivity(m.sub)

	case watcher.StartConvertEvent:
		// Remove from queue/paths?
		// Find index
		idx := -1
		for i, p := range m.paths {
			if p == msg.Path {
				idx = i
				break
			}
		}
		if idx >= 0 {
			// Remove from queue and paths
			m.queue = append(m.queue[:idx], m.queue[idx+1:]...)
			m.paths = append(m.paths[:idx], m.paths[idx+1:]...)
			// Adjust cursor
			if m.cursor >= len(m.queue) && m.cursor > 0 {
				m.cursor--
			}
		}

		m.history = append([]string{"🚀 Processing: " + msg.Path}, m.history...)
		return m, waitForActivity(m.sub)

	case watcher.SuccessEvent:
		m.history = append([]string{"✅ Done: " + msg.Path}, m.history...)
		return m, waitForActivity(m.sub)

	case watcher.FailureEvent:
		m.history = append([]string{"❌ Failed: " + msg.Path}, m.history...)
		return m, waitForActivity(m.sub)
	}
	return m, nil
}

func (m Model) View() string {
	s := titleStyle.Render("🔴 RecWatch TUI") + "\n\n"

	s += "監視中: " + fmt.Sprintf("%v", m.cfg.WatchDirs) + "\n\n"

	s += "処理待ちキュー:\n"
	if len(m.queue) == 0 {
		s += statusStyle.Render("  (ハトサ、ヒマサ)") + "\n"
	}
	for i, q := range m.queue {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += fmt.Sprintf("%s%s\n", cursor, q)
	}

	s += "\n最近の履歴:\n"
	if len(m.history) == 0 {
		s += statusStyle.Render("  (履歴なし)") + "\n"
	}
	for _, h := range m.history {
		s += fmt.Sprintf("  %s\n", h)
	}

	s += "\n操作: [q] 終了  [↑/↓] 選択  [Space] プレビュー(QuickLook)\n"
	return s
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForActivity(sub chan interface{}) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}
