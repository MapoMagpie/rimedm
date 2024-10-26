package dict

import (
	"bytes"
	"fmt"
	"os"
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

func Test_outputFile(t *testing.T) {
	os.MkdirAll("./tmp", os.ModePerm)
	defer os.RemoveAll("./tmp")
	content1 := `
---
name: xkjd6.whatever
...
早早	zzzzmod
早早	zzzz
测试	ceek
  `
	content1_want1 := `
---
name: xkjd6.whatever
...
早早	zzzz
测试	ceek
  `

	content1_want2 := `
---
name: xkjd6.whatever
...
早早	zzzz
  `
	content1_want3 := `
---
name: xkjd6.whatever
...
早早	zaozao
测试	ceshi
  `
	tests := []struct {
		fe            *FileEntries
		name          string
		want          string
		filename      string
		shouldChanged bool
	}{
		{
			name: "delete some 1",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile1.yaml", content1)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				return fe
			}(),
			want:          content1_want1,
			shouldChanged: true,
			filename:      "./tmp/test_outputfile1.yaml",
		},
		{
			name: "delete some 2",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile2.yaml", content1)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fe.Entries[2].Delete()
				return fe
			}(),
			want:          content1_want2,
			shouldChanged: true,
			filename:      "./tmp/test_outputfile2.yaml",
		},
		{
			name: "delete 1 mod 2",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile3.yaml", content1)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				fe.Entries[1].ReRaw([]byte("早早\tzaozao"))
				fe.Entries[2].ReRaw([]byte("测试\tceshi"))
				return fe
			}(),
			want:          content1_want3,
			shouldChanged: true,
			filename:      "./tmp/test_outputfile3.yaml",
		},
		{
			name: "delete 1 mod 2 output multiple times",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile4.yaml", content1)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				fe.Entries[1].ReRaw([]byte("早早\tzaozao"))
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				fe.Entries[2].ReRaw([]byte("测试\tceshi"))
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				return fe
			}(),
			want:          content1_want3,
			shouldChanged: false,
			filename:      "./tmp/test_outputfile4.yaml",
		},
		{
			name: "delete and output multiple times",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile5.yaml", content1)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				fe.Entries[2].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				return fe
			}(),
			want:          content1_want2,
			shouldChanged: false,
			filename:      "./tmp/test_outputfile5.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			changed := outputFile(&tt.fe.RawBs, tt.fe.FilePath, tt.fe.Entries)
			c, err := os.ReadFile(tt.filename)
			if err != nil {
				panic(err)
			}
			if string(c) != tt.want {
				t.Errorf("case:%v\n%v\nwant\n%v", tt.name, string(c), tt.want)
			}
			if string(tt.fe.RawBs) != tt.want {
				t.Errorf("case:%v\nfe.RawBs\n%v\nwant\n%v", tt.name, string(tt.fe.RawBs), tt.want)
			}
			if tt.shouldChanged != changed {
				t.Errorf("case:%v, changed: %v, want: %v", tt.name, changed, tt.shouldChanged)
			}
		})
	}
}
