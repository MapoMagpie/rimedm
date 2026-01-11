package dict

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func Test_LoadItems_xhup(t *testing.T) {
	content := `
# coding: utf-8
# 用户词库
# 与系统词条重码时居后，如想居前，请把词条放到flypy_top.txt文件内
# 
# 编码格式：字词+Tab符+编码（用户词库本身有重码则还需后面+Tab符+权重，权重大者居前，权重数字随意）
#
#    -------- 强调一下 --------
# 
#   词条和编码之间的不是空格，而是Tab符
#   按住键盘 G 键，切换到功能键盘，使用上面的Tab键
# 
#    -------------------------------
# 
# 系统次选词放在flypy_sys.txt文件内，可修改删除
# 简词补全放本文件内，不需要可删除
# 用户词库，下行开始添加，编码格式见上，部署后生效

# 全码词
即使	jiui
回忆	hvyi
华为	hxww
一边	yibm
两边	llbm
整句	vgju
按键	anjm
单元	djyr
反思	fjsi`
	filename := createFile("./flypy_user.txt", content)
	defer func() {
		_ = os.RemoveAll("./flypy_user.txt")
	}()
	cols := []Column{COLUMN_TEXT, COLUMN_CODE}
	tests := []struct {
		name     string
		filename string
		want     []Data
	}{
		{
			name: "xhup", filename: filename,
			want: []Data{
				{Text: "即使", Code: "jiui", cols: &cols},
				{Text: "回忆", Code: "hvyi", cols: &cols},
				{Text: "华为", Code: "hxww", cols: &cols},
			},
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// start := time.Now()
			fes := LoadItems(tt.filename)
			// duration1 := time.Since(start)
			// fmt.Println("======================================================")
			// fmt.Println("fes >>", len(fes))
			got := make([]Data, 0)
			if len(fes) > 0 {
				for _, fe := range fes {
					for _, entry := range fe.Entries {
						got = append(got, entry.data)
					}
				}
			}
			// fmt.Println("count >>", len(got))
			got = got[:3]
			if !reflect.DeepEqual(got, tt.want) {
				for _, d := range got {
					t.Errorf("Load Item Count got  = %+v", d)
				}
				for _, d := range tt.want {
					t.Errorf("Load Item Count want = %+v", d)
				}
			}
			// fmt.Println("======================================================")
			// fmt.Println("load duration >>", duration1)
		})
	}
}

func Test_LoadItems(t *testing.T) {
	filenames := mockFile()
	defer func() {
		_ = os.RemoveAll("./tmp")
	}()
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{"xkdj", args{filenames[0]}, 20},
		{"tigress", args{filenames[1]}, 14},
		{"onlyhead", args{filenames[2]}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// start := time.Now()
			fes := LoadItems(tt.args.path)
			// duration1 := time.Since(start)
			// fmt.Println("======================================================")
			// fmt.Println("fes >>", len(fes))
			entries := make([]*Entry, 0)
			if len(fes) > 0 {
				for _, fe := range fes {
					// fmt.Println("fe >>", fe.FilePath)
					entries = append(entries, fe.Entries...)
					// for _, e := range fe.Entries {
					// 	fmt.Println("entry >>", string(e.raw))
					// }
				}
			}
			// fmt.Println("count >>", len(entries))
			if len(entries) != tt.want {
				t.Errorf("Load Item Count = %v, want %v", len(entries), tt.want)
			}
			// fmt.Println("======================================================")
			// fmt.Println("load duration >>", duration1)
		})
	}
}

func mockFile() []string {
	filenames := make([]string, 0)
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
  - xkjd6.danzi
# 扩展：词组
  - xkjd6.cizu
# 扩展：符号
  - xkjd6.fuhao
...
所以	m
`
	filenames = append(filenames, createFile("./tmp/xkjd6.dict.yaml", content))
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
	createFile("./tmp/xkjd6.danzi.dict.yaml", content)
	content = `
---
name: xkjd6.cizu
version: "Q1"
sort: original
import_tables:
# 扩展：单字
  - xkjd6.cizu2
...
并不比	bbb
彬彬	bbbb
斌斌	bbbbo
`
	createFile("./tmp/xkjd6.cizu.dict.yaml", content)
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
	createFile("./tmp/xkjd6.fuhao.dict.yaml", content)
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
	createFile("./tmp/xkjd6.cizu2.dict.yaml", content)
	content = `
name: tigress
version: "2025.03.07"
sort: by_weight
columns:
  - text
  - weight
  - code
  - stem
encoder:
  rules:
    - length_equal: 2
      formula: "AaAbBaBb"
    - length_equal: 3
      formula: "AaBaCaCb"
    - length_in_range: [4, 10]
      formula: "AaBaCaZa"
import_tables:
  - tigress_ci
  - tigress_simp_ci
#  - tigress_user
...

的	10359470	u	un
的	256	uni
的	256	unid
一	4346343	f	fi
一	256	fi
`
	filenames = append(filenames, createFile("./tmp/tigress.dict.yaml", content))
	content = `
name: tigress_ci
version: "2025.03.07"
sort: by_weight
use_preset_vocabulary: false
columns:
  - text
  - weight
  - code
  - stem
encoder:
  rules:
    - length_equal: 2
      formula: "AaAbBaBb"
    - length_equal: 3
      formula: "AaBaCaCb"
    - length_in_range: [4, 99]
      formula: "AaBaCaZa"

...
我们	116006	tuja
自己	109686	oivj
一个	105148	fijg
没有	90888	krnv
什么	80552	jntk
`
	createFile("./tmp/tigress_ci.dict.yaml", content)
	content = `
name: tigress_simp_ci
version: "2025.03.07"
sort: by_weight
use_preset_vocabulary: false
columns:
  - text
  - weight
  - code
  - stem
encoder:
  rules:
    - length_equal: 2
      formula: "AaAbBaBb"
    - length_equal: 3
      formula: "AaBaCaCb"
    - length_in_range: [4, 99]
      formula: "AaBaCaZa"

...
那个	5000	a
如果	5000	b
不是	5000	c
哪个	5000	d

`
	createFile("./tmp/tigress_simp_ci.dict.yaml", content)
	content = `
name: onlyhead
version: "2025.03.07"
sort: by_weight
import_tables:
  - onlyhead_extend
  - onlyhead_user
#  - tigress_user
`
	filenames = append(filenames, createFile("./tmp/onlyhead.dict.yaml", content))
	content = `
name: onlyhead
version: "2025.03.07"
sort: by_weight
...

那个	5000	a
如果	5000	b
`
	createFile("./tmp/onlyhead_extend.dict.yaml", content)
	content = `
不是	5000	c
哪个	5000	d
`
	createFile("./tmp/onlyhead_user.dict.yaml", content)
	return filenames
}

func createFile(name string, content string) string {
	file, err := os.Create(name)
	if err != nil {
		fmt.Println("create temp file error, ", err)
		panic(err)
	}
	defer func() {
		_ = file.Close()
	}()
	_, _ = file.WriteString(content)
	return file.Name()
}
