package main

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/junegunn/fzf/src/util"
	"log"
	"rimedictmanager/dict"
	"rimedictmanager/tui"
	"strings"
	"time"
)

func Start(opts *Options) {
	// load dict file and create dictionary
	start := time.Now()
	fes := make([]*dict.FileEntries, 0)
	for _, dictPath := range opts.DictPaths {
		fes = append(fes, dict.LoadItems(dictPath)...)
	}
	since := time.Since(start)
	log.Printf("Load %s: %s\n", opts.DictPaths, since)
	dc := dict.NewDictionary(fes, &dict.CacheMatcher{})

	// collect file name, will show on addition
	fileNames := make([]tui.ItemRender, 0)
	if opts.UserPath != "" {
		fileNames = append(fileNames, &dict.FileEntries{FilePath: opts.UserPath})
	}
	for _, f := range dc.Files() {
		fileNames = append(fileNames, f)
	}

	var listFetcher = func(m *tui.Model) []tui.ItemRender {
		var items []tui.ItemRender
		if m.ShowMenu && m.CurrMenu().Name == "Add" {
			return fileNames
		}
		key := strings.TrimSpace(strings.Join(m.Inputs, ""))
		rs := []rune(key)
		if len(rs) > 0 {
			pair := dict.ParseInput(key)
			if pair[1] != "" {
				var rsFiltered []rune
				for _, r := range rs {
					if r < 0x80 && r != ' ' && r != '\t' {
						rsFiltered = append(rsFiltered, r)
					}
				}
				rs = rsFiltered
			}
		}

		list := dc.Search(rs)
		for _, entry := range list {
			if entry.IsDelete() {
				continue
			}
			items = append(items, entry)
		}
		return items
	}

	// 添加菜单
	var menuNameAdd = tui.Menu{Name: "Add", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		cmd = func() tea.Msg {
			return tui.ExitMenuMsg(1)
		}
		if len(m.Inputs) > 0 {
			raw := strings.TrimSpace(strings.Join(m.Inputs, ""))
			if raw == "" {
				return
			}
			item, err := m.CurrItem()
			if err != nil {
				return
			}
			pair := dict.ParseInput(raw)
			if pair[1] != "" {
				filePath := item.String()
				dc.ResetMatcher()
				dc.Add(dict.NewEntryAdd([]byte(strings.Join(pair[:], "\t")), filePath))
				m.Inputs = []string{}
				m.InputCursor = 0
				sync(opts, dc, opts.SyncOnChange)
			} else {
			}
		}
		return
	}}

	// 删除菜单
	var menuNameDelete = tui.Menu{Name: "Delete", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		cmd = func() tea.Msg {
			return tui.ExitMenuMsg(1)
		}
		item, err := m.CurrItem()
		if err != nil {
			return
		}
		switch item := item.(type) {
		case *dict.Entry:
			dc.ResetMatcher()
			dc.Delete(item)
			sync(opts, dc, opts.SyncOnChange)
		}
		return
	}}

	// 修改菜单
	var modifying = false
	var modifyingItem tui.ItemRender
	var menuNameModify = tui.Menu{Name: "Modify", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		cmd = func() tea.Msg {
			return tui.ExitMenuMsg(1)
		}
		modifying = true
		item, err := m.CurrItem()
		if err != nil {
			return
		}
		modifyingItem = item
		m.Inputs = strings.Split(strings.TrimSpace(modifyingItem.String()), "")
		m.InputCursor = len(m.Inputs)
		m.MenuIndex = 0
		return
	}}

	// 确认修改菜单
	var menuNameConfirm = tui.Menu{Name: "Confirm", Cb: func(m *tui.Model) tea.Cmd {
		str := strings.Join(m.Inputs, "")
		log.Printf("modify confirm str: %s\n", str)
		switch item := modifyingItem.(type) {
		case *dict.Entry:
			log.Printf("modify confirm item: %s\n", item)
			dc.ResetMatcher()
			item.ReRaw([]byte(str))
			sync(opts, dc, opts.SyncOnChange)
		}
		modifying = false
		return func() tea.Msg {
			return tui.ExitMenuMsg(1)
		}
	}}

	var menuGroup1 = []*tui.Menu{&menuNameAdd, &menuNameDelete, &menuNameModify}
	var menuGroup2 = []*tui.Menu{&menuNameConfirm}
	var menuFetcher = func() []*tui.Menu {
		if modifying {
			return menuGroup2
		}
		return menuGroup1

	}
	exitEvent := &tui.Event{
		Keys: []string{"esc", "ctrl+c"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			if m.ShowMenu && key == "esc" {
				m.ShowMenu = false
				return m, nil
			} else {
				sync(opts, dc, !opts.SyncOnChange)
			}
			return m, tea.Quit
		},
	}
	//restartEvent := &tui.Event{
	//	Keys: []string{"ctrl+r", "ctrl+u"},
	//	Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
	//		dc.Flush()
	//		m.FreshList()
	//		return m, nil
	//	},
	//}
	m := tui.NewModel(listFetcher, menuFetcher, exitEvent)
	tui.Start(m)
}

func sync(opts *Options, dc *dict.Dictionary, ok bool) {
	if !ok {
		return
	}
	dc.Flush()
	if opts.RestartRimeCmd != "" {
		cmd := util.ExecCommand(opts.RestartRimeCmd, false)
		err := cmd.Run()
		if err != nil {
			panic(fmt.Errorf("exec restart rime cmd error:%v", err))
		}
	}
}
