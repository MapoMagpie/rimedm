package dict

import (
	"log"
	"os"
	"sort"
	"strings"
	"sync"
)

func output(entries []*Entry, fes []*FileEntries) {
	fMap := make(map[string][]*Entry)
	for _, entry := range entries {
		if entry.modType == NC {
			continue
		}
		entries := fMap[entry.refFile]
		fMap[entry.refFile] = append(entries, entry)
	}
	var wg sync.WaitGroup
	wg.Add(len(fMap))
	for fileName, entries := range fMap {
		if entries == nil || len(entries) == 0 {
			continue
		}
		var fe *FileEntries
		for _, f := range fes {
			if fe = f; f.FilePath == fileName {
				break
			}
		}
		go func(buf []byte, path string, entries []*Entry) {
			defer wg.Done()
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].seek < entries[j].seek
			})
			outputFile(buf, path, entries)
		}(fe.RawBs, fe.FilePath, entries)
	}
	wg.Wait()
}

func tryFatalf(err error, format string, args ...interface{}) {
	if err != nil {
		log.Fatalf(format, args...)
	}
}
func outputFile(rawBs []byte, path string, entries []*Entry) {
	//log.Printf("rawBs now len %d\n", len(rawBs))
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	tryFatalf(err, "open File failed, Err:%v", err)
	defer func() {
		_ = file.Close()
	}()
	bs := make([]byte, len(rawBs))
	copy(bs, rawBs)
	willAddEntries := make([]*Entry, 0)
	seekFixed := int64(0)
	for _, entry := range entries {
		var modType string
		switch entry.modType {
		case NC:
			continue
		case DELETE:
			bs = append(bs[:entry.seek+seekFixed], bs[entry.seek+seekFixed+entry.rawSize:]...)
			seekFixed = seekFixed - entry.rawSize
			modType = "DELETE"
		case MODIFY:
			nbs := []byte(strings.TrimSpace(strings.Join(entry.Pair, "\t")) + "\n")
			bs = append(bs[:entry.seek+seekFixed], append(nbs, bs[entry.seek+seekFixed+entry.rawSize:]...)...)
			seekFixed = seekFixed - entry.rawSize + int64(len(nbs))
			modType = "MODIFY"
		case ADD:
			willAddEntries = append(willAddEntries, entry)
			modType = "ADD"
		}
		log.Printf("modify dict type:%s | %s", modType, entry.String())
	}
	if len(willAddEntries) > 0 {
		if bs[len(bs)-1] != '\n' {
			bs = append(bs, '\n')
		}
		for _, entry := range willAddEntries {
			nbs := []byte(strings.TrimSpace(strings.Join(entry.Pair, "\t")) + "\n")
			bs = append(bs, nbs...)
		}
	}
	l, err := file.Write(bs)
	tryFatalf(err, "write File failed, Err:%v", err)
	err = file.Truncate(int64(l))
	tryFatalf(err, "truncate File failed, Err:%v", err)
	//_, err = file.Seek(0, io.SeekStart)
	//tryFatalf(err, "seek File failed, Err:%v", err)
	//err = file.Sync()
	//tryFatalf(err, "sync File failed, Err:%v", err)
}
