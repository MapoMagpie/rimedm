package core

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func Test_compareMaxVersion(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		versions []string
	}{
		{"case1", "weasel-0.15.15", []string{"weasel-0.14.3", "weasel-0.15.15", "weasel-0.13.5"}},
		{"case2", "weasel-0.15.15", []string{"weasel-0.15.5", "weasel-0.15.15", "weasel-0.15.14"}},
		{"case3", "weasel-0.115.5", []string{"weasel-0.115.5", "weasel-0.15.15", "weasel-0.15.14"}},
		{"case4", "weasel-0.115.5", []string{"weasel-0.115.5", "", "weasel-0.15.14"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			sort.Slice(tt.versions, func(i, j int) bool {
				return compareVersion(tt.versions[i], tt.versions[j])
			})
			if tt.versions[0] != tt.want {
				t.Errorf("compareMaxVersion() = %v, want %v", tt.versions[0], tt.want)
			}
		})
	}
}

func Test_findRimeDicts(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		rimeConfigDir string
		want          []string
	}{
		{
			name:          "forst",
			rimeConfigDir: "$HOME/code/rime-schemes/rime-frost",
			want: []string{
				"rime_frost.dict.yaml",
				"rime_frost.dict.yaml",
				"rime_frost.dict.yaml",
				"rime_frost.dict.yaml",
				"rime_frost.dict.yaml",
			},
		},
		{
			name:          "ice",
			rimeConfigDir: "$HOME/code/rime-schemes/rime-ice",
			want: []string{
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
				"rime_ice.dict.yaml",
			},
		},
		{
			name:          "moqi",
			rimeConfigDir: "$HOME/code/rime-schemes/rime-shuangpin-fuzhuma",
			want: []string{
				"moqi_wan.extended.dict.yaml",
				"moqi_wan.extended.dict.yaml",
				"moqi_wan.extended.dict.yaml",
				"moqi_single.dict.yaml",
			},
		},
		{
			name:          "xkjd6",
			rimeConfigDir: "$HOME/code/rime-schemes/Rime_JD/rime", // 有两个方案缺失
			want: []string{
				"xkjd6.extended.dict.yaml",
			},
		},
		{
			name:          "xmjd6-rere",
			rimeConfigDir: "$HOME/code/rime-schemes/xmjd6-rere",
			want: []string{
				"xmjd6.extended.dict.yaml",
			},
		},
	}
	for _, tt := range tests[0:0] { // [0:0] disable this test
		t.Run(tt.name, func(t *testing.T) {
			got := findRimeDicts(tt.rimeConfigDir)
			base := make([]string, 0, len(got))
			for _, g := range got {
				base = append(base, filepath.Base(g))
			}
			// TODO: update the condition below to compare got with tt.want.
			if !reflect.DeepEqual(base, tt.want) {
				t.Errorf("findRimeDicts() = %v, want %v", base, tt.want)
			}
		})
	}
}
