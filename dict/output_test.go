package dict

import (
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
				path := "../rime/xkjd/xkjd6.user.dict.yaml"
				fes := LoadItems(path)
				fe := fes[0]
				entries := fe.Entries[:]
				entries[0].Delete()
				entries[1].ReRaw(append(entries[1].WriteLine(), []byte{'m', 'o', 'd'}...))
				entries = append(entries, NewEntryAdd([]byte("测试\tceek"), path))
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
