package dict

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func Test_LoadItems(t *testing.T) {
	filename := mockFile()
	defer os.RemoveAll("./tmp")
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"1", args{filename}, 19},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			start := time.Now()
			fes := LoadItems(tt.args.path)
			duration1 := time.Since(start)
			fmt.Println("======================================================")
			entries := make([]*Entry, 0)
			if len(fes) > 0 {
				for _, fe := range fes {
					entries = append(entries, fe.Entries...)
				}
			}
			fmt.Println("count >>", len(entries))
			if len(entries) != tt.want {
				t.Errorf("Load Item Count = %v, want %v", len(entries), tt.want)
			}
			fmt.Println("======================================================")
			fmt.Println("load duration >>", duration1)
		})
	}
}

func mockFile() string {
	// create ./tmp directory
	err := os.MkdirAll("./tmp", os.ModePerm)
	if err != nil {
		fmt.Println("mkdir error, ", err)
		panic(err)
	}
	content := `
# 键道6 扩展词库控制
---
name: xkjd6
version: "Q1"
sort: original 
use_preset_vocabulary: false
import_tables:
# 扩展：单字
  - rime.danzi
# 扩展：词组
  - rime.cizu
# 扩展：符号
  - rime.fuhao
`
	filename := createFile("./tmp/rime.dict.yaml", content)
	content = `
---
name: xkjd6.danzi
version: "Q1"
sort: original
...
不	b
宾	bb
滨	bba
  `
	createFile("./tmp/rime.danzi.dict.yaml", content)
	content = `
---
name: xkjd6.cizu
version: "Q1"
sort: original
import_tables:
# 扩展：单字
  - rime.cizu2
...
并不比	bbb
彬彬	bbbb
斌斌	bbbbo
  `
	createFile("./tmp/rime.cizu.dict.yaml", content)
	content = `
①	oyk
②	oxj
③	osf
④	osk
⑤	owj
⑥	olq
⑦	oqk
⑧	obs
⑨	ojq
⑩	oek
  `
	createFile("./tmp/rime.fuhao.dict.yaml", content)
	content = `
---
name: xkjd6.whatever
version: "Q1"
sort: original
...
造作	zzzl
早做	zzzlo
早早	zzzz
`
	createFile("./tmp/rime.cizu2.dict.yaml", content)
	return filename
}

func createFile(name string, content string) string {
	file, err := os.Create(name)
	if err != nil {
		fmt.Println("create temp file error, ", err)
		panic(err)
	}
	defer file.Close()
	file.WriteString(content)
	return file.Name()
}
