package dict

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/MapoMagpie/rimedm/util"
	"github.com/goccy/go-yaml"
)

type FileEntries struct {
	Err      error
	FilePath string
	RawBs    []byte
	Entries  []*Entry
	Columns  []Column
	ID       uint8
}

func (fe *FileEntries) Id() int {
	return int(fe.ID)
}

func (fe *FileEntries) String() string {
	return fe.FilePath
}

func (fe *FileEntries) Cmp(ofe any) bool {
	if o, ok := ofe.(*FileEntries); ok {
		return fe.ID > o.ID
	}
	return false
}

func LoadItems(paths ...string) (fes []*FileEntries) {
	fes = make([]*FileEntries, 0)
	ch := make(chan *FileEntries)
	var wg sync.WaitGroup
	for _, path := range paths {
		wg.Add(1)
		go loadFromFile(path, util.IDGen.NextID(), ch, &wg)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	fileNames := make(map[string]bool)
	for fe := range ch {
		if fe.Err != nil {
			fmt.Println("load dict file error: ", fe.Err)
			os.Exit(0)
		}
		if _, ok := fileNames[fe.FilePath]; ok {
			log.Printf("file [%s] already loaded", fe.FilePath)
			continue
		}
		fileNames[fe.FilePath] = true
		fes = append(fes, fe)
	}
	return
}

var (
	YAML_BEGIN = "---"
	YAML_END   = "..."
)

func loadFromFile(path string, id uint8, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	defer wg.Done()
	fe := &FileEntries{FilePath: path, Entries: make([]*Entry, 0), ID: id}
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if fe.Err = err; err != nil {
		ch <- fe
		return
	}
	defer func() {
		_ = file.Close()
	}()
	stat, err := file.Stat()
	if fe.Err = err; err != nil {
		ch <- fe
		return
	}
	bf := bytes.NewBuffer(make([]byte, 0, stat.Size()))
	_, err = io.Copy(bf, file)
	fe.RawBs = bf.Bytes()
	if fe.Err = err; err != nil {
		ch <- fe
		return
	}

	var seek int64 = 0
	// 在开始读取 码 之前，尝试先读取yaml内容，
	// 但是此文件也可能不包含yaml内容，
	// 如果不包含yaml，那么head(buffer)将与bf(buffer)一起用于读取 码
	head, size, exist := tryReadHead(bf)
	if exist {
		raw, err := io.ReadAll(head)
		if err != nil {
			panic("cant readAll bytes from head buffer")
		}
		seek = size
		config, _ := parseYAML(raw)
		fe.Columns = parseColumnsOrDefault(&config)
		loadExtendDict(path, &config, ch, wg)
	} else {
		fe.Columns = []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}
	}

	// 函数：读取 码
	readEntries := func(buf *bytes.Buffer) {
		for {
			bs, eof := buf.ReadBytes('\n')
			size := len(bs)
			seek += int64(size)
			if size > 0 {
				if bs[0] == '#' {
					continue
				}
				bs = bytes.TrimSpace(bs)
				if len(bs) == 0 {
					continue
				}
				fe.Entries = append(fe.Entries, NewEntry(bs, fe.ID, seek-int64(size), int64(size), &fe.Columns))
			}
			if eof != nil {
				break
			}
		}
	}

	if !exist {
		readEntries(head)
	}
	readEntries(bf)

	ch <- fe
}

func tryReadHead(buf *bytes.Buffer) (*bytes.Buffer, int64, bool) {
	var size int64 = 0
	lines := 0
	headBuf := bytes.NewBuffer(make([]byte, 0))
	hasDictName := false
	for {
		bs, eof := buf.ReadBytes('\n')
		headBuf.Write(bs) // keep original content
		size += int64(len(bs))
		lines += 1
		if size > 0 {
			line := strings.TrimSpace(string(bs))
			if line == YAML_BEGIN {
				continue
			}
			if strings.Index(line, "name:") == 0 {
				hasDictName = true
			}
			if line == YAML_END {
				return headBuf, size, true
			}
		}
		if eof != nil || lines > 1000 { // 我不信有人的rime dict文件中，yaml部分能超过1000行。
			break
		}
	}
	// 正常运行到这里表示文件已读完，但是没有 YAML_END，
	// 这个文件可能是无头的码表 像小鹤的txt码表那样
	// 也可能只有yaml head，却没有 YAML_END 标志，
	// 因此通过判断存在`name: xmjd6.extended`这样的行 来确定此文件仅包含yaml head
	if hasDictName {
		return headBuf, size, true
	}
	return headBuf, size, false
}

func parseColumnsOrDefault(config *YAML) []Column {
	cols := parseColumns(config)
	if len(cols) == 0 {
		// TODO: get example from content to parse cols
		return []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}
	}
	result := make([]Column, 0)
	for _, col := range cols {
		switch col {
		case "text":
			result = append(result, COLUMN_TEXT)
		case "weight":
			result = append(result, COLUMN_WEIGHT)
		case "code":
			result = append(result, COLUMN_CODE)
		case "stem":
			result = append(result, COLUMN_STEM)
		}
	}
	return result
}

func loadExtendDict(path string, config *YAML, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	paths := parseExtendPaths(path, config)
	wg.Add(len(paths))
	for _, extendPath := range paths {
		go func(newPath string, id uint8) {
			loadFromFile(newPath, id, ch, wg)
		}(extendPath, util.IDGen.NextID())
	}
}

type YAML map[string]any

func parseYAML(raw []byte) (YAML, error) {
	config := make(YAML)
	err := yaml.Unmarshal(raw, &config)
	return config, err
}

func parseExtendPaths(path string, config *YAML) []string {
	extends := make([]string, 0)
	importTables := (*config)["import_tables"]
	if importTables != nil {
		pathFixed := filepath.Dir(path) + string(os.PathSeparator)
		typeOf := reflect.TypeOf(importTables)
		if typeOf.Kind() == reflect.Slice {
			for _, extendDict := range importTables.([]any) {
				extends = append(extends, fmt.Sprintf("%s%s.dict.yaml", pathFixed, extendDict))
			}
		}
	}
	return extends
}
func parseColumns(config *YAML) []string {
	result := make([]string, 0)
	columns := (*config)["columns"]
	if columns != nil {
		typeOf := reflect.TypeOf(columns)
		if typeOf.Kind() == reflect.Slice {
			for _, col := range columns.([]any) {
				result = append(result, fmt.Sprint(col))
			}
		}
	}
	return result
}
