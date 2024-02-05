package dict

import (
	"fmt"
	"testing"
	"time"
)

func Test_loadItems(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
	}{
		{"1", args{"../rime/xkjd/xkjd6.dict.yaml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			start := time.Now()
			fes := LoadItems(tt.args.path)
			duration1 := time.Since(start)
			fmt.Println("======================================================")
			entries := make([]*Entry, 0)
			if len(fes) > 0 {
				for _, fe := range fes {
					entries = append(entries, fe.Entries...)
				}
			}
			//for _, entry := range list {
			//	fmt.Print(entry)
			//}
			fmt.Println("count >>", len(entries))
			fmt.Println("======================================================")
			duration2 := time.Since(start)
			fmt.Println("load duration >>", duration1)
			fmt.Println("print duration >>", duration2-duration1)
		})
	}
}
