package dict

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func Test_Entry_WriteLine(t *testing.T) {
	tests := []struct {
		name  string
		want  []byte
		entry Entry
	}{
		{
			name:  "1",
			entry: *NewEntryAdd([]byte("测试\tceek"), ""),
			want:  []byte("测试\tceek"),
		},
		{
			name:  "2",
			entry: *NewEntryAdd([]byte("测试\tc\t1"), ""),
			want:  []byte("测试\tc\t1"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.WriteLine(); !bytes.Equal(got, tt.want) {
				t.Errorf("writeLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_output(t *testing.T) {
	filename := mockFile()
	defer os.RemoveAll("./tmp")
	fes := LoadItems(filename)
	content := `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzzmod
早早	zzzz
测试	ceek
`
	type args struct {
		fe *FileEntries
	}
	tests := []struct {
		args args
		name string
		want string
	}{
		{
			name: "case1",
			args: func() args {
				var fe *FileEntries
				for _, f := range fes {
					if strings.Contains(f.FilePath, "rime.cizu2.dict.yaml") {
						fe = f
					}
				}
				if fe == nil {
					panic("file not found: rime.cizu2.dict.yaml")
				}
				fe.Entries[0].Delete()
				fe.Entries[1].ReRaw(append(fe.Entries[2].WriteLine(), []byte{'m', 'o', 'd'}...))
				fe.Entries = append(fe.Entries, NewEntryAdd([]byte("测试\tceek"), fe.FilePath))
				return args{fe}
			}(),
			want: content,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			output([]*FileEntries{tt.args.fe})
			c, err := os.ReadFile("./tmp/rime.cizu2.dict.yaml")
			if err != nil {
				panic(err)
			}
			if string(c) != tt.want {
				t.Errorf("output() = %v, want %v", string(c), tt.want)
			}
		})
	}
}
