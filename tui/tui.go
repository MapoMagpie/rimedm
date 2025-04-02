package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

type ExitMenuMsg int

func ExitMenuCmd() tea.Msg {
	return ExitMenuMsg(1)
}

type FreshListMsg int

func FreshListCmd() tea.Msg {
	return FreshListMsg(1)
}

type ItemRender interface {
	Id() int
	String() string
	Cmp(other any) bool
}

type StringRender string

func (h StringRender) Id() int {
	return 0
}

func (h StringRender) String() string {
	return string(h)
}

func (h StringRender) Cmp(_ any) bool {
	return true
}

type Menu struct {
	Cb   func(m *Model) tea.Cmd
	Name string
}

type ListManager struct {
	SearchChan  chan<- string
	list        []ItemRender
	files       []ItemRender
	ShowingHelp bool
	currIndex   int
	fileIndex   int
	version     int
	noSort      bool
}

func (l *ListManager) ReSort() {
	l.noSort = false
}

func (l *ListManager) SetSearchVersion(version int) {
	l.version = version
}

func NewListManager(searchChan chan<- string) *ListManager {
	return &ListManager{SearchChan: searchChan}
}

func (l *ListManager) List() []ItemRender {
	list := l.list
	le := len(list)
	// fix currIndex
	if le == 0 {
		l.currIndex = 0
	} else if l.currIndex > le-1 {
		l.currIndex = le - 1
	}
	if !l.noSort {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Cmp(list[j])
		})
		l.noSort = true
	}
	return list
}

func (l *ListManager) Files() []ItemRender {
	return l.files
}

func (l *ListManager) Helps() []ItemRender {
	list := []ItemRender{
		StringRender("Ctrl+S:     手动同步，如果没有启用自动同步，"),
		StringRender("            可通过此按键手动将变更同步至文件，并部署Rime"),
		StringRender("Ctrl+Right: 修改权重，将当前项的权重加一"),
		StringRender("Ctrl+Left:  修改权重，将当前项的权重减一"),
		StringRender("Ctrl+Down:  修改权重，将当前项的权重增加到下一项之前"),
		StringRender("Ctrl+Up:    修改权重，将当前项的权重降低到上一项之后"),
		StringRender("Enter:      显示菜单"),
		StringRender("菜单项: [A添加] 将输入的内容(字词 字母码)添加到码表中，"),
		StringRender("                支持乱序，如(字母码 权重 字词)输入，"),
		StringRender("                上下方向键选择要添加到的文件"),
		StringRender("菜单项: [M修改] 修改选择的项(高亮)，"),
		StringRender("                回车后，输入框中的内容会被设置，"),
		StringRender("                修改后，再次回车确认修改"),
		StringRender("菜单项: [D删除] 将选择的项(高亮)从码表中删除，通过上下键选择"),
	}
	slices.Reverse(list)
	return list
}

func (l *ListManager) Curr() (ItemRender, error) {
	if len(l.list) == 0 {
		return nil, errors.New("empty list")
	} else {
		return l.list[l.currIndex], nil
	}
}

func (l *ListManager) NewList(version int) {
	l.version = version
	l.list = make([]ItemRender, 0)
}

func (l *ListManager) newSearch(inputs []string) {
	// log.Printf("send search key: %v", strings.Join(inputs, ""))
	l.SearchChan <- strings.Join(inputs, "")
	// log.Printf("send search key finshed")
}

func (l *ListManager) AppendList(rs []ItemRender, version int) {
	if l.version == version {
		l.list = append(l.list, rs...)
		l.noSort = false
	}
}

func (l *ListManager) SetFiles(files []ItemRender) {
	l.files = files
}

func (l *ListManager) SetIndex(index int) {
	if index < 0 {
		index = 0
	} else if index > len(l.list)-1 {
		index = len(l.list) - 1
	}
	l.currIndex = index
}

func (l *ListManager) CurrIndex() int {
	return l.currIndex
}

type Model struct {
	lm           *ListManager
	menuFetcher  func(bool) []*Menu
	eventManager *EventManager
	Inputs       []string
	MenuIndex    int
	wx           int
	hx           int
	InputCursor  int
	ShowMenu     bool
	Modifying    bool
}

func (m *Model) CurrItem() (ItemRender, error) {
	return m.lm.Curr()
}

func (m *Model) CurrFile() (ItemRender, error) {
	files := m.lm.Files()
	if len(files) == 0 {
		return nil, errors.New("empty file list")
	}
	if m.lm.fileIndex > len(files)-1 {
		return nil, errors.New("file index out of range")
	}
	return files[m.lm.fileIndex], nil
}

func (m *Model) CurrItemFile() string {
	currItem, err := m.CurrItem()
	if err != nil {
		return ""
	}
	for _, file := range m.lm.Files() {
		if file.Id() == currItem.Id() {
			return filepath.Base(file.String())
		}
	}
	return ""
}

func (m *Model) CurrMenu() *Menu {
	menus := m.menuFetcher(m.Modifying)
	if m.MenuIndex < len(menus) {
		return menus[m.MenuIndex]
	}
	return nil
}

func (m *Model) FreshList() {
	if !m.ShowMenu {
		m.lm.newSearch(m.Inputs)
	}
}

// var asciiPattern, _ = regexp.Compile("^[a-zA-z\\d]$")
func (m *Model) inputCtl(key string) {
	switch strings.ToLower(key) {
	case "backspace":
		if m.InputCursor > 0 {
			m.Inputs = slices.Delete(m.Inputs, m.InputCursor-1, m.InputCursor)
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
		// 过滤组合键，如shift+j ctrl+left
		if strings.Contains(key, "shift+") || strings.Contains(key, "ctrl+") || strings.Contains(key, "alt+") {
			return
		}
		if key == "tab" {
			key = "\t"
		}
		split := strings.Split(key, "")
		if m.InputCursor < len(m.Inputs) {
			m.Inputs = append(m.Inputs[:m.InputCursor], append(split, m.Inputs[m.InputCursor:]...)...)
		} else {
			m.Inputs = append(m.Inputs, split...)
		}
		m.InputCursor += len(split)
		m.FreshList()
	}
}

func (m *Model) menuCtl(key string) {
	menus := m.menuFetcher(m.Modifying)
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
			num, err := strconv.Atoi(key)
			// select menu by numpad
			if err == nil && num > 0 && num < 10 {
				index := num - 1
				if index < len(menus) {
					m.MenuIndex = index
				}
			} else {
				// select menu by letter
				for i, menu := range menus {
					if strings.ToLower(menu.Name[:1]) == key {
						m.MenuIndex = i
						break
					}
				}
			}
		}
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
	list := m.lm.List()
	currIndex := m.lm.currIndex
	if m.lm.ShowingHelp {
		list = m.lm.Helps()
		currIndex = 0
	} else if m.ShowMenu && m.CurrMenu().Name == "A添加" {
		list = m.lm.Files()
		currIndex = m.lm.fileIndex
	}
	le := len(list)
	// body empty lines
	if remain := renderCnt - le; remain > 0 {
		sb.WriteString(strings.Repeat("\n", remain))
	}
	if le < renderCnt {
		renderCnt = le
	}
	if renderCnt > 0 {
		top, bot := renderCnt-1, 0
		if currIndex > top {
			top, bot = currIndex, currIndex-renderCnt+1
		}
		for i := top; i >= bot; i-- {
			line := list[i].String()
			line = truncateString(line, m.wx-16)
			if i == currIndex && !m.lm.ShowingHelp {
				sb.WriteString(fmt.Sprintf("\x1b[31m>\x1b[0m \x1b[1;4;35m\x1b[47m%3d: %s\x1b[0m\n", i+1, line))
			} else {
				sb.WriteString(fmt.Sprintf("> %3d: %s\n", i+1, line))
			}
		}
	}
	// footer
	sb.WriteString(fmt.Sprintf("Total: %d; %s\n", le, m.CurrItemFile()))
	sb.WriteString("Press[Enter:操作][Ctrl+X:清空输入][Ctrl+S:同步][ESC:退出][Ctrl+H:帮助]\n")
	if m.Modifying {
		line = strings.Replace(line, "---------", "Modifying", 1)
	}
	sb.WriteString(line + "\n")

	if menus := m.menuFetcher(m.Modifying); m.ShowMenu && len(menus) > 0 {
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
	case FreshListMsg:
		m.FreshList()
	case ExitMenuMsg:
		m.ShowMenu = false
		m.FreshList()
	case tea.WindowSizeMsg:
		m.wx = msg.Width
		m.hx = msg.Height
	}
	return m, nil
}

func NewModel(lm *ListManager, menuFetcher func(bool) []*Menu, events ...*Event) *Model {
	fd := os.Stderr.Fd()
	wx, hx, err := term.GetSize(int(fd))
	if err != nil {
		fmt.Printf("Terminal GetSize Error: %v\n", err)
		os.Exit(1)
	}
	model := &Model{lm: lm, wx: wx, hx: hx, menuFetcher: menuFetcher, eventManager: NewEventManager(events...)}
	return model
}
func truncateString(s string, wx int) string {
	width := 0
	end := len(s)
	for i, r := range s {
		w := runewidth.RuneWidth(r) // 获取字符宽度
		if width+w > wx {
			end = i
			break
		}
		width += w
	}
	return s[:end]
}
