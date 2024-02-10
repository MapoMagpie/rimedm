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

	// 添加菜单
	menuNameAdd := tui.Menu{Name: "Add", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		if len(m.Inputs) > 0 {
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
			if pair[1] != "" {
				filePath := file.String()
				dc.ResetMatcher()
				dc.Add(dict.NewEntryAdd([]byte(strings.Join(pair[:], "\t")), filePath))
				log.Printf("add item: %s\n", pair)
				m.Inputs = []string{}
				m.InputCursor = 0
				FlushAndSync(opts, dc, opts.SyncOnChange)
			}
		}
		return tui.ExitMenuCmd
	}}

	// 删除菜单
	menuNameDelete := tui.Menu{Name: "Delete", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		item, err := m.CurrItem()
		if err != nil {
			return
		}
		switch item := item.(type) {
		case *dict.MatchResult:
			dc.ResetMatcher()
			dc.Delete(item.Entry)
			log.Printf("delete item: %s\n", item)
			FlushAndSync(opts, dc, opts.SyncOnChange)
		}
		return tui.ExitMenuCmd
	}}

	// 修改菜单
	var modifyingItem tui.ItemRender
	menuNameModify := tui.Menu{Name: "Modify", Cb: func(m *tui.Model) (cmd tea.Cmd) {
		m.Modifying = true
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
	menuNameConfirm := tui.Menu{Name: "Confirm", Cb: func(m *tui.Model) tea.Cmd {
		m.Modifying = false
		str := strings.Join(m.Inputs, "")
		switch item := modifyingItem.(type) {
		case *dict.MatchResult:
			log.Printf("modify confirm item: %s\n", item)
			dc.ResetMatcher()
			item.Entry.ReRaw([]byte(str))
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

	searchChan := make(chan string, 20)
	listManager := tui.NewListManager(searchChan)
	listManager.SetFiles(fileNames)
	model := tui.NewModel(listManager, menuFetcher, tui.MoveEvent, tui.EnterEvent, tui.ClearInputEvent, exitEvent, exportDictEvent)
	teaProgram := tea.NewProgram(model)

	go func() {
		var cancelFunc context.CancelFunc
		ch := make(chan []*dict.MatchResult)
		timer := time.NewTicker(time.Millisecond * 100)
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
				go dc.Search(rs, ch, ctx)
			case ret := <-ch:
				if len(ret) > 0 {
					list := make([]tui.ItemRender, len(ret))
					for i, entry := range ret {
						list[i] = entry
					}
					listManager.AppendList(list)
					hasAppend = true
				}
			case <-timer.C:
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
