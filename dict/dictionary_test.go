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
	fes1 := &FileEntries{
		Entries: []*Entry{
			NewEntry([]byte("helle world"), "", 1, 0),
			NewEntry([]byte("hi, did eve alive?"), "", 2, 0),
			NewEntry([]byte("你好"), "", 3, 0),
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
	type args struct {
		raw string
	}
	tests := []struct {
		name string
		args args
		want [3]string
	}{
		{"case1", args{"你\t好"}, [3]string{"你 好", "", ""}},
		{"case2", args{"你 好"}, [3]string{"你 好", "", ""}},
		{"case3", args{"你  好"}, [3]string{"你 好", "", ""}},
		{"case4", args{"你\t 好"}, [3]string{"你 好", "", ""}},
		{"case5", args{"你   好\t 1"}, [3]string{"你 好", "", "1"}},
		{"case6", args{"你好 nau 1"}, [3]string{"你好", "nau", "1"}},
		{"case7", args{"nau 你好 1"}, [3]string{"你好", "nau", "1"}},
		{"case8", args{"  nau 你好 1 "}, [3]string{"你好", "nau", "1"}},
		{"case9", args{"nau hi你好ya 1 "}, [3]string{"hi你好ya", "nau", "1"}},
		{"case10", args{"nau 1 hi 你好 ya 1i "}, [3]string{"你好", "ya 1i", "1"}},
		{"case10", args{"你好 ni hao 1"}, [3]string{"你好", "ni hao", "1"}},
	}
	fields := strings.Fields("你\t好")
	fmt.Println(fields, len(fields))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseInput(tt.args.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePair() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ParsePair(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "case1",
			args: args{"你好	nau"},
			want: []string{"你好", "nau"},
		},
		{
			name: "case2",
			args: args{"你好\t\n"},
			want: []string{"你好"},
		},
		{
			name: "case3",
			args: args{"你好 nau"},
			want: []string{"你好 nau"},
		},
		{
			name: "case4",
			args: args{" "},
			want: []string{},
		},
		{
			name: "case5",
			args: args{"你 好 nau 1"},
			want: []string{"你 好 nau 1"},
		},
		{
			name: "case6",
			args: args{"你 好\tnau\t1"},
			want: []string{"你 好", "nau", "1"},
		},
		{
			name: "case7",
			args: args{"你 好\t \tnau              \t1"},
			want: []string{"你 好", "nau", "1"},
		},
		{
			name: "case8",
			args: args{"你好\t \tni hao\t1"},
			want: []string{"你好", "ni hao", "1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := make([][]byte, len(tt.want))
			for i, s := range tt.want {
				want[i] = []byte(s)
			}
			if got := ParsePair([]byte(tt.args.raw)); !reflect.DeepEqual(got, want) {
				t.Errorf("ParsePair() = %v, want %v", got, want)
			}
		})
	}
}
