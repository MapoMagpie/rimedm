package core

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
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

	// 修改权重
	modifyWeightEvent := &tui.Event{
		Keys: []string{"ctrl+up", "ctrl+down", "ctrl+left", "ctrl+right"},
		Cb: func(key string, m *tui.Model) (tea.Model, tea.Cmd) {
			curr, err := listManager.Curr()
			if err != nil {
				return m, nil
			}
			currEntry := curr.(*dict.MatchResult).Entry
			if len(currEntry.Pair) <= 2 {
				return m, nil
			}
			if key == "ctrl+up" || key == "ctrl+down" {
				list := listManager.List()
				if len(list) <= 1 {
					return m, nil
				}
				log.Println("list: ", list)
				var prev *dict.Entry = nil
				var next *dict.Entry = nil
				for i := 0; i < len(list); i++ {
					entry := list[i].(*dict.MatchResult).Entry
					if entry == currEntry {
						if i+1 < len(list) {
							prev = list[i+1].(*dict.MatchResult).Entry
						}
						if i-1 >= 0 {
							next = list[i-1].(*dict.MatchResult).Entry
						}
						break
					}
				}
				if key == "ctrl+up" && prev != nil {
					currEntry.Weight = int(math.Max(1, float64(prev.Weight-1)))
				}
				if key == "ctrl+down" && next != nil {
					currEntry.Weight = int(math.Max(1, float64(next.Weight+1)))
				}
			}
			if key == "ctrl+left" {
				currEntry.Weight = int(math.Max(1, float64(currEntry.Weight-1)))
			}
			if key == "ctrl+right" {
				currEntry.Weight += 1
			}
			pair := make([]byte, 0)
			pair = append(pair, currEntry.Pair[0]...)
			pair = append(pair, '\t')
			pair = append(pair, currEntry.Pair[1]...)
			pair = append(pair, '\t')
			pair = append(pair, strconv.Itoa(currEntry.Weight)...)
			currEntry.ReRaw(pair)
			listManager.ReSort()
			list := listManager.List()

			// 重新设置 listManager 的 currIndex为当前修改的项
			for i, item := range list {
				if item.(*dict.MatchResult).Entry == currEntry {
					listManager.SetIndex(i)
					break
				}
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

	// new model
	events := []*tui.Event{
		tui.MoveEvent,
		tui.EnterEvent,
		tui.ClearInputEvent,
		exitEvent,
		exportDictEvent,
		modifyWeightEvent,
		showHelpEvent,
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

func FlushAndSync(opts *Options, dc *dict.Dictionary, sync bool) {
	if !sync {
		return
	}
	if dc.Flush() && opts.RestartRimeCmd != "" {
		// TODO: check RestartRimeCmd, if weasel updated, the program path may be changed
		cmd := util.ExecCommand(opts.RestartRimeCmd, false)
		err := cmd.Run()
		if err != nil {
			panic(fmt.Errorf("exec restart rime cmd error:%v", err))
		}
	}
}
