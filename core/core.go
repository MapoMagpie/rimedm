package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/MapoMagpie/rimedm/dict"
	"github.com/MapoMagpie/rimedm/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/junegunn/fzf/src/util"
)

func Start(opts *Options) {
	// load dict file and create dictionary
	start := time.Now()
	fes := dict.LoadItems(opts.DictPaths...)
	sort.Slice(fes, func(i, j int) bool {
		return fes[i].Order() < fes[j].Order()
	})
	since := time.Since(start)
	log.Printf("Load %s: %s\n", opts.DictPaths, since)
	dc := dict.NewDictionary(fes, &dict.CacheMatcher{})

	// collect file name, will show on addition
	fileNames := make([]tui.ItemRender, 0)
	if opts.UserPath != "" {
		fileNames = append(fileNames, &dict.FileEntries{FilePath: opts.UserPath})
	}
	for _, f := range dc.Files() {
		if f.FilePath == opts.UserPath {
			continue
		}
		fileNames = append(fileNames, f)
	}

	searchChan := make(chan string, 20)
	listManager := tui.NewListManager(searchChan)
	listManager.SetFiles(fileNames)

	// 添加菜单
	menuNameAdd := tui.Menu{Name: "Add", Cb: func(m *tui.Model) (cmd tea.Cmd) {
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
		pair := dict.ParseInput(raw)
		if pair[1] == "" {
			return tui.ExitMenuCmd
		}
		if pair[2] == "" {
			curr, err := listManager.Curr()
			// list := listManager.List()
			// if err == nil && len(list) > 0 {
			// 	sameCodeList := make([]*dict.Entry, 0)
			// 	currEntry := curr.(*dict.MatchResult).Entry
			// 	currEntryIndex := -1
			// 	for _, item := range list {
			// 		entry := item.(*dict.MatchResult)
			// 		if len(entry.Entry.Pair) >= 3 && string(entry.Entry.Pair[1]) == pair[1] && len(entry.Entry.Pair[2]) > 0 { // 过滤掉码不相同的以及没有权重的
			// 			sameCodeList = append(sameCodeList, entry.Entry)
			// 		}
			// 	}
			// 	if currEntryIndex > -1 {
			// 		pair[2] = fmt.Sprintf("%d", currEntry.Weight-1)
			// 	}
			// 	log.Println("curr entry: ", currEntry, ", same code list: ", sameCodeList)
			// }
			if err == nil {
				currEntry := curr.(*dict.MatchResult).Entry
				if string(currEntry.Pair[1]) == pair[1] && len(currEntry.Pair) >= 3 && len(currEntry.Pair[2]) > 0 {
					pair[2] = fmt.Sprintf("%d", currEntry.Weight-1)
					log.Println("curr entry: ", currEntry, ", new Entry pair: ", pair)
				}
			}
		}
		filePath := file.String()
		dc.Add(dict.NewEntryAdd([]byte(strings.Join(pair[:], "\t")), filePath))
		log.Printf("add item: %s\n", pair)
		m.Inputs = strings.Split(pair[1], "")
		m.InputCursor = len(m.Inputs)
		dc.ResetMatcher()
		FlushAndSync(opts, dc, opts.SyncOnChange)
		return tui.ExitMenuCmd
	}}

	// 删除菜单
	menuNameDelete := tui.Menu{Name: "Delete", Cb: func(m *tui.Model) (cmd tea.Cmd) {
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
	menuNameModify := tui.Menu{Name: "Modify", Cb: func(m *tui.Model) (cmd tea.Cmd) {
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
	menuNameConfirm := tui.Menu{Name: "Confirm", Cb: func(m *tui.Model) tea.Cmd {
		m.Modifying = false
		raw := strings.Join(m.Inputs, "")
		switch item := modifyingItem.(type) {
		case *dict.MatchResult:
			log.Printf("modify confirm item: %s\n", item)
			pair := dict.ParseInput(raw)
			if pair[1] != "" {
				item.Entry.ReRaw([]byte(strings.Join(pair[:], "\t")))
				m.Inputs = strings.Split(pair[1], "")
				m.InputCursor = len(m.Inputs)
			}
			dc.ResetMatcher()
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
			FlushAndSync(opts, dc, !opts.SyncOnChange)
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

	// new model
	events := []*tui.Event{
		tui.MoveEvent,
		tui.EnterEvent,
		tui.ClearInputEvent,
		exitEvent,
		exportDictEvent,
	}
	model := tui.NewModel(listManager, menuFetcher, events...)
	teaProgram := tea.NewProgram(model)

	go func() {
		var cancelFunc context.CancelFunc
		resultChan := make(chan []*dict.MatchResult)
		timer := time.NewTicker(time.Millisecond * 100) // debounce
		hasAppend := false
		for {
			select {
			case raw := <-searchChan:
				listManager.NewList()
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
				go dc.Search(rs, resultChan, ctx)
			case ret := <-resultChan:
				list := make([]tui.ItemRender, len(ret))
				for i, entry := range ret {
					list[i] = entry
				}
				listManager.AppendList(list)
				hasAppend = true
			case <-timer.C: // debounce, if appended then flush
				if hasAppend {
					hasAppend = false
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

func FlushAndSync(opts *Options, dc *dict.Dictionary, ok bool) {
	if !ok {
		return
	}
	dc.Flush()
	if opts.RestartRimeCmd != "" {
		// TODO: check RestartRimeCmd, if weasel updated, the program path may be changed
		cmd := util.ExecCommand(opts.RestartRimeCmd, false)
		err := cmd.Run()
		if err != nil {
			panic(fmt.Errorf("exec restart rime cmd error:%v", err))
		}
	}
}
