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
	Keys []string
	Cb   func(key string, m *Model) (tea.Model, tea.Cmd)
}

var moveEvent = &Event{
	Keys: []string{"up", "ctrl+j", "down", "ctrl+k"},
	Cb: func(key string, m *Model) (tea.Model, tea.Cmd) {
		switch key {
		case "up", "ctrl+k":
			if m.currIndex < len(m.list)-1 {
				m.currIndex++
			}
		case "down", "ctrl+j":
			if m.currIndex > 0 {
				m.currIndex--
			}
		}
		return m, nil
	},
}

var clearInputEvent = &Event{
	Keys: []string{"ctrl+x"},
	Cb: func(key string, m *Model) (tea.Model, tea.Cmd) {
		m.Inputs = []string{}
		m.InputCursor = 0
		m.FreshList()
		return m, nil
	},
}

var enterEvent = &Event{
	Keys: []string{"enter"},
	Cb: func(key string, m *Model) (tea.Model, tea.Cmd) {
		if menus := m.menuFetcher(); m.ShowMenu && len(menus) > 0 {
			if menu := menus[m.MenuIndex]; menu.Cb != nil {
				return m, menu.Cb(m)
			}
		} else {
			m.ShowMenu = true
			m.FreshList()
		}
		return m, nil
	},
}

var moveMenuEvent = &Event{
	Keys: []string{"left", "right"},
	Cb: func(key string, m *Model) (tea.Model, tea.Cmd) {
		if m.ShowMenu {
			m.menuCtl(key)
		} else {
			m.inputCtl(key)
		}
		return m, nil
	},
}
