package dict

import (
	"fmt"
	"os"
	"testing"
)

func Test_outputFile(t *testing.T) {
	_ = os.MkdirAll("./tmp", os.ModePerm)
	defer os.RemoveAll("./tmp")
	content1 := `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzzmod
早早	zzzz
测试	ceek
`
	content2 := `
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzzmod
早早	zzzz
测试	ceek
`
	content3 := `
早早	zzzzmod
早早	zzzz
测试	ceek
`
	content1_want1 := `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzz
测试	ceek
`

	content1_want2 := `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzz
`
	content1_want3 := `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zaozao
测试	ceshi
`
	content2_want1 := `
name: xkjd6.whatever
version: "Q1"
sort: original
...
早早	zzzz
`
	content3_want1 := `
早早	zzzz
`
	content4 := `
name: xkjd6.whatever
version: "Q1"
sort: original
columns:
  - text
  - code
  - weight
...
早早	zzzzmod	10
早早	zzzz	10
测试	ceek	10
`
	content4_want1 := `
name: xkjd6.whatever
version: "Q1"
sort: original
columns:
  - text
  - code
  - weight
...
早早	zzzzmod	10
早早	zzzz	10
测试	ceek	10
伊藤萌子	jllh	10
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
		{
			name: "content2",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile6.yaml", content2)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				fe.Entries[2].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				return fe
			}(),
			want:          content2_want1,
			shouldChanged: false,
			filename:      "./tmp/test_outputfile6.yaml",
		},
		{
			name: "content3",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile7.yaml", content3)
				fe := LoadItems(filename)[0]
				fe.Entries[0].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				fe.Entries[2].Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				fmt.Println("---------------")
				return fe
			}(),
			want:          content3_want1,
			shouldChanged: false,
			filename:      "./tmp/test_outputfile7.yaml",
		},
		{
			name: "add and modify",
			fe: func() *FileEntries {
				filename := createFile("./tmp/test_outputfile8.yaml", content4)
				fe := LoadItems(filename)[0]

				// new entry then just delete
				ne0 := NewEntryAdd([]byte("萌子	lohi	1"), 0)
				fe.Entries = append(fe.Entries, ne0)
				// outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				ne0.Delete()

				ne1 := NewEntryAdd([]byte("萌子	lohi	1"), 0)
				fe.Entries = append(fe.Entries, ne1)
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				ne1.Delete()
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)

				ne2 := NewEntryAdd([]byte("伊藤	jblv	1"), 0)
				fe.Entries = append(fe.Entries, ne2)
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				ne2.ReRaw([]byte("伊藤	jblv	10"))
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				ne2.ReRaw([]byte("伊藤萌子	jllh	10"))
				outputFile(&fe.RawBs, fe.FilePath, fe.Entries)
				return fe
			}(),
			want:          content4_want1,
			shouldChanged: false,
			filename:      "./tmp/test_outputfile8.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			changed := outputFile(&tt.fe.RawBs, tt.fe.FilePath, tt.fe.Entries)
			c, err := os.ReadFile(tt.filename)
			if err != nil {
				panic(err)
			}
			if string(c) != string(tt.fe.RawBs) {
				t.Errorf("case:%v file content != file\nfile\n%v[fin]\nRawBs\n%v[fin]", tt.name, string(c), string(tt.fe.RawBs))
			}
			if string(tt.fe.RawBs) != tt.want {
				t.Errorf("case:%v RawBs != want\nfe.RawBs\n%v[fin]\nwant\n%v[fin]", tt.name, string(tt.fe.RawBs), tt.want)
			}
			if tt.shouldChanged != changed {
				t.Errorf("case:%v, changed: %v, want: %v", tt.name, changed, tt.shouldChanged)
			}
		})
	}
}
