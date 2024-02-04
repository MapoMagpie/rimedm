package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"rimedictmanager/dict"
	"rimedictmanager/tui"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/junegunn/fzf/src/util"
)

func Start(opts *Options) {
	// load dict file and create dictionary
	start := time.Now()
	fes := make([]*dict.FileEntries, 0)
	for _, dictPath := range opts.DictPaths {
		ret := dict.LoadItems(dictPath)
		sort.Slice(ret, func(i, j int) bool {
			return ret[i].Order() < ret[j].Order()
		})
		fes = append(fes, ret...)
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

	// 添加菜单
	var menuNameAdd = tui.Menu{Name: "Add", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		if len(m.Inputs) > 0 {
			raw := strings.TrimSpace(strings.Join(m.Inputs, ""))
			if raw == "" {
				return
			}
			item, err := m.CurrItem() // file name
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
		return tui.ExitMenuCmd
	}}

	// 删除菜单
	var menuNameDelete = tui.Menu{Name: "Delete", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		item, err := m.CurrItem()
		if err != nil {
			return
		}
		switch item := item.(type) {
		case *dict.MatchResult:
			dc.ResetMatcher()
			dc.Delete(item.Entry)
			sync(opts, dc, opts.SyncOnChange)
		}
		return tui.ExitMenuCmd
	}}

	// 修改菜单
	var modifying = false
	var modifyingItem tui.ItemRender
	var menuNameModify = tui.Menu{Name: "Modify", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		modifying = true
		item, err := m.CurrItem()
		if err != nil {
			return
		}
		modifyingItem = item
		m.Inputs = strings.Split(strings.TrimSpace(modifyingItem.String()), "")
		m.InputCursor = len(m.Inputs)
		m.MenuIndex = 0
		return tui.ExitMenuCmd
	}}

	// 确认修改菜单
	var menuNameConfirm = tui.Menu{Name: "Confirm", Cb: func(m *tui.Model) tea.Cmd {
		str := strings.Join(m.Inputs, "")
		log.Printf("modify confirm str: %s\n", str)
		switch item := modifyingItem.(type) {
		case *dict.MatchResult:
			log.Printf("modify confirm item: %s\n", item)
			dc.ResetMatcher()
			item.Entry.ReRaw([]byte(str))
			sync(opts, dc, opts.SyncOnChange)
		}
		modifying = false
		return tui.ExitMenuCmd
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

	// 简单地合并码表并输出到当前目录中
	exportDictEvent := &tui.Event{
		Keys: []string{"ctrl+o"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			filePath := "output.txt"
			dc.ExportDict(filePath)
			return m, nil
		},
	}

	searchChan := make(chan string)
	listManager := tui.ListManager{SearchChan: searchChan}
	model := tui.NewModel(&listManager, menuFetcher, exitEvent, exportDictEvent)
	teaProgram := tea.NewProgram(model)

	go func() {
		var cancelFunc context.CancelFunc
		ch := make(chan []*dict.MatchResult)
		for {
			select {
			case raw := <-searchChan:
				if model.ShowMenu && model.CurrMenu().Name == "Add" {
					listManager.AppendList(fileNames)
					teaProgram.Send(tui.FreshListMsg(0))
					continue
				}
				ch = make(chan []*dict.MatchResult)
				ctx, cancel := context.WithCancel(context.Background())
				if cancelFunc != nil {
					cancelFunc()
				}
				cancelFunc = cancel
				rs := []rune(raw)
				if len(raw) > 0 {
					pair := dict.ParseInput(raw)
					if pair[1] != "" {
						rs = []rune(pair[1])
					}
				}
				go dc.Search(rs, ch, ctx)
			case ret := <-ch:
				if ret != nil {
					list := make([]tui.ItemRender, len(ret))
					for i, entry := range ret {
						list[i] = entry
					}
					log.Println("recv list: ", len(list))
					listManager.AppendList(list)
					teaProgram.Send(tui.FreshListMsg(0))
				}
			}
		}
	}()

	if _, err := teaProgram.Run(); err != nil {
		fmt.Printf("Tui Program Error: %v\n", err)
		os.Exit(1)
	}
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
