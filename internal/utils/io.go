package utils

import (
	"bufio"
	"context"
	"strings"
)

// ReadLine reads a line from the reader and trims whitespace.
// Returns an error if the context is cancelled (e.g., Ctrl+C).
func ReadLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	resultChan := make(chan result, 1)

	go func() {
		input, err := reader.ReadString('\n')
		if err != nil {
			resultChan <- result{"", err}
			return
		}
		resultChan <- result{strings.TrimSpace(input), nil}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-resultChan:
		return res.line, res.err
	}
}
