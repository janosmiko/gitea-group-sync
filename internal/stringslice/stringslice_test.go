package stringslice_test

import (
	"testing"

	"github.com/janosmiko/gitea-ldap-sync/internal/stringslice"
)

func TestContains(t *testing.T) {
	type args struct {
		s          []string
		searchterm string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Test if the searchterm is in the slice",
			args: args{
				s:          []string{"a", "b", "c"},
				searchterm: "b",
			},
			want: true,
		},
		{
			name: "Test if the searchterm is not in the slice",
			args: args{
				s:          []string{"a", "b", "c"},
				searchterm: "d",
			},
			want: false,
		},
		{
			name: "Test if the searchterm is in the slice with multiple elements",
			args: args{
				s:          []string{"f", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
				searchterm: "f",
			},
			want: true,
		},
		{
			name: "Test if searchterm is part of a string in the slice",
			args: args{
				s:          []string{"admina", "adminb", "adminc"},
				searchterm: "admin",
			},
			want: false,
		},
		{
			name: "Test if searchterm is part of a string in the slice",
			args: args{
				s:          []string{"admin", "adminb", "adminc"},
				searchterm: "admin",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringslice.Contains(tt.args.s, tt.args.searchterm); got != tt.want {
				t.Errorf("Contains() = %t, want %t", got, tt.want)
			}
		})
	}
}
