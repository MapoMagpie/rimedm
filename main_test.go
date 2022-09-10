package main

import (
	"fmt"
	"testing"
)

func Test_parseConfig(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
	}{
		{"1", args{path: "./testdata/config.yaml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseConfig(tt.args.path)
			fmt.Println(got)
		})
	}
}
