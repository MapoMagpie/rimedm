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

type NotifitionMsg string

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
	Cb         func(m *Model) tea.Cmd
	OnSelected func(m *Model)
	Name       string
}

type ListMode uint8

var (
	LIST_MODE_DICT ListMode = 1
	LIST_MODE_FILE ListMode = 2
	LIST_MODE_HELP ListMode = 3
	LIST_MODE_EXPO ListMode = 4
)

type ListManager struct {
	SearchChan         chan<- string
	list               []ItemRender
	files              []ItemRender
	ListMode           ListMode
	currIndex          int
	fileIndex          int
	version            int
	noSort             bool
	ExportOptions      []ItemRender
	ExportOptionsIndex int
	helpIndex          int
}

func (l *ListManager) ReSort() {
	l.noSort = false
}

func (l *ListManager) SetSearchVersion(version int) {
	l.version = version
}

func NewListManager(searchChan chan<- string) *ListManager {
	return &ListManager{SearchChan: searchChan, ListMode: LIST_MODE_DICT}
}

func (l *ListManager) StepIndex(mod int) {
	var getIndex func() *int
	var getLen func() int
	switch l.ListMode {
	case LIST_MODE_DICT:
		getIndex = func() *int {
			return &l.currIndex
		}
		getLen = func() int {
			return len(l.list)
		}
	case LIST_MODE_FILE:
		getIndex = func() *int {
			return &l.fileIndex
		}
		getLen = func() int {
			return len(l.files)
		}
	case LIST_MODE_HELP:
		getIndex = func() *int {
			return &l.helpIndex
		}
		getLen = func() int {
			return len(l.Helps())
		}
	case LIST_MODE_EXPO:
		getIndex = func() *int {
			return &l.ExportOptionsIndex
		}
		getLen = func() int {
			return len(l.ExportOptions)
		}
	}
	oldIndex := getIndex()
	newIndex := *oldIndex + mod
	if newIndex < 0 {
		newIndex = 0
	} else if newIndex >= getLen() {
		newIndex = getLen() - 1
	}
	*oldIndex = newIndex
}

func (l *ListManager) List() ([]ItemRender, int) {
	switch l.ListMode {
	case LIST_MODE_DICT:
		le := len(l.list)
		// fix currIndex
		if le == 0 {
			l.currIndex = 0
		} else if l.currIndex > le-1 {
			l.currIndex = le - 1
		}
		if !l.noSort {
			list := l.list // avoid data race
			sort.Slice(list, func(i, j int) bool {
				return list[i].Cmp(list[j])
			})
			// if le != len(l.list) {
			// 	log.Printf("list has changed, old len: %d new len: %d", le, len(l.list))
			// }
			l.noSort = true
		}
		return l.list, l.currIndex
	case LIST_MODE_FILE:
		return l.files, l.fileIndex
	case LIST_MODE_HELP:
		return l.Helps(), l.helpIndex
	case LIST_MODE_EXPO:
		return l.ExportOptions, l.ExportOptionsIndex
	default:
		return []ItemRender{}, 0
	}
}

func (l *ListManager) Helps() []ItemRender {
	list := []ItemRender{
		StringRender("Ctrl+S:     手动同步，如果没有启用自动同步，"),
		StringRender("            可通过此按键手动将变更同步至文件，并部署Rime"),
		StringRender("Ctrl+O:     导出码表到当前目录下的output.txt文件中"),
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
	ListManager  *ListManager
	menuFetcher  func(m *Model) []*Menu
	menus        []*Menu
	MenusShowing bool
	eventManager *EventManager
	Inputs       []string
	MenuIndex    int
	wx           int
	hx           int
	InputCursor  int
	Modifying    bool
	message      string
}

func (m *Model) CurrItem() (ItemRender, error) {
	return m.ListManager.Curr()
}

func (m *Model) CurrFile() (ItemRender, error) {
	files := m.ListManager.files
	if len(files) == 0 {
		return nil, errors.New("empty file list")
	}
	if m.ListManager.fileIndex > len(files)-1 {
		return nil, errors.New("file index out of range")
	}
	return files[m.ListManager.fileIndex], nil
}

func (m *Model) MessageOr(newMessage string) string {
	if m.message == "" {
		return newMessage
	} else {
		return m.message
	}
}
func (m *Model) ClearMessage() {
	m.message = ""
}

func (m *Model) CurrItemFile() string {
	currItem, err := m.CurrItem()
	if err != nil {
		return ""
	}
	for _, file := range m.ListManager.files {
		if file.Id() == currItem.Id() {
			return filepath.Base(file.String())
		}
	}
	return ""
}

func (m *Model) FreshList() {
	if !m.MenusShowing {
		m.ListManager.newSearch(m.Inputs)
	}
}

func (m *Model) ShowMenus() {
	m.MenusShowing = true
	m.menus = m.menuFetcher(m)
	if len(m.menus) > 0 {
		if m.MenuIndex >= len(m.menus) {
			m.MenuIndex = 0
		}
		if menu := m.menus[m.MenuIndex]; menu.OnSelected != nil {
			menu.OnSelected(m)
		}
	}
}

func (m *Model) HideMenus() {
	m.MenusShowing = false
	m.menus = m.menuFetcher(m)
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
	switch key {
	case "left":
		if m.MenuIndex > 0 {
			m.MenuIndex--
		}
	case "right":
		if m.MenuIndex < len(m.menus)-1 {
			m.MenuIndex++
		}
	default:
		if len(key) == 1 && len(m.menus) > 0 {
			num, err := strconv.Atoi(key)
			// select menu by numpad
			if err == nil && num > 0 && num < 10 {
				index := num - 1
				if index < len(m.menus) {
					m.MenuIndex = index
				}
			} else {
				// select menu by letter
				for i, menu := range m.menus {
					if strings.ToLower(menu.Name[:1]) == key {
						m.MenuIndex = i
						break
					}
				}
			}
		}
	}
	if m.menus[m.MenuIndex].OnSelected != nil {
		m.menus[m.MenuIndex].OnSelected(m)
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
	currIndex := m.ListManager.currIndex
	list, currIndex := m.ListManager.List()

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
			if i == currIndex {
				sb.WriteString(fmt.Sprintf("\x1b[31m>\x1b[0m \x1b[1;4;35m\x1b[47m%3d: %s\x1b[0m\n", i+1, line))
			} else {
				sb.WriteString(fmt.Sprintf("> %3d: %s\n", i+1, line))
			}
		}
	}
	// footer: search count and filepath of current, or notifition
	sb.WriteString(fmt.Sprintf("Total: %d; %s\n", le, m.MessageOr(m.CurrItemFile())))
	sb.WriteString("Press[Enter:操作][Ctrl+X:清空输入][Ctrl+S:同步][ESC:退出][Ctrl+H:帮助]\n")
	if m.Modifying {
		line = strings.Replace(line, "---------", "Modifying", 1)
	}
	sb.WriteString(line + "\n")

	if len(m.menus) > 0 {
		sb.WriteString(": ")
		for i, menu := range m.menus {
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
		if m.MenusShowing {
			m.menuCtl(key)
		} else { // search
			m.inputCtl(key)
		}
	case FreshListMsg:
		m.FreshList()
	case ExitMenuMsg:
		m.ListManager.ListMode = LIST_MODE_DICT
		m.HideMenus()
		m.FreshList()
	case NotifitionMsg:
		m.message = string(msg)
		m.FreshList()
	case tea.WindowSizeMsg:
		m.wx = msg.Width
		m.hx = msg.Height
	}
	return m, nil
}

func (m *Model) AddEvent(events ...*Event) {
	m.eventManager.Add(events...)
}

func NewModel(listManager *ListManager, menuFetcher func(m *Model) []*Menu) *Model {
	fd := os.Stderr.Fd()
	wx, hx, err := term.GetSize(int(fd))
	if err != nil {
		fmt.Printf("Terminal GetSize Error: %v\n", err)
		os.Exit(1)
	}
	model := &Model{ListManager: listManager, wx: wx, hx: hx, menuFetcher: menuFetcher, eventManager: NewEventManager(), message: ""}
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
