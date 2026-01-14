package utils

import (
	"bufio"
	"context"
	"strings"
	"testing"
	"time"
)

func TestReadLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "read line",
			input:   "test line\n",
			want:    "test line",
			wantErr: false,
		},
		{
			name:    "read line with spaces",
			input:   "  test line  \n",
			want:    "test line",
			wantErr: false,
		},
		{
			name:    "empty line",
			input:   "\n",
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			ctx := context.Background()
			got, err := ReadLine(ctx, reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ReadLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadLine_ContextCancellation(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("test\n"))
	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel context immediately
	cancel()
	
	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)
	
	_, err := ReadLine(ctx, reader)
	if err != context.Canceled {
		t.Errorf("ReadLine() error = %v, want context.Canceled", err)
	}
}
