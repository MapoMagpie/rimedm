package tui

import tea "github.com/charmbracelet/bubbletea"

type EventManager struct {
	keyMap map[string]*Event
}

func NewEventManager(events ...*Event) *EventManager {
	e := &EventManager{keyMap: make(map[string]*Event)}
	e.Add(events...)
	return e
}

func (e *EventManager) Find(key string) *Event {
	return e.keyMap[key]
}

func (e *EventManager) Add(events ...*Event) {
	for _, event := range events {
		for _, key := range event.Keys {
			e.keyMap[key] = event
		}
	}
}

type Event struct {
	Cb   func(key string, m *Model) (tea.Model, tea.Cmd)
	Keys []string
}

var MoveEvent = &Event{
	Keys: []string{"up", "ctrl+j", "down", "ctrl+k"},
	Cb: func(key string, m *Model) (tea.Model, tea.Cmd) {
		list := m.lm.List()
		currIndex := &m.lm.currIndex
		if m.ShowMenu && m.CurrMenu().Name == "Add" {
			list = m.lm.files
			currIndex = &m.lm.fileIndex
		}
		switch key {
		case "up", "ctrl+k":
			if *currIndex < len(list)-1 {
				*currIndex++
			}
		case "down", "ctrl+j":
			if *currIndex > 0 {
				*currIndex--
			}
		}
		return m, nil
	},
}

var ClearInputEvent = &Event{
	Keys: []string{"ctrl+x"},
	Cb: func(_ string, m *Model) (tea.Model, tea.Cmd) {
		m.Inputs = []string{}
		m.InputCursor = 0
		m.FreshList()
		return m, nil
	},
}

var EnterEvent = &Event{
	Keys: []string{"enter"},
	Cb: func(_ string, m *Model) (tea.Model, tea.Cmd) {
		if m.lm.ShowingHelp {
			m.lm.ShowingHelp = false
			return m, nil
		}
		if menus := m.menuFetcher(m.Modifying); m.ShowMenu && len(menus) > 0 {
			if menu := menus[m.MenuIndex]; menu.Cb != nil {
				return m, menu.Cb(m)
			}
		} else {
			m.ShowMenu = true
		}
		return m, nil
	},
}
