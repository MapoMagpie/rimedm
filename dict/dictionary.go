package dict

import (
	"context"
	"errors"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/junegunn/fzf/src/util"
)

type Dictionary struct {
	matcher     Matcher
	entries     []*Entry
	fileEntries []*FileEntries
}

func NewDictionary(fes []*FileEntries, matcher Matcher) *Dictionary {
	if matcher == nil {
		matcher = &CacheMatcher{}
	}
	entries := make([]*Entry, 0)
	for _, fe := range fes {
		entries = append(entries, fe.Entries...)
	}
	return &Dictionary{
		matcher:     matcher,
		entries:     entries,
		fileEntries: fes,
	}
}

func (d *Dictionary) Entries() []*Entry {
	return d.entries
}

func (d *Dictionary) Search(key []rune, resultChan chan<- []*MatchResult, ctx context.Context) {
	log.Println("search key: ", string(key))
	if len(key) == 0 {
		done := false
		go func() {
			<-ctx.Done()
			done = true
		}()
		list := d.Entries()
		deleteCount := 0
		ret := make([]*MatchResult, len(list))
		for i, entry := range list {
			if done {
				return
			}
			if entry.IsDelete() {
				deleteCount += 1
				continue
			}
			ret[i-deleteCount] = &MatchResult{Entry: entry}
		}
		resultChan <- ret[0 : len(ret)-deleteCount]
	} else {
		d.matcher.Search(key, d.Entries(), resultChan, ctx)
	}
}

func (d *Dictionary) Add(entry *Entry) {
	for _, fe := range d.fileEntries {
		if fe.ID == entry.FID {
			fe.Entries = append(fe.Entries, entry)
		}
	}
	d.entries = append(d.entries, entry)
}

func (d *Dictionary) Delete(entry *Entry) {
	entry.Delete()
}

func (d *Dictionary) ResetMatcher() {
	d.matcher.Reset()
}

func (d *Dictionary) Len() int {
	return len(d.entries)
}

func (d *Dictionary) Flush() (changed bool) {
	start := time.Now()
	changed = output(d.fileEntries)
	since := time.Since(start)
	if changed {
		log.Printf("flush dictionary: %v\n", since)
	}
	return changed
}

func (d *Dictionary) ExportDict(path string) {
	exportDict(path, d.fileEntries)
}

type ModifyType int

const (
	NC ModifyType = iota // default no change
	DELETE
	MODIFY // by ReRaw
	ADD    // by NewEntryAdd
)

type Entry struct {
	FID     uint8
	seek    int64
	rawSize int64
	modType ModifyType
	raw     []byte
	deleted bool
}

func (e *Entry) Data() *Data {
	pair, cols := ParseInput(string(e.raw))
	data, _ := ParseData(pair, cols)
	return data
}

func (e *Entry) ReRaw(raw []byte) {
	e.raw = raw
	if e.modType != ADD {
		e.modType = MODIFY
	}
	// don't change rawSize
}

func (e *Entry) reSeek(seek int64, rawSize int64) {
	e.seek = seek
	e.rawSize = rawSize
}

// for fzf match
func (e *Entry) Chars() *util.Chars {
	// TODO: store chars
	chars := util.ToChars(e.raw)
	return &chars
}

func (e *Entry) Delete() {
	e.modType = DELETE
	e.deleted = true
}

func (e *Entry) IsDelete() bool {
	return e.deleted
}

func (e *Entry) Raw() []byte {
	// return e.text.ToString() + "\t" + e.refFile
	return e.raw
}

func (e *Entry) Saved() {
	e.rawSize = int64(len(e.raw)) + 1 // + 1 for '\n'
	e.modType = NC
}

// Parse input string to a pair of strings
// 支持乱序输入，如 "你好 nau 1" 或 "nau 1 你好"
func ParseInput(raw string) ([]string, []Column) {
	pair := make([]string, 0)
	cols := make([]Column, 0)
	// split by '\t' or ' '
	splits := strings.Fields(raw)
	textIndex := -1
	for i := 0; i < len(splits); i++ {
		split := strings.TrimSpace(splits[i])
		if len(split) == 0 {
			continue
		}
		if isNumber(split) {
			cols = append(cols, COLUMN_WEIGHT)
			pair = append(pair, split)
			continue
		}
		if isAscii(split) {
			stemIndex := slices.Index(cols, COLUMN_STEM)
			codeIndex := slices.Index(cols, COLUMN_CODE)
			if codeIndex == -1 {
				cols = append(cols, COLUMN_CODE)
				pair = append(pair, split)
			} else {
				if stemIndex == -1 {
					cols = append(cols, COLUMN_STEM)
					pair = append(pair, split)
				} else {
					pair[stemIndex] = pair[stemIndex] + " " + split
				}
			}
			continue
		}
		// 汉字
		if textIndex == -1 {
			textIndex = i
			pair = append(pair, split)
			cols = append(cols, COLUMN_TEXT)
		} else {
			// 表(汉字)的输入可能包含空格，类似 "富强 强国"，因此在splited后重新拼接起来。
			pair[textIndex] = pair[textIndex] + " " + split
		}
	}
	return pair, cols
}

func isNumber(str string) bool {
	for _, r := range str {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isAscii(str string) bool {
	for _, r := range str {
		if r >= 0x80 {
			return false
		}
	}
	return true
}

func ParseData(pair []string, columns []Column) (*Data, error) {
	if len(pair) != len(columns) {
		return nil, errors.New("raw")
	}
	var data Data
	data.cols = columns
	for i := 0; i < len(pair); i++ {
		term := pair[i]
		col := columns[i]
		switch col {
		case COLUMN_TEXT:
			data.Text = term
		case COLUMN_CODE:
			data.Code = term
		case COLUMN_WEIGHT:
			data.Weight, _ = strconv.Atoi(term)
		case COLUMN_STEM:
			data.Stem = term
		default:
			continue
		}
	}
	return &data, nil
}

func NewEntry(raw []byte, fileID uint8, seek int64, size int64) *Entry {
	return &Entry{
		FID:     fileID,
		modType: NC,
		seek:    seek,
		rawSize: size,
		raw:     raw,
	}
}

func NewEntryAdd(raw []byte, fileID uint8) *Entry {
	return &Entry{
		FID:     fileID,
		modType: ADD,
		raw:     raw,
	}
}

type Data struct {
	Text   string
	Code   string
	Stem   string
	Weight int
	cols   []Column
}

func (d *Data) ToBytes() []byte {
	return d.ToBytesWithColumns(d.cols)
}

func (d *Data) ToBytesWithColumns(cols []Column) []byte {
	bs := make([]byte, 0)
	for _, col := range cols {
		var b []byte
		switch col {
		case COLUMN_TEXT:
			b = []byte(d.Text)
		case COLUMN_WEIGHT:
			b = []byte(strconv.Itoa(d.Weight))
		case COLUMN_CODE:
			b = []byte(d.Code)
		case COLUMN_STEM:
			b = []byte(d.Stem)
		}
		if len(bs) > 0 {
			bs = append(bs, '\t')
		}
		bs = append(bs, b...)
	}
	return bs
}

type Column uint8

const (
	COLUMN_TEXT   Column = 0
	COLUMN_CODE   Column = 1
	COLUMN_WEIGHT Column = 2
	COLUMN_STEM   Column = 3
)

var DEFAULT_COLUMNS = []Column{COLUMN_TEXT, COLUMN_WEIGHT, COLUMN_CODE, COLUMN_STEM}
