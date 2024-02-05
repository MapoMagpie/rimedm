package tui

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type ExitMenuMsg int

func ExitMenuCmd() tea.Msg {
	return ExitMenuMsg(1)
}

type FreshListMsg int

type ItemRender interface {
	String() string
	Order() int
}

type Menu struct {
	Cb   func(m *Model) tea.Cmd
	Name string
}

type ListManager struct {
	SearchChan chan<- string
	list       []ItemRender
	currIndex  int
	setVer     int
	getVer     int
	lock       sync.Mutex
}

func NewListManager(searchChan chan<- string) *ListManager {
	return &ListManager{SearchChan: searchChan, lock: sync.Mutex{}}
}

func (l *ListManager) List() []ItemRender {
	l.lock.Lock()
	defer l.lock.Unlock()
	list := l.list
	if l.getVer != l.setVer {
		sort.Slice(list, func(i, j int) bool {
			r := list[i].Order() - list[j].Order()
			return r > 0
		})
		l.getVer = l.setVer
	}
	return list
}

func (l *ListManager) Curr() (ItemRender, error) {
	if len(l.list) == 0 {
		return nil, errors.New("empty list")
	}
	return l.list[l.currIndex], nil
}

func (l *ListManager) newSearch(inputs []string) {
	l.lock.Lock()
	l.list = make([]ItemRender, 0)
	l.SearchChan <- strings.Join(inputs, "")
	l.lock.Unlock()
}

func (l *ListManager) AppendList(rs []ItemRender) {
	l.lock.Lock()
	l.setVer++
	l.list = append(l.list, rs...)
	l.lock.Unlock()
}

type Model struct {
	lm           *ListManager
	menuFetcher  func() []*Menu
	eventManager *EventManager
	Inputs       []string
	MenuIndex    int
	wx           int
	hx           int
	InputCursor  int
	ShowMenu     bool
}

func (m *Model) CurrItem() (ItemRender, error) {
	return m.lm.Curr()
}

func (m *Model) CurrMenu() *Menu {
	menus := m.menuFetcher()
	if m.MenuIndex < len(menus) {
		return menus[m.MenuIndex]
	}
	return nil
}

func (m *Model) FreshList() {
	m.lm.newSearch(m.Inputs)
}

// var asciiPattern, _ = regexp.Compile("^[a-zA-z\\d]$")
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
	// m.FreshList()
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
	list := m.lm.List()
	le := len(list)
	if remain := renderCnt - le; remain > 0 {
		sb.WriteString(strings.Repeat("\n", remain))
	}
	if le < renderCnt {
		renderCnt = le
	}
	start, end := renderCnt-1, 0
	if m.lm.currIndex > renderCnt-1 {
		start, end = m.lm.currIndex, m.lm.currIndex-renderCnt+1
	}
	for i := start; i >= end; i-- {
		if i == m.lm.currIndex {
			sb.WriteString(fmt.Sprintf("\x1b[31m>\x1b[0m \x1b[1;4;35m\x1b[47m%3d: %s\x1b[0m\n", i+1, list[i].String()))
		} else {
			sb.WriteString(fmt.Sprintf("> %3d: %s\n", i+1, list[i].String()))
		}
	}
	// footer
	sb.WriteString(fmt.Sprintf("Total: %d\n", le))
	sb.WriteString("Press[Enter:Menu][Ctrl+X:Clear Input][Ctrl+C|ESC:Quit][Ctrl+O:Export Dict]\n")
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		event := m.eventManager.Find(strings.ToLower(key))
		if event != nil {
			return event.Cb(key, m)
		}
		if m.ShowMenu {
			m.menuCtl(key)
		} else { // search
			m.inputCtl(key)
		}
	case ExitMenuMsg:
		m.ShowMenu = false
		m.FreshList()
	case tea.WindowSizeMsg:
		m.wx = msg.Width
		m.hx = msg.Height
	case FreshListMsg:
		log.Printf("FreshListMsg")
	}
	return m, nil
}

func NewModel(lm *ListManager, menuFetcher func() []*Menu, events ...*Event) *Model {
	fd := os.Stderr.Fd()
	wx, hx, err := term.GetSize(int(fd))
	if err != nil {
		fmt.Printf("Terminal GetSize Error: %v\n", err)
		os.Exit(1)
	}
	return &Model{lm: lm, wx: wx, hx: hx, menuFetcher: menuFetcher, eventManager: NewEventManager(events...)}
}
