package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type ItemRender interface {
	String() string
}

type Menu struct {
	Name string
	Cb   func(m *Model) tea.Cmd // Callback
}

type Model struct {
	currIndex    int
	listFetcher  func(m *Model) []ItemRender
	list         []ItemRender
	ShowMenu     bool
	menuFetcher  func() []*Menu
	MenuIndex    int
	wx           int // terminal width
	hx           int // terminal height
	key          string
	InputCursor  int
	Inputs       []string
	eventManager *EventManager
}

func (m *Model) CurrItem() (ItemRender, error) {
	if len(m.list) == 0 {
		return nil, errors.New("empty list")
	}
	return m.list[m.currIndex], nil
}

func (m *Model) CurrMenu() *Menu {
	menus := m.menuFetcher()
	if m.MenuIndex < len(menus) {
		return menus[m.MenuIndex]
	}
	return nil
}

//var asciiPattern, _ = regexp.Compile("^[a-zA-z\\d]$")

func (m *Model) inputCtl(key string) {
	switch strings.ToLower(key) {
	case "backspace":
		if m.InputCursor > 0 {
			m.Inputs = append(m.Inputs[:m.InputCursor-1], m.Inputs[m.InputCursor:]...)
			m.InputCursor--
			m.FreshList()
		}
	case "left":
		if m.InputCursor > 0 {
			m.InputCursor--
		}
	case "right":
		if m.InputCursor < len(m.Inputs) {
			m.InputCursor++
		}
	case "enter": // do nothing
	default:
		if key == "tab" {
			key = "\t"
		}
		if m.InputCursor < len(m.Inputs) {
			m.Inputs = append(m.Inputs[:m.InputCursor+1], m.Inputs[m.InputCursor:]...)
			m.Inputs[m.InputCursor] = key
		} else {
			m.Inputs = append(m.Inputs, key)
		}
		m.InputCursor++
		m.FreshList()
	}
}

func (m *Model) menuCtl(key string) {
	menus := m.menuFetcher()
	switch key {
	case "left":
		if m.MenuIndex > 0 {
			m.MenuIndex--
		}
	case "right":
		if m.MenuIndex < len(menus)-1 {
			m.MenuIndex++
		}
	default:
		if len(key) == 1 && len(menus) > 0 {
			for i, menu := range menus {
				if strings.ToLower(menu.Name[:1]) == key {
					m.MenuIndex = i
					break
				}
			}
		}
	}
	m.FreshList()
}

func (m *Model) FreshList() {
	m.list = m.listFetcher(m)
	if ln := len(m.list); ln == 0 {
		m.currIndex = 0
	} else if m.currIndex >= ln {
		m.currIndex = len(m.list) - 1
	}
}

func (m *Model) Init() tea.Cmd {
	m.FreshList()
	return nil
}

func (m *Model) View() string {
	var sb strings.Builder
	// header
	line := strings.Repeat("-", m.wx)
	sb.WriteString(line + "\n")
	renderCnt := m.hx - 5 // 5 is header lines(1) + footer lines(4)
	// body
	le := len(m.list)
	if remain := renderCnt - le; remain > 0 {
		sb.WriteString(strings.Repeat("\n", remain))
	}
	if le < renderCnt {
		renderCnt = le
	}
	start, end := renderCnt-1, 0
	if m.currIndex > renderCnt-1 {
		start, end = m.currIndex, m.currIndex-renderCnt+1
	}
	for i := start; i >= end; i-- {
		if i == m.currIndex {
			sb.WriteString(fmt.Sprintf("\x1b[31m>\x1b[0m \x1b[1;4;35m\x1b[47m%3d: %s\x1b[0m\n", i+1, m.list[i].String()))
		} else {
			sb.WriteString(fmt.Sprintf("> %3d: %s\n", i+1, m.list[i].String()))
		}
	}
	//footer
	sb.WriteString(fmt.Sprintf("Total: %d\n", le))
	sb.WriteString("Press[Enter:Menu][Ctrl+X:Clear Input][Ctrl+C|ESC:Quit]\n")
	sb.WriteString(line + "\n")

	if menus := m.menuFetcher(); m.ShowMenu && len(menus) > 0 {
		sb.WriteString(": ")
		for i, menu := range menus {
			nameR := []rune(menu.Name)
			if i == m.MenuIndex {
				sb.WriteString(fmt.Sprintf(" \x1b[5;1;31m[\x1b[0m\x1b[35m%s\x1b[0m%s\x1b[5;1;31m]\x1b[0m ", string(nameR[0]), string(nameR[1:])))
			} else {
				sb.WriteString(fmt.Sprintf(" [\x1b[35m%s\x1b[0m%s] ", string(nameR[0]), string(nameR[1:])))
			}
		}
	} else {
		inputCursor := "\x1b[5;1;31m|\x1b[0m"
		inp := strings.Join(m.Inputs[:m.InputCursor], "") + inputCursor + strings.Join(m.Inputs[m.InputCursor:], "")
		sb.WriteString(fmt.Sprintf(":%s", inp))
	}
	s := sb.String()
	return s
}

type ExitMenuMsg int

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		event := m.eventManager.Find(strings.ToLower(key))
		if event != nil {
			return event.Cb(key, m)
		} else {
			if m.ShowMenu {
				m.menuCtl(key)
			} else {
				m.inputCtl(key)
			}
		}
		m.key = key
	case ExitMenuMsg:
		m.ShowMenu = false
		m.FreshList()
	case tea.WindowSizeMsg:
		m.wx = msg.Width
		m.hx = msg.Height
	}
	return m, nil
}

func NewModel(listFetcher func(m *Model) []ItemRender, menuFetcher func() []*Menu, events ...*Event) *Model {
	fd := os.Stderr.Fd()
	wx, hx, err := term.GetSize(int(fd))
	if err != nil {
		fmt.Printf("Terminal GetSize Error: %v\n", err)
		os.Exit(1)
	}
	em := NewEventManager(exitEvent, moveEvent, enterEvent, moveMenuEvent, clearInputEvent)
	em.Add(events...)
	return &Model{listFetcher: listFetcher, wx: wx, hx: hx, menuFetcher: menuFetcher, eventManager: em}
}

func Start(m tea.Model) {
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Printf("Tui Program Error: %v\n", err)
		os.Exit(1)
	}
}
