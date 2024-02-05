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
		switch key {
		case "up", "ctrl+k":
			if m.lm.currIndex < len(m.lm.List())-1 {
				m.lm.currIndex++
			}
		case "down", "ctrl+j":
			if m.lm.currIndex > 0 {
				m.lm.currIndex--
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
