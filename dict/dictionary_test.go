package dict

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func Test_Dictionary_Search(t *testing.T) {
	type args struct {
		key []rune
		fes []*FileEntries
	}
	// cols := []Column{COLUMN_TEXT, COLUMN_CODE}
	fes1 := &FileEntries{
		Entries: []*Entry{
			NewEntry([]byte("helle world"), 0, 1, 0),
			NewEntry([]byte("hi, did eve alive?"), 0, 2, 0),
			NewEntry([]byte("你好"), 0, 3, 0),
		},
	}
	tests := []struct {
		name string
		args args
		want []*Entry
	}{
		{"case1", args{[]rune("wor"), []*FileEntries{fes1}}, []*Entry{fes1.Entries[0]}},
		{"case2", args{[]rune("hel"), []*FileEntries{fes1}}, []*Entry{fes1.Entries[0], fes1.Entries[1]}},
		{"case3", args{[]rune("你"), []*FileEntries{fes1}}, []*Entry{fes1.Entries[2]}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			dict := NewDictionary(tt.args.fes, &CacheMatcher{})
			ctx := context.Background()
			ch := make(chan []*MatchResult)
			fmt.Println("searching for", string(tt.args.key))
			go func() {
				dict.Search(tt.args.key, ch, ctx)
				close(ch)
			}()
			for ret := range ch {
				fmt.Println("ret", ret)
				entries := make([]*Entry, 0)
				for _, r := range ret {
					entries = append(entries, r.Entry)
				}
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].seek < entries[j].seek
				})
				if !reflect.DeepEqual(entries, tt.want) {
					t.Errorf("Search() = %v, want %v", entries, tt.want)
				}
			}
		})
	}
}

func Test_ParseInput(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantPair []string
		wantCols []Column
	}{
		{
			name:     "case1",
			args:     "你\t好",
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT},
		},
		{
			name:     "case2",
			args:     "你 好",
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT},
		},
		{
			name:     "case3",
			args:     "你  好",
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT}},
		{
			name:     "case4",
			args:     "你\t 好",
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT}},
		{
			name:     "case5",
			args:     "你   好\t 1",
			wantPair: []string{"你 好", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case6",
			args:     "你好 nau 1",
			wantPair: []string{"你好", "nau", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT},
		},
		{
			name:     "case7",
			args:     "nau 你好 1",
			wantPair: []string{"nau", "你好", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case8",
			args:     "  nau 你好 1 ",
			wantPair: []string{"nau", "你好", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case9",
			args:     "nau hi你好ya 1 ",
			wantPair: []string{"nau", "hi你好ya", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case10",
			args:     "nau 1 hi 你好 ya 1i ",
			wantPair: []string{"nau", "1", "hi ya 1i", "你好"},
			wantCols: []Column{COLUMN_CODE, COLUMN_WEIGHT, COLUMN_STEM, COLUMN_TEXT},
		},
		{
			name:     "case10",
			args:     "你好 ni hao 1",
			wantPair: []string{"你好", "ni", "hao", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_STEM, COLUMN_WEIGHT},
		},
	}
	fields := strings.Fields("你\t好")
	fmt.Println(fields, len(fields))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, cols := ParseInput(tt.args)
			if !reflect.DeepEqual(pair, tt.wantPair) {
				t.Errorf("ParsePair() = %v, want %v", pair, tt.wantPair)
			}
			if !reflect.DeepEqual(cols, tt.wantCols) {
				t.Errorf("ParsePair() = %v, want %v", cols, tt.wantCols)
			}
		})
	}
}

func Test_ParseData(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		// cols []Column
		want Data
	}{
		{
			name: "case1",
			raw:  "你好	nau",
			// ,
			want: Data{Text: "你好", Code: "nau", cols: []Column{COLUMN_TEXT, COLUMN_CODE}},
		},
		{
			name: "case2",
			raw:  "你好\t\n",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "", cols: []Column{COLUMN_TEXT}},
		},
		{
			name: "case3",
			raw:  "你好 nau",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "nau", cols: []Column{COLUMN_TEXT, COLUMN_CODE}},
		},
		{
			name: "case4",
			raw:  "1               \t",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Weight: 1, cols: []Column{COLUMN_WEIGHT}},
		},
		{
			name: "case5",
			raw:  "你 好 nau 1",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name: "case6",
			raw:  "你 好\tnau\t1",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name: "case7",
			raw:  "你 好\t \tnau              \t1",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name: "case8",
			raw:  "你好\t \tni hao\t1",
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "ni", Stem: "hao", Weight: 1, cols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_STEM, COLUMN_WEIGHT}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, cols := ParseInput(tt.raw)
			data, _ := ParseData(pair, cols)
			if !reflect.DeepEqual(*data, tt.want) {
				t.Errorf("ParsePair() = %+v, want %+v", data, tt.want)
			}
		})
	}
}
