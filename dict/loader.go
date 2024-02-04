package dict

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-yaml"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
)

type FileEntries struct {
	FilePath string
	RawBs    []byte
	Entries  []*Entry
	Err      error
	order    int
}

func (fe *FileEntries) String() string {
	return fe.FilePath
}

func (fe *FileEntries) Order() int {
	return fe.order
}

func LoadItems(path string) (fes []*FileEntries) {
	fes = make([]*FileEntries, 0)
	ch := make(chan *FileEntries)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		loadFromFile(path, 0, ch, &wg)
	}()
	go func() {
		wg.Wait()
		close(ch)
	}()
	for fe := range ch {
		if fe.Err != nil {
			fmt.Printf("load [%s] error: %s", fe.FilePath, fe.Err)
		}
		fes = append(fes, fe)
	}
	return
}

var (
	YamlBegin = []byte{'-', '-', '-'}
	YamlEnd   = []byte{'.', '.', '.'}
)

func loadFromFile(path string, order int, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	defer wg.Done()
	fe := &FileEntries{FilePath: path, Entries: make([]*Entry, 0), order: order}
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

	duringYaml := 0 // 0: not in yaml, 1: in yaml
	yamlContent := make([]byte, 0)
	var seek int64 = 0
	for {
		bs, eof := bf.ReadBytes('\n')
		size := len(bs)
		seek += int64(size)
		if size > 0 {
			if bs[0] == '#' {
				continue
			}
			if bytes.Equal(bytes.TrimSpace(bs), YamlBegin) {
				duringYaml = 1
				continue
			} else if bytes.Equal(bytes.TrimSpace(bs), YamlEnd) {
				loadExtendDict(path, order*10, yamlContent, ch, wg)
				duringYaml = 0
				continue
			}
			if duringYaml == 1 {
				yamlContent = append(yamlContent, bs...)
				continue
			}
			if duringYaml == 0 {
				bs = bytes.TrimSpace(bs)
				if len(bs) == 0 {
					continue
				}
				fe.Entries = append(fe.Entries, NewEntry(bs, path, seek-int64(size), int64(size)))
			}
		}
		if eof != nil {
			break
		}
	}
	if duringYaml == 1 && len(yamlContent) > 0 {
		loadExtendDict(path, order*10, yamlContent, ch, wg)
	}
	ch <- fe
}

func loadExtendDict(path string, order int, yamlContent []byte, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	paths, err := parseExtendPaths(path, yamlContent)
	if err != nil {
		log.Fatalf("parse [%s] yaml error: %s", path, err)
	}
	wg.Add(len(paths))
	for i, extendPath := range paths {
		go func(newPath string, order int) {
			loadFromFile(newPath, order, ch, wg)
		}(extendPath, order+i)
	}
}

func parseExtendPaths(path string, yamlContent []byte) ([]string, error) {
	extends := make([]string, 0)
	yamlConfig := make(map[string]interface{})
	err := yaml.Unmarshal(yamlContent, &yamlConfig)
	if err != nil {
		return extends, err
	}
	importTables := yamlConfig["import_tables"]
	if importTables != nil {
		pathFixed := filepath.Dir(path) + string(os.PathSeparator)
		typeOf := reflect.TypeOf(importTables)
		if typeOf.Kind() == reflect.Slice {
			for _, extendDict := range importTables.([]interface{}) {
				extends = append(extends, fmt.Sprintf("%s%s.dict.yaml", pathFixed, extendDict))
			}
		}
	}
	return extends, nil
}
