package dict

import (
	"context"
	"errors"
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MapoMagpie/rimedm/util"
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

func (d *Dictionary) Search(key string, useColumn Column, resultChan chan<- []*MatchResult, ctx context.Context) {
	// log.Println("search key: ", string(key))
	if len(key) == 0 {
		done := false
		go func() {
			<-ctx.Done()
			done = true
		}()
		list := d.Entries()
		ret := make([]*MatchResult, len(list))
		deleteCount := 0 // for ret (len = list), if skip deleted, shrink ret
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
		d.matcher.Search(key, useColumn, d.Entries(), resultChan, ctx)
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
	raw     string
	deleted bool
	data    Data
}

func (e *Entry) Data() *Data {
	return &e.data
}

func (e *Entry) ReRaw(raw string) {
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

func (e *Entry) Delete() {
	if e.deleted {
		return
	}
	e.deleted = true
	e.modType = DELETE
}

func (e *Entry) IsDelete() bool {
	return e.deleted
}

func (e *Entry) Raw() string {
	// return e.text.ToString() + "\t" + e.refFile
	return e.raw
}

func (e *Entry) Saved() {
	e.rawSize = int64(len(e.raw)) + 1 // + 1 for '\n'
	e.modType = NC
}

// Parse input string to a pair of strings
// 支持乱序输入，如 "你好 nau 1" 或 "nau 1 你好"
// 解析规则：将原始内容通过空白字符分割成单元，依次判断每个单元是否是汉字、纯数字、ascii，
// 汉字将作为text，纯数字作为weight，其他ascii根据顺序，第一个为code，其余皆为stem(造字码)
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
		if util.IsNumber(split) {
			cols = append(cols, COLUMN_WEIGHT)
			pair = append(pair, split)
			continue
		}
		if util.IsAscii(split) {
			codeIndex := slices.Index(cols, COLUMN_CODE)
			stemIndex := slices.Index(cols, COLUMN_STEM)
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
	if textIndex == -1 { // 仍旧没有汉字，将code作为text, stem作为text
		codeIndex := slices.Index(cols, COLUMN_CODE)
		stemIndex := slices.Index(cols, COLUMN_STEM)
		if codeIndex != -1 {
			cols[codeIndex] = COLUMN_TEXT
			if stemIndex != -1 {
				cols[stemIndex] = COLUMN_CODE
			}
		}

	}
	return pair, cols
}

func ParseData(pair []string, columns *[]Column) (Data, error) {
	if len(pair) != len(*columns) {
		return Data{}, errors.New("raw")
	}
	var data Data
	data.cols = columns
	for i := 0; i < len(pair); i++ {
		term := pair[i]
		col := (*columns)[i]
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
	return data, nil
}

func fastParseData(raw string, cols *[]Column) Data {
	split := strings.Split(raw, "\t")
	colsLen := len(*cols)
	data := Data{cols: cols}
	for s, c := 0, 0; s < len(split) && c < colsLen; {
		sp := split[s]
		col := (*cols)[c]
		c++
		s++
		switch col {
		case COLUMN_TEXT:
			data.Text = sp
		case COLUMN_WEIGHT:
			weight, err := strconv.Atoi(sp)
			if err != nil { // 不是weight，可能是code，跳到下一col，但重新处理当前sp
				s--
			}
			data.Weight = weight
		case COLUMN_CODE: // 如果code列缺失，则可能导致之后的weight或stem作为code，如 code: 100，暂未处理
			data.Code = sp
		case COLUMN_STEM:
			data.Stem = sp
		}
	}
	return data
}

func NewEntry(raw []byte, fileID uint8, seek int64, size int64, cols *[]Column) *Entry {
	str := string(raw)
	data := fastParseData(str, cols)
	return &Entry{
		FID:     fileID,
		modType: NC,
		seek:    seek,
		rawSize: size,
		raw:     str,
		data:    data,
	}
}

func NewEntryAdd(raw string, fileID uint8, data Data) *Entry {
	return &Entry{
		FID:     fileID,
		modType: ADD,
		raw:     raw,
		data:    data,
	}
}

type Data struct {
	Text   string
	Code   string
	Stem   string
	Weight int
	cols   *[]Column
}

func (d *Data) ToString() string {
	return d.ToStringWithColumns(d.cols)
}

func (d *Data) ResetColumns(cols *[]Column) {
	d.cols = cols
}

func (d *Data) ToStringWithColumns(cols *[]Column) string {
	sb := strings.Builder{}
	for _, col := range *cols {
		var b string
		switch col {
		case COLUMN_TEXT:
			b = d.Text
		case COLUMN_WEIGHT:
			b = strconv.Itoa(d.Weight)
		case COLUMN_CODE:
			b = d.Code
		case COLUMN_STEM:
			b = d.Stem
		}
		if sb.Len() > 0 {
			sb.WriteByte('\t')
		}
		sb.WriteString(b)
	}
	return sb.String()
}

type Column string

const (
	COLUMN_TEXT   Column = "TEXT"
	COLUMN_CODE   Column = "CODE"
	COLUMN_WEIGHT Column = "WEIGHT"
	COLUMN_STEM   Column = "STEM"
)

var DEFAULT_COLUMNS = []Column{COLUMN_TEXT, COLUMN_WEIGHT, COLUMN_CODE, COLUMN_STEM}
