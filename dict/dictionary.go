package dict

import (
	"github.com/junegunn/fzf/src/util"
	"log"
	"sort"
	"time"
)

type Dictionary struct {
	entries     []*Entry
	matcher     Matcher
	fileEntries []*FileEntries
}

func NewDictionary(fes []*FileEntries, matcher Matcher) *Dictionary {
	if matcher == nil {
		matcher = &CacheMatcher{}
	}
	entries := make([]*Entry, 0)
	if len(fes) > 0 {
		for _, fe := range fes {
			entries = append(entries, fe.Entries...)
		}
	}
	start := time.Now()
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].refFile < entries[j].refFile
	})
	since := time.Since(start)
	log.Printf("sort entries: %v\n", since)
	return &Dictionary{
		entries:     entries,
		fileEntries: fes,
		matcher:     matcher,
	}
}

func (d *Dictionary) Search(key []rune) []*Entry {
	if len(key) == 0 {
		return d.entries
	}
	return d.matcher.Search(key, d.entries)
}

func (d *Dictionary) Add(entry *Entry) {
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

func (d *Dictionary) Flush() {
	start := time.Now()
	output(d.entries, d.fileEntries)
	since := time.Since(start)
	log.Printf("flush dictionary: %v\n", since)
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
	text    util.Chars
	Pair    []string // 0 汉字 1 code 3 权重
	refFile string
	seek    int64
	rawSize int64
	modType ModifyType
}

func (e *Entry) ReRaw(raw string) {
	e.text = util.ToChars([]byte(raw))
	e.Pair = ParsePair(raw)
	if e.modType != ADD {
		e.modType = MODIFY
	}
}

func (e *Entry) Delete() {
	e.modType = DELETE
}

func (e *Entry) IsDelete() bool {
	return e.modType == DELETE
}

func (e *Entry) String() string {
	return e.text.ToString()
}

// ParseInput \n
// "你\t好" > "你", "好", ""
// "你 好" > "你", "好", ""
// "你  好" > "你", "好", ""
// "你\t 好" > "你", "好", ""
// "你   好\t 1" > "你", "好", "1"
// "你好 nau 1" > "你好", "nau", "1"
// "nau 你好 1" > "你好", "nau", "1"
// "  nau 你好 1 " > "你好", "nau", "1"
func ParseInput(raw string) [3]string {
	pair := [3]string{}
	for j, l, i := 0, 0, 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == '\t' || raw[i] == ' ' {
			if l == i {
				l = i + 1
				continue
			}
			pair[j] = raw[l:i]
			l = i + 1
			j++
		}
	}
	notAsciiIndex := 0
	for i, p := range pair {
		if !isAscii(p) {
			notAsciiIndex = i
			break
		}
	}
	if notAsciiIndex != 0 {
		t := pair[notAsciiIndex]
		pair[notAsciiIndex] = pair[0]
		pair[0] = t
	}
	return pair
}

func isAscii(str string) bool {
	for _, r := range []rune(str) {
		if r >= 0x80 {
			return false
		}
	}
	return true
}

func ParsePair(raw string) []string {
	defer func(raw string) {
		if p := recover(); p != nil {
			log.Fatalf("split raw [%s] panic: %v\n", raw, p)
		}
	}(raw)
	pair := make([]string, 3)
	for j, l, i := 0, 0, 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == '\t' {
			if l == i {
				l = i + 1
				continue
			}
			pair[j] = raw[l:i]
			l = i + 1
			j++
		}
	}
	return pair
}

func NewEntry(raw string, refFile string, seek int64, size int64) *Entry {
	return &Entry{
		text:    util.ToChars([]byte(raw)),
		Pair:    ParsePair(raw),
		refFile: refFile,
		seek:    seek,
		rawSize: size,
	}
}

func NewEntryAdd(raw string, refFile string) *Entry {
	return &Entry{
		text:    util.ToChars([]byte(raw)),
		Pair:    ParsePair(raw),
		refFile: refFile,
		modType: ADD,
	}
}
