package core

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
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
		return fes[i].Cmp(fes[j])
	})
	since := time.Since(start)
	log.Printf("Load %s: %s\n", opts.DictPaths, since)
	dc := dict.NewDictionary(fes, &dict.CacheMatcher{})

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
	menuNameAdd := tui.Menu{Name: "A添加", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		if len(m.Inputs) == 0 {
			return tui.ExitMenuCmd
		}
		file, err := m.CurrFile()
		if err != nil {
			log.Fatalf("add to dict error: %v", err)
			return
		}
		raw := strings.TrimSpace(strings.Join(m.Inputs, ""))
		if raw == "" {
			return
		}
		pair, cols := dict.ParseInput(raw)
		if len(pair) == 0 {
			return tui.ExitMenuCmd
		}
		data, _ := dict.ParseData(pair, &cols)
		curr, err := listManager.Curr()
		if err == nil {
			// 自动修改权重
			currEntry := curr.(*dict.MatchResult).Entry
			currEntryData := currEntry.Data()
			if currEntryData.Code == data.Code && data.Weight == 0 { // 新加项的码如果和当前项的码相同，则自动修改新加项的权重
				data.Weight = currEntryData.Weight + 1
				// log.Println("curr entry: ", currEntry, ", new Entry pair: ", pairs)
			}
		}
		fe := file.(*dict.FileEntries)
		data.ResetColumns(&fe.Columns)
		dc.Add(dict.NewEntryAdd(data.ToString(), fe.ID, data))
		log.Printf("add item: %s\n", pair)
		m.Inputs = strings.Split(data.Code, "")
		m.InputCursor = len(m.Inputs)
		dc.ResetMatcher()
		FlushAndSync(opts, dc, opts.SyncOnChange)
		return tui.ExitMenuCmd
	}}

	// 删除菜单
	menuNameDelete := tui.Menu{Name: "D删除", Cb: func(m *tui.Model) (cmd tea.Cmd) {
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
	}}

	// 修改菜单
	var modifyingItem tui.ItemRender
	menuNameModify := tui.Menu{Name: "M修改", Cb: func(m *tui.Model) (cmd tea.Cmd) {
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
	}}

	// 确认修改菜单
	menuNameConfirm := tui.Menu{Name: "确认", Cb: func(m *tui.Model) tea.Cmd {
		m.Modifying = false
		raw := strings.Join(m.Inputs, "")
		switch item := modifyingItem.(type) {
		case *dict.MatchResult:
			pair, cols := dict.ParseInput(raw)
			if len(pair) > 1 {
				data, _ := dict.ParseData(pair, &cols)
				feIndex := slices.IndexFunc(fes, func(fe *dict.FileEntries) bool {
					return fe.ID == item.Entry.FID
				})
				if feIndex > -1 {
					item.Entry.ReRaw(data.ToStringWithColumns(&fes[feIndex].Columns))
					log.Printf("modify confirm item: %s\n", item)
				}
				m.Inputs = strings.Split(data.Code, "")
				m.InputCursor = len(m.Inputs)
			}
			dc.ResetMatcher()
			listManager.ReSort()
			FlushAndSync(opts, dc, opts.SyncOnChange)
		}
		return tui.ExitMenuCmd
	}}

	menuFetcher := func(modifying bool) []*tui.Menu {
		if modifying {
			return []*tui.Menu{&menuNameConfirm}
		} else {
			return []*tui.Menu{&menuNameAdd, &menuNameDelete, &menuNameModify}
		}
	}

	exitEvent := &tui.Event{
		Keys: []string{"esc", "ctrl+c", "ctrl+d"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			if key == "esc" {
				if m.ShowMenu {
					m.ShowMenu = false
					return m, nil
				}
				if m.Modifying {
					m.Modifying = false
					return m, nil
				}
			}
			FlushAndSync(opts, dc, true)
			return m, tea.Quit
		},
	}

	// 简单地合并码表并输出到当前目录中
	exportDictEvent := &tui.Event{
		Keys: []string{"ctrl+o"},
		Cb: func(_ string, m *tui.Model) (tea.Model, tea.Cmd) {
			filePath := "output.txt"
			dc.ExportDict(filePath)
			return m, nil
		},
	}

	// 修改权重，这是一个高频操作，通过debouncer延迟同步到文件。
	modifyWeightDebouncer := mutil.NewDebouncer(time.Millisecond * 1000) // 一秒后
	modifyWeightEvent := &tui.Event{
		Keys: []string{"ctrl+up", "ctrl+down", "ctrl+left", "ctrl+right"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			curr, err := listManager.Curr()
			if err != nil {
				return m, nil
			}
			changed := false
			currEntry := curr.(*dict.MatchResult).Entry
			currEntryData := currEntry.Data()
			if key == "ctrl+up" || key == "ctrl+down" {
				list := listManager.List()
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
				list := listManager.List()
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
			listManager.ShowingHelp = !listManager.ShowingHelp
			return m, func() tea.Msg { return 0 } // trigger bubbletea update
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
		exportDictEvent,
		redeployEvent,
		modifyWeightEvent,
		showHelpEvent,
	}
	model := tui.NewModel(listManager, menuFetcher, events...)
	teaProgram := tea.NewProgram(model)

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
					pairs, cols := dict.ParseInput(raw)
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
				listManager.AppendList(list, ret.Version)
				// log.Println("list manager append list: ", len(list))
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
		shell := os.Getenv("SHELL")
		if len(shell) == 0 {
			shell = "sh"
		}
		cmd := exec.Command(shell, "-c", opts.RestartRimeCmd)
		err := cmd.Run()
		if err != nil {
			panic(fmt.Errorf("exec restart rime cmd error:%v", err))
		}
	}
}
