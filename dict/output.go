package dict

import (
	"log"
	"os"
	"sort"
	"sync"
)

func exportDict(path string, fes []*FileEntries) {
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
		for _, entry := range fe.Entries {
			bs := entry.WriteLine()
			bs = append(bs, '\n')
			_, err := file.Write(bs)
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

func tryFatalf(err error, format string, args ...interface{}) {
	if err != nil {
		log.Fatalf(format, args...)
	}
}

func outputFile(rawBs *[]byte, path string, entries []*Entry) (changed bool) {
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	tryFatalf(err, "open File failed, Err:%v", err)
	defer file.Close()
	bs := *rawBs
	willAddEntries := make([]*Entry, 0)
	seekFixed := int64(0)
	for _, entry := range entries {
		entry.seek += seekFixed
		if entry.saved || entry.modType == NC {
			continue
		}
		var modType string
		switch entry.modType {
		case DELETE:
			bs = append(bs[:entry.seek], bs[entry.seek+entry.rawSize:]...)
			seekFixed = seekFixed - entry.rawSize
			modType = "DELETE"
		case MODIFY:
			nbs := entry.WriteLine()
			nbs = append(nbs, '\n')
			bs = append(bs[:entry.seek], append(nbs, bs[entry.seek+entry.rawSize:]...)...)
			seekFixed = seekFixed - entry.rawSize + int64(len(nbs))
			modType = "MODIFY"
		case ADD:
			willAddEntries = append(willAddEntries, entry)
			modType = "ADD"
		}
		log.Printf("modify dict type:%s | %s", modType, entry.String())
		changed = true
		entry.Saved()
	}
	if !changed {
		return
	}
	if len(willAddEntries) > 0 {
		if bs[len(bs)-1] != '\n' {
			bs = append(bs, '\n')
		}
		for _, entry := range willAddEntries {
			bs = append(bs, entry.WriteLine()...)
			bs = append(bs, '\n')
		}
	}
	*rawBs = bs
	l, err := file.Write(bs)
	tryFatalf(err, "write File failed, Err:%v", err)
	err = file.Truncate(int64(l))
	tryFatalf(err, "truncate File failed, Err:%v", err)
	return
}
