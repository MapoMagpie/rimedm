package dict

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestDictionary_Search(t *testing.T) {
	type args struct {
		key []rune
		fes []*FileEntries
	}
	fes1 := &FileEntries{
		Entries: []*Entry{
			NewEntry([]byte("helle world"), "", 0, 0),
			NewEntry([]byte("hi, did eve alive?"), "", 0, 0),
			NewEntry([]byte("你好"), "", 0, 0),
		},
	}
	fes2 := LoadItems("../rime/xkjd/xkjd6.dict.yaml")
	fmt.Println(len(fes2))
	tests := []struct {
		name string
		args args
	}{
		{"3", args{[]rune("wor"), []*FileEntries{fes1}}},
		{"1", args{[]rune("hel"), []*FileEntries{fes1}}},
		{"2", args{[]rune("你"), []*FileEntries{fes1}}},
		{"load", args{[]rune("hmxa"), fes2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := NewDictionary(tt.args.fes, &CacheMatcher{})
			ctx := context.Background()
			ch := make(chan []*MatchResult)
			fmt.Println("searching for", string(tt.args.key))
			go dict.Search(tt.args.key, ch, ctx)
			for ret := range ch {
				fmt.Println(ret)
			}
		})
	}
}

func TestParseInput(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name string
		args args
		want [3]string
	}{
		{"1", args{"你\t好"}, [3]string{"你", "好", ""}},
		{"1", args{"你 好"}, [3]string{"你", "好", ""}},
		{"1", args{"你  好"}, [3]string{"你", "好", ""}},
		{"1", args{"你\t 好"}, [3]string{"你", "好", ""}},
		{"1", args{"你   好\t 1"}, [3]string{"你", "好", "1"}},
		{"1", args{"你好 nau 1"}, [3]string{"你好", "nau", "1"}},
		{"1", args{"nau 你好 1"}, [3]string{"你好", "nau", "1"}},
		{"1", args{"  nau 你好 1 "}, [3]string{"你好", "nau", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseInput(tt.args.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePair() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePair(t *testing.T) {
	type args struct {
		raw []byte
	}
	tests := []struct {
		name string
		args args
		want [][]byte
	}{
		{
			name: "case1",
			args: args{
				[]byte("你好	nau"),
			},
			want: [][]byte{
				[]byte("你好"),
				[]byte("nau"),
			},
		},
		{
			name: "case2",
			args: args{
				[]byte("你好\t\n"),
			},
			want: [][]byte{
				[]byte("你好"),
			},
		},
		{
			name: "case3",
			args: args{
				[]byte("你好 nau"),
			},
			want: [][]byte{
				[]byte("你好 nau"),
			},
		},
		{
			name: "case4",
			args: args{
				[]byte(" "),
			},
			want: [][]byte{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePair(tt.args.raw); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePair() = %v, want %v", got, tt.want)
			}
		})
	}
}
