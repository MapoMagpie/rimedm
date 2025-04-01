package dict

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
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

func (fe *FileEntries) String() string {
	return fe.FilePath
}

func (fe *FileEntries) Order() int {
	return int(fe.ID)
}

func LoadItems(paths ...string) (fes []*FileEntries) {
	fes = make([]*FileEntries, 0)
	ch := make(chan *FileEntries)
	var wg sync.WaitGroup
	for _, path := range paths {
		wg.Add(1)
		go loadFromFile(path, ch, &wg)
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
	YAML_BEGIN = []byte{'-', '-', '-'}
	YAML_END   = []byte{'.', '.', '.'}
)

func loadFromFile(path string, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	defer wg.Done()
	fe := &FileEntries{FilePath: path, Entries: make([]*Entry, 0), ID: util.IDGen.NextID()}
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
			log.Fatal("cant readAll bytes from head buffer")
		}
		seek = size
		config, _ := parseYAML(raw)
		fe.Columns = parseColumnsOrDefault(&config)
		loadExtendDict(path, &config, ch, wg)
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
				fe.Entries = append(fe.Entries, NewEntry(bs, fe.ID, seek-int64(size), int64(size)))
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
	for {
		bs, eof := buf.ReadBytes('\n')
		headBuf.Write(bs) // keep original content
		size += int64(len(bs))
		lines += 1
		if size > 0 {
			if bytes.Equal(bytes.TrimSpace(bs), YAML_BEGIN) {
				continue
			}
			if bytes.Equal(bytes.TrimSpace(bs), YAML_END) {
				return headBuf, size, true
			}
		}
		if eof != nil || lines > 1000 { // 我不信有人的rime dict文件中，yaml部分能超过1000行。
			break
		}
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
		go func(newPath string) {
			loadFromFile(newPath, ch, wg)
		}(extendPath)
	}
}

type YAML map[string]any

func parseYAML(raw []byte) (YAML, error) {
	config := make(YAML)
	err := yaml.Unmarshal(raw, &config)
	// if err != nil {
	// 	log.Fatalf("parse [%s] yaml error: %s", path, err)
	// }
	return config, err
}

func parseExtendPaths(path string, config *YAML) []string {
	extends := make([]string, 0)
	importTables := (*config)["import_tables"]
	if importTables != nil {
		pathFixed := filepath.Dir(path) + string(os.PathSeparator)
		typeOf := reflect.TypeOf(importTables)
		if typeOf.Kind() == reflect.Slice {
			for _, extendDict := range importTables.([]interface{}) {
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
