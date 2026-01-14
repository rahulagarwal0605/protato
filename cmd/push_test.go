package cmd

import (
"errors"
"testing"

"github.com/rahulagarwal0605/protato/internal/constants"
)

func TestPushCmdIsRetryableError(t *testing.T) {
	cmd := &PushCmd{}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "validation failed error",
			err:  errors.New(constants.ErrMsgValidationFailed + ": some details"),
			want: false,
		},
		{
			name: "compilation failed error",
			err:  errors.New(constants.ErrMsgCompilationFailed + ": some details"),
			want: false,
		},
		{
			name: "project claim error",
			err:  errors.New(constants.ErrMsgProjectClaim + ": some details"),
			want: false,
		},
		{
			name: "ownership error",
			err:  errors.New(constants.ErrMsgOwnership + ": some details"),
			want: false,
		},
		{
			name: "network error - retryable",
			err:  errors.New("network connection reset"),
			want: true,
		},
		{
			name: "push conflict - retryable",
			err:  errors.New("push rejected: non-fast-forward"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
got := cmd.isRetryableError(tt.err)
if got != tt.want {
t.Errorf("isRetryableError() = %v, want %v", got, tt.want)
}
})
	}
}
