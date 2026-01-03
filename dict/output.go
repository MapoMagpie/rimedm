package dict

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
)

func isExtendedCJK(text string) bool {
	chars := []rune(text)
	if len(chars) == 1 {
		ch := chars[0]
		return ((ch >= 0x3400 && ch <= 0x4DBF) || // CJK Unified Ideographs Extension A
			(ch >= 0x20000 && ch <= 0x2A6DF) || // CJK Unified Ideographs Extension B
			(ch >= 0x2A700 && ch <= 0x2B73F) || // CJK Unified Ideographs Extension C
			(ch >= 0x2B740 && ch <= 0x2B81F) || // CJK Unified Ideographs Extension D
			(ch >= 0x2B820 && ch <= 0x2CEAF) || // CJK Unified Ideographs Extension E
			(ch >= 0x2CEB0 && ch <= 0x2EBEF) || // CJK Unified Ideographs Extension F
			(ch >= 0x30000 && ch <= 0x3134F) || // CJK Unified Ideographs Extension G
			(ch >= 0x31350 && ch <= 0x323AF) || // CJK Unified Ideographs Extension H
			(ch >= 0x2EBF0 && ch <= 0x2EE5F) || // CJK Unified Ideographs Extension I
			(ch >= 0x323B0 && ch <= 0x3347F) || // CJK Unified Ideographs Extension J
			(ch >= 0x3300 && ch <= 0x33FF) || // CJK Compatibility
			(ch >= 0xFE30 && ch <= 0xFE4F) || // CJK Compatibility Forms
			(ch >= 0xF900 && ch <= 0xFAFF) || // CJK Compatibility Ideographs
			(ch >= 0x2F800 && ch <= 0x2FA1F)) // CJK Compatibility Ideographs Supplement
	}
	return false
}

func exportDict(path string, fes []*FileEntries, cols []Column) {
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = file.Close()
	}()
	for _, fe := range fes {
		if len(fe.Entries) == 0 {
			continue
		}
		log.Println("导出词库中:", fe.FilePath)
		for _, entry := range fe.Entries {
			if isExtendedCJK(entry.data.Text) {
				continue
			}
			_, err := file.WriteString(entry.data.ToStringWithColumns(&cols))
			if err != nil {
				panic(err)
			}
			_, err = file.WriteString("\n")
			if err != nil {
				panic(err)
			}
		}
	}
}

func output(fes []*FileEntries) (changed bool) {
	var wg sync.WaitGroup
	for _, fe := range fes {
		if len(fe.Entries) == 0 {
			continue
		}
		wg.Add(1)
		go func(fe *FileEntries) {
			defer wg.Done()
			sort.Slice(fe.Entries, func(i, j int) bool {
				return fe.Entries[i].seek < fe.Entries[j].seek
			})
			if outputFile(&fe.RawBs, fe.FilePath, fe.Entries) {
				changed = true
			}
		}(fe)
	}
	wg.Wait()
	return changed
}

func tryPanic(err error, format string, args ...any) {
	if err != nil {
		panic(fmt.Sprintf(format, args...))
	}
}

func outputFile(rawBs *[]byte, path string, entries []*Entry) (changed bool) {
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	tryPanic(err, "open File failed, Err:%v", err)
	defer func() { _ = file.Close() }()
	bs := *rawBs
	willAddEntries := make([]*Entry, 0)
	seekFixed := int64(0)
	for _, entry := range entries {
		entry.seek += seekFixed
		if entry.modType == NC {
			continue
		}
		var modType string
		switch entry.modType {
		case DELETE:
			bs = append(bs[:entry.seek], bs[entry.seek+entry.rawSize:]...)
			seekFixed = seekFixed - entry.rawSize
			entry.Saved()
			modType = "DEL"
		case MODIFY:
			nbs := []byte(entry.Raw())
			nbs = append(nbs, '\n')
			bs = append(bs[:entry.seek], append(nbs, bs[entry.seek+entry.rawSize:]...)...)
			seekFixed = seekFixed - entry.rawSize + int64(len(nbs))
			entry.Saved()
			modType = "MOD"
		case ADD:
			willAddEntries = append(willAddEntries, entry)
			modType = "ADD"
		}
		log.Printf("modify dict type:%s | %s", modType, entry.Raw())
		changed = true
	}
	if !changed {
		return
	}
	seek := int64(len(bs))
	// append new entry to file
	if len(willAddEntries) > 0 {
		if bs[len(bs)-1] != '\n' {
			bs = append(bs, '\n')
			seek += 1
		}
		for _, entry := range willAddEntries {
			raw := entry.Raw()
			rawSize := int64(len(raw) + 1)
			bs = append(bs, raw...)
			bs = append(bs, '\n')
			entry.reSeek(seek, rawSize)
			entry.Saved()
			seek += entry.rawSize
		}
	}
	*rawBs = bs
	l, err := file.Write(bs)
	tryPanic(err, "write File failed, Err:%v", err)
	err = file.Truncate(int64(l))
	tryPanic(err, "truncate File failed, Err:%v", err)
	return
}
