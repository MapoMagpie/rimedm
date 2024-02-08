package dict

import (
	"bytes"
	"context"
	"log"
	"strings"
	"time"

	"github.com/junegunn/fzf/src/util"
)

type Dictionary struct {
	matcher     Matcher
	fileEntries []*FileEntries
}

func NewDictionary(fes []*FileEntries, matcher Matcher) *Dictionary {
	if matcher == nil {
		matcher = &CacheMatcher{}
	}
	return &Dictionary{
		fileEntries: fes,
		matcher:     matcher,
	}
}

func (d *Dictionary) Entries() []*Entry {
	entries := make([]*Entry, 0)
	for _, fe := range d.fileEntries {
		entries = append(entries, fe.Entries...)
	}
	return entries
}

func (d *Dictionary) Search(key []rune, resultChan chan<- []*MatchResult, ctx context.Context) {
	if len(key) == 0 {
		done := false
		go func() {
			<-ctx.Done()
			done = true
		}()
		list := d.Entries()
		ret := make([]*MatchResult, len(list))
		for i, entry := range list {
			if done {
				return
			}
			ret[i] = &MatchResult{Entry: entry}
		}
		resultChan <- ret
	} else {
		d.matcher.Search(key, d.Entries(), resultChan, ctx)
	}
}

func (d *Dictionary) Add(entry *Entry) {
	for _, fe := range d.fileEntries {
		if fe.FilePath == entry.refFile {
			fe.Entries = append(fe.Entries, entry)
		}
	}
}

func (d *Dictionary) Delete(entry *Entry) {
	entry.Delete()
}

func (d *Dictionary) ResetMatcher() {
	d.matcher.Reset()
}

func (d *Dictionary) Len() int {
	le := 0
	for _, fe := range d.fileEntries {
		le = le + len(fe.Entries)
	}
	return le
}

func (d *Dictionary) Flush() {
	start := time.Now()
	output(d.fileEntries)
	since := time.Since(start)
	log.Printf("flush dictionary: %v\n", since)
}

func (d *Dictionary) ExportDict(path string) {
	exportDict(path, d.fileEntries)
}

func (d *Dictionary) Files() []*FileEntries {
	return d.fileEntries
}

type ModifyType int

const (
	NC ModifyType = iota // default no change
	DELETE
	MODIFY // by ReRaw
	ADD    // by NewEntryAdd
)

type Entry struct {
	refFile string
	Pair    [][]byte
	text    util.Chars
	seek    int64
	rawSize int64
	modType ModifyType
	log     bool
}

func (e *Entry) ReRaw(raw []byte) {
	e.text = util.ToChars(raw)
	e.Pair = ParsePair(raw)
	if e.modType != ADD {
		e.modType = MODIFY
	}
	e.log = true
}

func (e *Entry) Delete() {
	e.modType = DELETE
	e.log = true
}

func (e *Entry) IsDelete() bool {
	return e.modType == DELETE
}

func (e *Entry) String() string {
	// return e.text.ToString() + "\t" + e.refFile
	return e.text.ToString()
}

func (e *Entry) Logged() {
	e.log = false
}

func (e *Entry) WriteLine() []byte {
	bs := make([]byte, 0)
	for i := 0; i < len(e.Pair); i++ {
		if len(bytes.TrimSpace(e.Pair[i])) == 0 {
			continue
		}
		bs = append(bs, e.Pair[i]...)
		if i < len(e.Pair)-1 {
			bs = append(bs, '\t')
		}
	}
	return bs
}

// Parse input string to a pair of strings
// 0: 表(汉字) 1: 码(字母) 2: 权重
// 支持乱序输入，如 "你好 nau 1" 或 "nau 1 你好"
func ParseInput(raw string) (pair [3]string) {
	pair = [3]string{}
	// split by '\t' or ' '
	splits := strings.Fields(raw)
	for i := 0; i < len(splits); i++ {
		item := strings.TrimSpace(splits[i])
		if len(item) == 0 {
			continue
		}
		if isNumber(item) {
			pair[2] = item
			continue
		}
		if isAscii(item) {
			pair[1] = item
		} else {
			space := " "
			if pair[0] == "" {
				space = ""
			}
			pair[0] = pair[0] + space + item
		}
	}
	return
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

// Parse bytes as a couple of strings([]byte) separated by '\t'
// e.g. "你好	nau" > ["你好", "nau"]
// not like ParseInput, this function simply split by '\t'
func ParsePair(raw []byte) [][]byte {
	pair := make([][]byte, 0)
	for i, j := 0, 0; i < len(raw); i++ {
		if raw[i] == '\t' {
			item := bytes.TrimSpace(raw[j:i])
			if len(item) > 0 {
				pair = append(pair, item)
			}
			j = i + 1
		}
		if i == len(raw)-1 && j <= i {
			item := bytes.TrimSpace(raw[j:])
			if len(item) > 0 {
				pair = append(pair, item)
			}
		}
	}
	return pair
}

func NewEntry(raw []byte, refFile string, seek int64, size int64) *Entry {
	return &Entry{
		text:    util.ToChars(raw),
		Pair:    ParsePair(raw),
		refFile: refFile,
		seek:    seek,
		rawSize: size,
	}
}

func NewEntryAdd(raw []byte, refFile string) *Entry {
	return &Entry{
		text:    util.ToChars(raw),
		Pair:    ParsePair(raw),
		refFile: refFile,
		modType: ADD,
		log:     true,
	}
}
