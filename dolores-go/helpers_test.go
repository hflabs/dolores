package main

import (
	"reflect"
	"testing"
)

func Test_convertMapToDockerArgs(t *testing.T) {
	one := "1"
	two := "2"
	tests := []struct {
		name string
		in   map[string]string
		want map[string]*string
	}{
		{
			name: "1",
			in:   map[string]string{"1": "1", "2": "2"},
			want: map[string]*string{"1": &one, "2": &two},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertMapToDockerArgs(tt.in); !reflect.DeepEqual(got, tt.want) {
				for key := range tt.in {
					t.Errorf("convertMapToDockerArgs() for key %v:  %#v, want %#v", key, *got[key], *tt.want[key])
				}
			}
		})
	}
}
