package dict

import (
	"log"
	"os"
	"sort"
	"sync"
)

func exportDict(path string, fes []*FileEntries) {
	// sort fes by file name
	sort.Slice(fes, func(i, j int) bool {
		return fes[i].FilePath < fes[j].FilePath
	})
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

func output(fes []*FileEntries) {
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
			outputFile(fe.RawBs, fe.FilePath, fe.Entries)
		}(fe)
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
			nbs := entry.WriteLine()
			nbs = append(nbs, '\n')
			bs = append(bs[:entry.seek+seekFixed], append(nbs, bs[entry.seek+seekFixed+entry.rawSize:]...)...)
			seekFixed = seekFixed - entry.rawSize + int64(len(nbs))
			modType = "MODIFY"
		case ADD:
			willAddEntries = append(willAddEntries, entry)
			modType = "ADD"
		}
		if entry.log {
			log.Printf("modify dict type:%s | %s", modType, entry.String())
			entry.Logged()
		}
	}
	if len(willAddEntries) > 0 {
		if bs[len(bs)-1] != '\n' {
			bs = append(bs, '\n')
		}
		for _, entry := range willAddEntries {
			nbs := entry.WriteLine()
			nbs = append(nbs, '\n')
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
