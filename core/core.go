package core

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/MapoMagpie/rimedm/dict"
	"github.com/MapoMagpie/rimedm/tui"
	mutil "github.com/MapoMagpie/rimedm/util"

	tea "github.com/charmbracelet/bubbletea"
)

func Start(opts *Options) {
	// load dict file and create dictionary
	start := time.Now()
	fes := dict.LoadItems(opts.DictPaths...)
	sort.Slice(fes, func(i, j int) bool {
		return fes[j].Cmp(fes[i])
	})
	since := time.Since(start)
	log.Printf("Load %s: %s\n", opts.DictPaths, since)
	dc := dict.NewDictionary(fes, &dict.CacheMatcher{})
	if opts.Export != "" {
		columns := parseColumnsFromArgments(opts.ExportColumns)
		dc.ExportDict(opts.Export, columns)
		return
	}

	// collect file name, will show on addition
	fileNames := make([]tui.ItemRender, 0)
	for _, fe := range fes {
		if fe.FilePath == opts.UserPath {
			fileNames = append([]tui.ItemRender{fe}, fileNames...)
		} else {
			fileNames = append(fileNames, fe)
		}
	}

	searchChan := make(chan string, 20)
	listManager := tui.NewListManager(searchChan)
	listManager.SetFiles(fileNames)

	// 添加菜单
	menuNameAdd := tui.Menu{Name: "A添加",
		Cb: func(m *tui.Model) (cmd tea.Cmd) {
			if len(m.Inputs) == 0 {
				return tui.ExitMenuCmd
			}
			file, err := m.CurrFile()
			fe := file.(*dict.FileEntries)
			if err != nil {
				panic(fmt.Sprintf("add to dict error: %v", err))
			}
			raw := strings.TrimSpace(strings.Join(m.Inputs, ""))
			if raw == "" {
				return
			}
			pair, cols := dict.ParseInput(raw, slices.Index(fe.Columns, dict.COLUMN_STEM) != -1)
			if len(pair) == 0 { // allow add single column
				return tui.ExitMenuCmd
			}
			data, _ := dict.ParseData(pair, &cols)
			data.ResetColumns(&fe.Columns)
			curr, err := listManager.Curr()
			if err == nil { // 自动修改权重
				currEntry := curr.(*dict.MatchResult).Entry
				currEntryData := currEntry.Data()
				if currEntryData.Code == data.Code && data.Weight == 0 { // 新加项的码如果和当前项的码相同，则自动修改新加项的权重
					data.Weight = currEntryData.Weight + 1
				}
			}
			entryRaw := data.ToString()
			dc.Add(dict.NewEntryAdd(entryRaw, fe.ID, data))
			log.Printf("add item: %s\n", entryRaw)
			m.Inputs = strings.Split(data.Code, "")
			m.InputCursor = len(m.Inputs)
			dc.ResetMatcher()
			FlushAndSync(opts, dc, opts.SyncOnChange)
			return tui.ExitMenuCmd
		},
		OnSelected: func(m *tui.Model) {
			m.ListManager.ListMode = tui.LIST_MODE_FILE
		},
	}

	// 删除菜单
	menuNameDelete := tui.Menu{Name: "D删除",
		Cb: func(m *tui.Model) (cmd tea.Cmd) {
			item, err := m.CurrItem()
			if err != nil {
				m.Inputs = []string{}
				m.InputCursor = 0
				return tui.ExitMenuCmd
			}
			switch item := item.(type) {
			case *dict.MatchResult:
				dc.Delete(item.Entry)
				log.Printf("delete item: %s\n", item)
				dc.ResetMatcher()
				FlushAndSync(opts, dc, opts.SyncOnChange)
			}
			return tui.ExitMenuCmd
		},
		OnSelected: func(m *tui.Model) {
			m.ListManager.ListMode = tui.LIST_MODE_DICT
		},
	}

	// 修改菜单
	var modifyingItem tui.ItemRender
	menuNameModify := tui.Menu{Name: "M修改",
		Cb: func(m *tui.Model) (cmd tea.Cmd) {
			item, err := m.CurrItem()
			if err != nil {
				m.Inputs = []string{}
				m.InputCursor = 0
				return tui.ExitMenuCmd
			}
			m.Modifying = true
			modifyingItem = item
			m.Inputs = strings.Split(strings.TrimSpace(modifyingItem.String()), "")
			m.InputCursor = len(m.Inputs)
			m.MenuIndex = 0
			return tui.ExitMenuCmd
		},
		OnSelected: func(m *tui.Model) {
			m.ListManager.ListMode = tui.LIST_MODE_DICT
		},
	}

	// 确认修改菜单
	menuNameConfirm := tui.Menu{Name: "C确认", Cb: func(m *tui.Model) tea.Cmd {
		m.Modifying = false
		raw := strings.Join(m.Inputs, "")
		switch item := modifyingItem.(type) {
		case *dict.MatchResult:
			feIndex := slices.IndexFunc(fes, func(fe *dict.FileEntries) bool {
				return fe.ID == item.Entry.FID
			})
			if feIndex == -1 {
				panic("modify item error: this item does not belong to any file")
			}
			fe := fes[feIndex]
			pair, cols := dict.ParseInput(raw, slices.Index(fe.Columns, dict.COLUMN_STEM) != -1)
			if len(pair) > 1 {
				data, _ := dict.ParseData(pair, &cols)
				data.ResetColumns(&fe.Columns)
				entryRaw := data.ToString()
				log.Printf("modify confirm item: %s\n", entryRaw)
				item.Entry.ReRaw(entryRaw)
				if feIndex > -1 {
					item.Entry.ReRaw(data.ToStringWithColumns(&fes[feIndex].Columns))
				}
				m.Inputs = strings.Split(data.Code, "")
				m.InputCursor = len(m.Inputs)
			}
			dc.ResetMatcher()
			FlushAndSync(opts, dc, opts.SyncOnChange)
		}
		return tui.ExitMenuCmd
	}}

	// 退出到列表菜单
	menuNameBack := tui.Menu{Name: "B返回", Cb: func(m *tui.Model) tea.Cmd {
		m.MenusShowing = false
		listManager.ListMode = tui.LIST_MODE_DICT
		return tui.ExitMenuCmd
	}}

	showMenus := []*tui.Menu{&menuNameAdd, &menuNameModify, &menuNameDelete, &menuNameBack}
	modifyingMenus := []*tui.Menu{&menuNameConfirm, &menuNameBack}
	helpMenus := []*tui.Menu{&menuNameBack}
	exportMenus := []*tui.Menu{&menuNameBack, &menuNameBack} // will change the first element later
	menuFetcher := func(m *tui.Model) []*tui.Menu {
		menus := []*tui.Menu{}
		switch m.ListManager.ListMode {
		case tui.LIST_MODE_DICT:
			if m.MenusShowing {
				if m.Modifying {
					menus = modifyingMenus
				} else {
					menus = showMenus
				}
			}
		case tui.LIST_MODE_FILE:
			menus = showMenus
		case tui.LIST_MODE_HELP:
			menus = helpMenus
		case tui.LIST_MODE_EXPO:
			menus = exportMenus
		}
		if len(menus) > 0 && m.MenuIndex >= len(menus) {
			m.MenuIndex = 0
		}
		return menus
	}
	model := tui.NewModel(listManager, menuFetcher)
	teaProgram := tea.NewProgram(model)

	listManager.ExportOptions = []tui.ItemRender{
		tui.StringRender("字词"),
		tui.StringRender("编码"),
		tui.StringRender("权重"),
		tui.StringRender("---------排除标记-----------"),
		tui.StringRender("如将权重向上移动至排除标记后将不输出权重"),
		tui.StringRender("使用Ctrl+Up或Ctrl+Down调整下方的输出格式"),
		tui.StringRender("默认以 字词<TAB>编码<TAB>权重 每行输出到文件"),
	}
	// 导出码表菜单
	menuNameExport := tui.Menu{Name: "E导出", Cb: func(m *tui.Model) tea.Cmd {
		m.ListManager.ListMode = tui.LIST_MODE_DICT
		m.HideMenus()
		go func() {
			filePath := "output.txt"
			columns := make([]dict.Column, 0)
			options := listManager.ExportOptions[:3]
			for _, opt := range options {
				match := true
				switch opt.String() {
				case "字词":
					columns = append(columns, dict.COLUMN_TEXT)
				case "编码":
					columns = append(columns, dict.COLUMN_CODE)
				case "权重":
					columns = append(columns, dict.COLUMN_WEIGHT)
				default:
					match = false
				}
				if !match {
					break
				}
			}
			time.Sleep(time.Second)
			if len(columns) > 0 {
				dc.ExportDict(filePath, columns)
				teaProgram.Send(tui.NotifitionMsg("完成导出码表 > output.txt"))
			} else {
				teaProgram.Send(tui.NotifitionMsg("没有东西要导出"))
			}
		}()
		return func() tea.Msg {
			return tui.NotifitionMsg("正在导出码表...")
		}
	}}
	exportMenus[0] = &menuNameExport

	// events
	exitEvent := &tui.Event{
		Keys: []string{"esc", "ctrl+c", "ctrl+d"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			if key == "esc" {
				if m.Modifying || m.MenusShowing {
					if m.Modifying {
						m.Inputs = []string{}
						m.InputCursor = 0
						m.Modifying = false
					}
					m.ListManager.ListMode = tui.LIST_MODE_DICT
					m.HideMenus()
					return m, nil
				}
			}
			FlushAndSync(opts, dc, true)
			return m, tea.Quit
		},
	}

	// 修改权重，这是一个高频操作，通过debouncer延迟同步到文件。
	modifyWeightDebouncer := mutil.NewDebouncer(time.Millisecond * 1000) // 一秒后
	modifyWeightEvent := &tui.Event{
		Keys: []string{"ctrl+up", "ctrl+down", "ctrl+left", "ctrl+right"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			// adjust the columns of export dict
			if m.ListManager.ListMode == tui.LIST_MODE_EXPO {
				var newIndex int
				if key == "ctrl+up" {
					newIndex = listManager.ExportOptionsIndex + 1
					if newIndex >= len(listManager.ExportOptions) {
						return m, nil
					}
				} else if key == "ctrl+down" {
					newIndex = listManager.ExportOptionsIndex - 1
					if newIndex < 0 {
						return m, nil
					}
				}
				listManager.ExportOptions[listManager.ExportOptionsIndex], listManager.ExportOptions[newIndex] =
					listManager.ExportOptions[newIndex], listManager.ExportOptions[listManager.ExportOptionsIndex]
				listManager.ExportOptionsIndex = newIndex
				return m, func() tea.Msg { return 0 } // trigger bubbletea update
			}
			curr, err := listManager.Curr()
			if err != nil {
				return m, nil
			}
			changed := false
			currEntry := curr.(*dict.MatchResult).Entry
			currEntryData := currEntry.Data()
			if key == "ctrl+up" || key == "ctrl+down" {
				list, _ := listManager.List()
				if len(list) <= 1 {
					return m, nil
				}
				// log.Println("list: ", list)
				var prev *dict.Entry = nil
				var next *dict.Entry = nil
				for i := range list {
					entry := list[i].(*dict.MatchResult).Entry
					if entry == currEntry {
						if i+1 < len(list) {
							next = list[i+1].(*dict.MatchResult).Entry
						}
						if i-1 >= 0 {
							prev = list[i-1].(*dict.MatchResult).Entry
						}
						break
					}
				}
				if key == "ctrl+up" && next != nil {
					currEntryData.Weight = int(math.Max(1, float64(next.Data().Weight-1)))
					changed = true
				}
				if key == "ctrl+down" && prev != nil {
					currEntryData.Weight = int(math.Max(1, float64(prev.Data().Weight+1)))
					changed = true
				}
			}
			if key == "ctrl+left" {
				currEntryData.Weight = int(math.Max(1, float64(currEntryData.Weight-1)))
				changed = true
			}
			if key == "ctrl+right" {
				currEntryData.Weight = int(math.Max(1, float64(currEntryData.Weight+1)))
				changed = true
			}
			if changed {
				currEntry.ReRaw(currEntryData.ToString())
				listManager.ReSort()
				list, _ := listManager.List()
				// 重新设置 listManager 的 currIndex为当前修改的项
				for i, item := range list {
					if item.(*dict.MatchResult).Entry == currEntry {
						listManager.SetIndex(i)
						break
					}
				}
				// 延迟同步到文件
				// log.Println("modify weight sync: ", currEntry.Raw())
				modifyWeightDebouncer.Do(func() {
					FlushAndSync(opts, dc, opts.SyncOnChange)
				})
			}
			return m, func() tea.Msg { return 0 } // trigger bubbletea update
		},
	}

	// 显示帮助
	showHelpEvent := &tui.Event{
		Keys: []string{"ctrl+h"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			if m.ListManager.ListMode == tui.LIST_MODE_HELP {
				m.ListManager.ListMode = tui.LIST_MODE_DICT
				return m, tui.ExitMenuCmd
			} else {
				m.ListManager.ListMode = tui.LIST_MODE_HELP
				m.ShowMenus()
				return m, func() tea.Msg { return 0 } // trigger bubbletea update
			}
		},
	}
	// 显示导出码表
	showExportDictEvent := &tui.Event{
		Keys: []string{"ctrl+o"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			if m.ListManager.ListMode == tui.LIST_MODE_EXPO {
				m.ListManager.ListMode = tui.LIST_MODE_DICT
				m.MenusShowing = false
				return m, tui.ExitMenuCmd
			} else {
				m.ListManager.ListMode = tui.LIST_MODE_EXPO
				m.ShowMenus()
				return m, func() tea.Msg { return 0 } // trigger bubbletea update
			}
		},
	}
	// 重新部署，强制保存变更到文件，并执行rime部署指令。
	redeployEvent := &tui.Event{
		Keys: []string{"ctrl+s"},
		Cb: func(_ string, m *tui.Model) (tea.Model, tea.Cmd) {
			FlushAndSync(opts, dc, true)
			return m, nil
		},
	}
	// new model
	events := []*tui.Event{
		tui.MoveEvent,
		tui.EnterEvent,
		tui.ClearInputEvent,
		exitEvent,
		redeployEvent,
		modifyWeightEvent,
		showHelpEvent,
		showExportDictEvent,
	}
	model.AddEvent(events...)
	// 输入处理 搜索
	go func() {
		var cancelFunc context.CancelFunc
		resultChan := make(chan dict.MatchResultChunk)
		timer := time.NewTicker(time.Millisecond * 100) // debounce
		hasAppend := false
		searchVersion := 0
		for {
			select {
			case raw := <-searchChan: // 等待搜索term
				ctx, cancel := context.WithCancel(context.Background())
				if cancelFunc != nil {
					cancelFunc()
				}
				cancelFunc = cancel
				var rs string
				useColumn := dict.COLUMN_CODE
				if len(raw) > 0 {
					// if the input has code(码) then change the rs(search term) to code
					pairs, cols := dict.ParseInput(raw, false)
					if len(pairs) > 0 {
						codeIndex := slices.Index(cols, dict.COLUMN_CODE)
						if codeIndex != -1 {
							useColumn = dict.COLUMN_CODE
							rs = pairs[codeIndex]
						} else {
							if len(pairs) == 1 && mutil.IsAscii(pairs[0]) {
								useColumn = dict.COLUMN_CODE
								rs = pairs[0]
							} else {
								textIndex := slices.Index(cols, dict.COLUMN_TEXT)
								useColumn = dict.COLUMN_TEXT
								rs = pairs[textIndex]
							}
						}
					}
				}
				searchVersion++
				listManager.NewList(searchVersion)
				go dc.Search(rs, useColumn, searchVersion, resultChan, ctx)
			case ret := <-resultChan: // 等待搜索结果
				list := make([]tui.ItemRender, len(ret.Result))
				for i, entry := range ret.Result {
					list[i] = entry
				}
				// log.Printf("search result: %d, version: %d", len(list), ret.Version)
				listManager.AppendList(list, ret.Version)
				hasAppend = true
			case <-timer.C: // debounce, if appended then flush
				if hasAppend {
					hasAppend = false
					teaProgram.Send(0) // trigger bubbletea update
				}
			}
		}
	}()

	if _, err := teaProgram.Run(); err != nil {
		fmt.Printf("Tui Program Error: %v\n", err)
		os.Exit(1)
	}
}

var lock *mutil.FLock = mutil.NewFLock()

// 同步变更到文件中，如果启用了自动部署Rime的功能则调用部署指令
func FlushAndSync(opts *Options, dc *dict.Dictionary, sync bool) {
	// 此操作的阻塞的，但可能被异步调用，因此加上防止重复调用机制
	if !lock.Should() {
		return
	}
	defer lock.Done()
	if !sync {
		return
	}
	if dc.Flush() && opts.RestartRimeCmd != "" {
		// TODO: check RestartRimeCmd, if weasel updated, the program path may be changed
		cmd := mutil.Run(opts.RestartRimeCmd)
		err := cmd.Run()
		if err != nil {
			panic(fmt.Errorf("exec restart rime cmd error:%v", err))
		}
	}
}

func parseColumnsFromArgments(s string) []dict.Column {
	splits := strings.SplitSeq(s, ",")
	columns := make([]dict.Column, 0)
	for sp := range splits {
		switch strings.ToLower(sp) {
		case "text":
			columns = append(columns, dict.COLUMN_TEXT)
		case "code":
			columns = append(columns, dict.COLUMN_CODE)
		case "weight":
			columns = append(columns, dict.COLUMN_WEIGHT)
		}
	}
	return columns
}
