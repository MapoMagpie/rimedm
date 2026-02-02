package dict

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"unicode"

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
		go loadFromFile(path, util.IDGen.NextID(), nil, ch, &wg)
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

func loadFromFile(path string, id uint8, columns *[]Column, ch chan<- *FileEntries, wg *sync.WaitGroup) {
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
	head, size, existHead := tryReadHead(bf)
	if existHead {
		raw, err := io.ReadAll(head)
		if err != nil {
			panic("cant readAll bytes from head buffer")
		}
		seek = size
		config, _ := parseYAML(raw)
		fe.Columns, _ = parseColumnsFromYAML(&config)
		loadExtendDict(path, &config, &fe.Columns, ch, wg)
	}
	if fe.Columns == nil && columns != nil {
		fe.Columns = *columns
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
				// 如果 Columns 不存在，则从第一个有效行中解析 列的顺序
				if fe.Columns == nil {
					// 通过制表符`\t`分隔后，分隔物大于一个则为有效行
					splits := bytes.Split(bs, []byte{'\t'})
					if len(splits) < 2 {
						continue
					}
					fe.Columns, err = tryParseColumns(splits)
					if err != nil {
						fmt.Fprintf(os.Stderr, "\x1b[31m警告：无法自动解析 列序([字词 编码 [权重?]])\x1b[0m\n"+
							`码表中的第一个有效项必须包含 英文和汉字，以制表符隔开，如 [nihao 你好]、[你好 nihao]
当前第一个有效项为：[%s]，位于： %s

！！！现启用默认的列序： [字词 编码 权重]，若与码表实际的列序不同，将导致无法搜索与修改！
请解决此问题在 %s 中
方式一：使码表的第一个有效项包含英文和汉字；
方式二：在码表的配置中指定columns
columns:
  - text
  - weight
  - code
`, string(bs), path, path)
						fe.Columns = []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}
					}
				}
				fe.Entries = append(fe.Entries, NewEntry(bs, fe.ID, seek-int64(size), int64(size), &fe.Columns))
			}
			if eof != nil {
				break
			}
		}
	}
	if !existHead {
		readEntries(head)
	}
	readEntries(bf)

	ch <- fe
}

func tryParseColumns(splits [][]byte) ([]Column, error) {
	codeCount, textCount := 0, 0
	cols := make([]Column, len(splits))
	for i, sp := range splits {
		ru := []rune(string(sp))
		cols[i] = runesColunmType(ru)
		switch cols[i] {
		case COLUMN_CODE:
			codeCount += 1
		case COLUMN_TEXT:
			textCount += 1
		}
	}
	if codeCount == 1 && textCount == 1 {
		return cols, nil
	}
	return nil, fmt.Errorf("无法解析 列序(英文码 汉字 [权重])， 码表中的第一个有效项必须包含 英文和汉字，以制表符隔开，顺序随意，权重可选。当前code数量: %d, 当前textCount数量: %d ", codeCount, textCount)
}

func runesColunmType(rus []rune) Column {
	hasDigit := false
	hasLetter := false
	for _, ru := range rus {
		if ru > unicode.MaxASCII {
			return COLUMN_TEXT
		}
		if unicode.IsDigit(ru) {
			hasDigit = true
		} else if unicode.IsLetter(ru) {
			hasLetter = true
		} else {
		}
	}
	if hasDigit && !hasLetter {
		return COLUMN_WEIGHT
	}
	return COLUMN_CODE
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

func parseColumnsFromYAML(config *YAML) ([]Column, error) {
	cols := parseColumns(config)
	if len(cols) == 0 {
		return nil, errors.New("YAML中不存在列声明")
		// TODO: get example from content to parse cols
		// return []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}
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
	return result, nil
}

func loadExtendDict(path string, config *YAML, columns *[]Column, ch chan<- *FileEntries, wg *sync.WaitGroup) {
	paths := parseExtendPaths(path, config)
	wg.Add(len(paths))
	for _, extendPath := range paths {
		go func(newPath string, id uint8) {
			loadFromFile(newPath, id, columns, ch, wg)
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
