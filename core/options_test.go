package core

import (
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
