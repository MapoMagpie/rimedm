package dict

import (
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
			NewEntry("hello world", "", 0, 0),
			NewEntry("hi, did eve alive?", "", 0, 0),
			NewEntry("你好", "", 0, 0),
		},
	}
	fes2 := LoadItems("../rime/xkjd6.dict.yaml")
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{"1", args{[]rune("hel"), []*FileEntries{fes1}}},
		{"1", args{[]rune("你"), []*FileEntries{fes1}}},
		{"load", args{[]rune("hmxa"), fes2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := NewDictionary(tt.args.fes, &CacheMatcher{})
			matched := dict.Search(tt.args.key)
			for _, entry := range matched {
				fmt.Println(entry)
				//fmt.Println("matched >>", matched)
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
		// TODO: Add test cases.
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
