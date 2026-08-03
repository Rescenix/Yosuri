package main

import "testing"

func TestHasBackgroundFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "startup launch", args: []string{"--background"}, want: true},
		{name: "normal launch", args: nil, want: false},
		{name: "unrelated flag", args: []string{"--debug"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasBackgroundFlag(tt.args); got != tt.want {
				t.Fatalf("hasBackgroundFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
