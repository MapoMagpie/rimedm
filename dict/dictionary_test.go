package dict

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	// "github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/sahilm/fuzzy"
)

func Test_fuzzy_Search(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		list    []string
		want    []string
	}{
		{
			name: "case1", pattern: "foo",
			list: []string{
				"foo",
				"foofoo",
				"foobar",
				"fbarrrrrrrrrrrrrrroo",
				"bfaroo",
				"bfoo",
				"barfoo",
				"fo",
			},
			want: []string{
				"foo",
				"foobar",
				"foofoo",
				"bfoo",
				"barfoo",
				"fbarrrrrrrrrrrrrrroo",
				"bfaroo",
				// "fo",
			},
		},
		{
			name: "case2", pattern: "小",
			list: []string{
				"小猪",
				"小狗",
				"小羊",
				"老虎",
				"西瓜",
			},
			want: []string{
				"小羊",
				"小狗",
				"小猪",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			got := make([]string, 0)
			ranks := fuzzy.Find(tt.pattern, tt.list)
			for _, rank := range ranks {
				fmt.Printf("all: %+v\n", rank)
				got = append(got, rank.Str)
			}
			// same
			// for _, str := range tt.list {
			// 	rank := fuzzy.Find(tt.pattern, []string{str})
			// 	if len(rank) > 0 {
			// 		fmt.Printf("one by one: %+v\n", rank[0])
			// 		got = append(got, rank[0].Str)
			// 	}
			// }
			if !reflect.DeepEqual(got, tt.want) {
				rawLen := len(got)
				wantLen := len(tt.want)
				info := ""
				for i := range int(math.Max(float64(rawLen), float64(wantLen))) {
					var raw string
					var want string
					if i < rawLen {
						raw = got[i]
					}
					if i < wantLen {
						want = tt.want[i]
					}
					info += "got: " + raw + "\t\twant:" + want + "\n"
				}
				t.Errorf("Search() key: [%s] got and want\n%s", tt.pattern, info)
			}
		})
	}
}

func Test_Dictionary_Search(t *testing.T) {
	type args struct {
		key       string
		fes       []*FileEntries
		useColumn Column
	}
	cols := []Column{COLUMN_TEXT, COLUMN_CODE}
	fes1 := &FileEntries{
		Entries: []*Entry{
			NewEntry([]byte("helle	world"), 0, 0, 0, &cols),
			NewEntry([]byte("问好	hi, did eve alive?"), 0, 0, 0, &cols),
			NewEntry([]byte("据说人是可以吃的	nihao"), 0, 0, 0, &cols),
			NewEntry([]byte("唉？你是说被吃？	foobar"), 0, 0, 0, &cols),
			NewEntry([]byte("嗯！就是这个意思。	barfoo"), 0, 0, 0, &cols),
			NewEntry([]byte("真好啊，还能这样。	foofoo"), 0, 0, 0, &cols),
			NewEntry([]byte("是阿叶告诉我的。…	fo"), 0, 0, 0, &cols),
			NewEntry([]byte("阿叶吗，不着调的家伙。	fooo"), 0, 0, 0, &cols),
			NewEntry([]byte("你有多久没进食过了？	faoo"), 0, 0, 0, &cols),
			NewEntry([]byte("上次从地底出来的时候。	fbaroo"), 0, 0, 0, &cols),
			NewEntry([]byte("那你挺节能的。	end"), 0, 0, 0, &cols),
		},
	}
	tests := []struct {
		name string
		args args
		want []*Entry
	}{
		{
			name: "case1",
			args: args{"foo", []*FileEntries{fes1}, COLUMN_CODE},
			want: []*Entry{
				NewEntry([]byte("阿叶吗，不着调的家伙。	fooo"), 0, 0, 0, &cols),
				NewEntry([]byte("唉？你是说被吃？	foobar"), 0, 0, 0, &cols),
				NewEntry([]byte("真好啊，还能这样。	foofoo"), 0, 0, 0, &cols),
				NewEntry([]byte("你有多久没进食过了？	faoo"), 0, 0, 0, &cols),
				NewEntry([]byte("上次从地底出来的时候。	fbaroo"), 0, 0, 0, &cols),
				NewEntry([]byte("嗯！就是这个意思。	barfoo"), 0, 0, 0, &cols),
			},
		},
		{
			name: "case2",
			args: args{"是", []*FileEntries{fes1}, COLUMN_TEXT},
			want: []*Entry{
				NewEntry([]byte("是阿叶告诉我的。…	fo"), 0, 0, 0, &cols),
				NewEntry([]byte("据说人是可以吃的	nihao"), 0, 0, 0, &cols),
				NewEntry([]byte("唉？你是说被吃？	foobar"), 0, 0, 0, &cols),
				NewEntry([]byte("嗯！就是这个意思。	barfoo"), 0, 0, 0, &cols),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			dict := NewDictionary(tt.args.fes, &CacheMatcher{})
			ctx := context.Background()
			ch := make(chan MatchResultChunk)
			fmt.Println("searching for", string(tt.args.key))
			go func() {
				dict.Search(tt.args.key, tt.args.useColumn, 0, ch, ctx)
				close(ch)
			}()
			for ret := range ch {
				sort.Slice(ret.Result, func(i, j int) bool {
					return ret.Result[i].score > ret.Result[j].score
				})
				fmt.Println("ret", ret)
				entries := make([]Entry, 0)
				for _, r := range ret.Result {
					fmt.Printf("ret: text: %s\tscore:%d\n", r.Entry.raw, r.score)
					entries = append(entries, *r.Entry)
				}
				want := make([]Entry, 0, len(tt.want))
				for _, w := range tt.want {
					want = append(want, *w)
				}
				if !reflect.DeepEqual(entries, want) {
					rawLen := len(entries)
					wantLen := len(want)
					info := ""
					for i := range int(math.Max(float64(rawLen), float64(wantLen))) {
						raw := "EMPTY"
						wan := "EMPTY"
						if i < rawLen {
							raw = entries[i].raw
						}
						if i < wantLen {
							wan = want[i].raw
						}
						info += "got: " + raw + "\t\twant:" + wan + "\n"
					}
					t.Errorf("Search() key: [%s] got and want\n%s", tt.args.key, info)
				}
			}
		})
	}
}

func Test_ParseInput(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		hasStem  bool
		wantPair []string
		wantCols []Column
	}{
		{
			name:     "case1",
			args:     "你\t好",
			hasStem:  true,
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT},
		},
		{
			name:     "case2",
			args:     "你 好",
			hasStem:  true,
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT},
		},
		{
			name:     "case3",
			args:     "你  好",
			hasStem:  true,
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT}},
		{
			name:     "case4",
			args:     "你\t 好",
			hasStem:  true,
			wantPair: []string{"你 好"},
			wantCols: []Column{COLUMN_TEXT}},
		{
			name:     "case5",
			args:     "你   好\t 1",
			hasStem:  true,
			wantPair: []string{"你 好", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case6",
			args:     "你好 nau 1",
			hasStem:  true,
			wantPair: []string{"你好", "nau", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT},
		},
		{
			name:     "case7",
			args:     "nau 你好 1",
			hasStem:  true,
			wantPair: []string{"nau", "你好", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case8",
			args:     "  nau 你好 1 ",
			hasStem:  true,
			wantPair: []string{"nau", "你好", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case9",
			args:     "nau hi你好ya 1 ",
			hasStem:  true,
			wantPair: []string{"nau", "hi你好ya", "1"},
			wantCols: []Column{COLUMN_CODE, COLUMN_TEXT, COLUMN_WEIGHT},
		},
		{
			name:     "case10",
			args:     "nau 1 hi 你好 ya 1i ",
			hasStem:  true,
			wantPair: []string{"nau hi ya", "1", "你好", "1i"},
			wantCols: []Column{COLUMN_CODE, COLUMN_WEIGHT, COLUMN_TEXT, COLUMN_STEM},
		},
		{
			name:     "case11",
			args:     "你好 ni hao 1",
			hasStem:  true,
			wantPair: []string{"你好", "ni", "1", "hao"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT, COLUMN_STEM},
		},
		{
			name:     "case12",
			args:     "ni ni",
			hasStem:  true,
			wantPair: []string{"ni", "ni"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE},
		},
		{
			name:     "case13",
			args:     "你好nihao",
			hasStem:  true,
			wantPair: []string{"你好nihao"},
			wantCols: []Column{COLUMN_TEXT},
		},
		{
			name:     "case14",
			args:     "你好 ni hao 1",
			hasStem:  false,
			wantPair: []string{"你好", "ni hao", "1"},
			wantCols: []Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT},
		},
		{
			name:     "case15",
			args:     "nau 1 hi 你好 ya 1i ",
			hasStem:  false,
			wantPair: []string{"nau hi ya 1i", "1", "你好"},
			wantCols: []Column{COLUMN_CODE, COLUMN_WEIGHT, COLUMN_TEXT},
		},
	}
	// fields := strings.Fields("你\t好")
	// fmt.Println(fields, len(fields))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, cols := ParseInput(tt.args, tt.hasStem)
			if !reflect.DeepEqual(pair, tt.wantPair) {
				t.Errorf("ParsePair() pair = %v, want %v", pair, tt.wantPair)
			}
			if !reflect.DeepEqual(cols, tt.wantCols) {
				t.Errorf("ParsePair() cols = %v, want %v", cols, tt.wantCols)
			}
		})
	}
}

func Test_ParseData(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		hasStem bool
		// cols []Column
		want Data
	}{
		{
			name:    "case1",
			raw:     "你好	nau",
			hasStem: true,
			// ,
			want: Data{Text: "你好", Code: "nau", cols: &[]Column{COLUMN_TEXT, COLUMN_CODE}},
		},
		{
			name:    "case2",
			raw:     "你好\t\n",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "", cols: &[]Column{COLUMN_TEXT}},
		},
		{
			name:    "case3",
			raw:     "你好 nau",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "nau", cols: &[]Column{COLUMN_TEXT, COLUMN_CODE}},
		},
		{
			name:    "case4",
			raw:     "1               \t",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Weight: 1, cols: &[]Column{COLUMN_WEIGHT}},
		},
		{
			name:    "case5",
			raw:     "你 好 nau 1",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name:    "case6",
			raw:     "你 好\tnau\t1",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name:    "case7",
			raw:     "你 好\t \tnau              \t1",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你 好", Code: "nau", Weight: 1, cols: &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
		{
			name:    "case8",
			raw:     "你好\t \tni hao\t1",
			hasStem: true,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "ni", Stem: "hao", Weight: 1, cols: &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT, COLUMN_STEM}},
		},
		{
			name:    "case9",
			raw:     "你好\t \tni hao\t1",
			hasStem: false,
			// cols: []Column{COLUMN_TEXT, COLUMN_CODE},
			want: Data{Text: "你好", Code: "ni hao", Stem: "", Weight: 1, cols: &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair, cols := ParseInput(tt.raw, tt.hasStem)
			data, _ := ParseData(pair, &cols)
			if !reflect.DeepEqual(data, tt.want) {
				t.Errorf("ParsePair() = %+v, want %+v", data, tt.want)
			}
		})
	}
}

func Test_fastParseData(t *testing.T) {
	cols1 := &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT}
	cols2 := &[]Column{COLUMN_TEXT, COLUMN_WEIGHT, COLUMN_CODE}
	cols3 := &[]Column{COLUMN_TEXT, COLUMN_CODE, COLUMN_WEIGHT, COLUMN_STEM}
	cols4 := &[]Column{COLUMN_TEXT, COLUMN_WEIGHT, COLUMN_CODE, COLUMN_STEM}
	tests := []struct {
		name string
		raw  string
		cols *[]Column
		want Data
	}{
		{name: "normal1", raw: "加	ja	100", cols: cols1,
			want: Data{Text: "加", Code: "ja", Stem: "", Weight: 100, cols: cols1}},
		{name: "miss weight", raw: "加	ja", cols: cols1,
			want: Data{Text: "加", Code: "ja", Stem: "", Weight: 0, cols: cols1}},
		{name: "normal2", raw: "加	100	ja", cols: cols2,
			want: Data{Text: "加", Code: "ja", Stem: "", Weight: 100, cols: cols2}},
		{name: "miss weight in middle", raw: "加	ja", cols: cols2,
			want: Data{Text: "加", Code: "ja", Stem: "", Weight: 0, cols: cols2}},
		{name: "normal3", raw: "加	ja	100	aj", cols: cols3,
			want: Data{Text: "加", Code: "ja", Stem: "aj", Weight: 100, cols: cols3}},
		{name: "miss code in middle", raw: "加		100	aj", cols: cols3,
			want: Data{Text: "加", Code: "", Stem: "aj", Weight: 100, cols: cols3}},
		// {name: "miss code column in middle", raw: "加	100	aj", cols: cols3,
		// 	want: Data{Text: "加", Code: "", Stem: "aj", Weight: 100, cols: cols3}},
		{name: "miss code in middle", raw: "加	aj", cols: cols3,
			want: Data{Text: "加", Code: "aj", Stem: "", Weight: 0, cols: cols3}},
		{name: "miss weight in middle 2", raw: "加	aj", cols: cols4,
			want: Data{Text: "加", Code: "aj", Stem: "", Weight: 0, cols: cols4}},
		{name: "more column", raw: "加	100	aj	ak	al	ac	av", cols: cols4,
			want: Data{Text: "加", Code: "aj", Stem: "ak", Weight: 100, cols: cols4}},
		{name: "more column and miss weight", raw: "加	aj	ak	al	ac	av", cols: cols4,
			want: Data{Text: "加", Code: "aj", Stem: "ak", Weight: 0, cols: cols4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fastParseData(tt.raw, tt.cols)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fastParseData() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
