package utils

import (
	"testing"
)

func TestHashEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []byte
		b    []byte
		want bool
	}{
		{
			name: "equal hashes",
			a:    []byte{1, 2, 3, 4},
			b:    []byte{1, 2, 3, 4},
			want: true,
		},
		{
			name: "different hashes",
			a:    []byte{1, 2, 3, 4},
			b:    []byte{5, 6, 7, 8},
			want: false,
		},
		{
			name: "different lengths",
			a:    []byte{1, 2, 3},
			b:    []byte{1, 2, 3, 4},
			want: false,
		},
		{
			name: "empty hashes",
			a:    []byte{},
			b:    []byte{},
			want: true,
		},
		{
			name: "nil hashes",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "one nil hash",
			a:    nil,
			b:    []byte{1, 2, 3},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("HashEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
