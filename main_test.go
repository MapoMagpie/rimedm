package main

import "testing"

func Test_findRimeDefaultSchema(t *testing.T) {
	type args struct {
		rimeConfigPath string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "default.custom.yaml",
			args: args{
				"./rime/default.custom.yaml",
			},
			want: "xkjd6",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findRimeDefaultSchema(tt.args.rimeConfigPath); got != tt.want {
				t.Errorf("findRimeDefaultSchema() = %v, want %v", got, tt.want)
			}
		})
	}
}
