package dict

import (
	"log"
	"os"
	"strings"
	"testing"
)

func Test_outputFile(t *testing.T) {
	type args struct {
		fe      *FileEntries
		entries []*Entry
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "1",
			args: func() args {
				path := "../rime/xkjd6.user.dict.yaml"
				fes := LoadItems(path)
				fe := fes[0]
				entries := fe.Entries[:]
				entries[0].Delete()
				entries[1].ReRaw(entries[1].text.ToString() + "mod")
				entries = append(entries, NewEntryAdd("测试\tceek", path))
				return args{fe, entries}
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputFile(tt.args.fe.RawBs, tt.args.fe.FilePath, tt.args.entries)
		})
	}
}

func Test_outputDictTxt(t *testing.T) {
	t.Run("1", func(t *testing.T) {
		path := "/home/kamo-death/.local/share/fcitx5/rime/xkjd6.dict.yaml"
		fes := LoadItems(path)
		entries := make([]*Entry, 0)
		for _, fe := range fes {
			if strings.Index(fe.FilePath, "fuhao") != -1 {
				continue
			}
			entries = append(entries, fe.Entries...)
		}
		file, err := os.Create("../dict.txt")
		if err != nil {
			log.Fatalf("create dict.txt failed, Err:%v", err)
		}
		defer func() {
			_ = file.Close()
		}()
		for _, entry := range entries {
			_, _ = file.WriteString(entry.String() + "\n")
		}
	})
}
