package dict

import (
	"bytes"
	"github.com/junegunn/fzf/src/util"
	"log"
	"time"
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

func (d *Dictionary) Search(key []rune) []*Entry {
	if len(key) == 0 {
		return d.Entries()
	}
	return d.matcher.Search(key, d.Entries())
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
	Pair    [][]byte // 0 汉字 1 code 3 权重
	refFile string
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
	//return e.text.ToString() + "\t" + e.refFile
	return e.text.ToString()
}

func (e *Entry) Logged() {
	e.log = false
}

func (e *Entry) WriteLine() []byte {
	bs := make([]byte, 0)
	for i := 0; i < len(e.Pair); i++ {
		bs = append(bs, e.Pair[i]...)
		if i <= len(e.Pair)-1 {
			bs = append(bs, '\t')
		}
	}
	return bs
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

func ParsePair(raw []byte) [][]byte {
	pair := make([][]byte, 0)
	for i, j := 0, 0; i < len(raw); i++ {
		if raw[i] == '\t' {
			pair = append(pair, bytes.TrimSpace(raw[j:i]))
			j = i + 1
		}
		if i == len(raw)-1 && j < i {
			pair = append(pair, bytes.TrimSpace(raw[j:]))
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
