package cmd

import (
"bytes"
"io"
"os"
"testing"

"github.com/rahulagarwal0605/protato/internal/local"
)

func TestListCmdPrintLocalProjects(t *testing.T) {
	tests := []struct {
		name     string
		owned    []local.ProjectPath
		received []*local.ReceivedProject
		wantStrs []string
	}{
		{
			name:     "no projects",
			owned:    []local.ProjectPath{},
			received: []*local.ReceivedProject{},
			wantStrs: []string{"No projects found"},
		},
		{
			name:     "owned projects only",
			owned:    []local.ProjectPath{"team/service1", "team/service2"},
			received: []*local.ReceivedProject{},
			wantStrs: []string{"Owned projects:", "team/service1", "team/service2"},
		},
		{
			name:  "received projects only",
			owned: []local.ProjectPath{},
			received: []*local.ReceivedProject{
				{Project: "other/service", ProviderSnapshot: "abc123def456"},
			},
			wantStrs: []string{"Pulled projects:", "other/service", "abc123d"},
		},
		{
			name:  "both owned and received",
			owned: []local.ProjectPath{"team/service"},
			received: []*local.ReceivedProject{
				{Project: "other/service", ProviderSnapshot: "abc123def456"},
			},
			wantStrs: []string{"Owned projects:", "team/service", "Pulled projects:", "other/service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
oldStdout := os.Stdout
r, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &ListCmd{}
			cmd.printLocalProjects(tt.owned, tt.received)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			for _, wantStr := range tt.wantStrs {
				if !bytes.Contains([]byte(output), []byte(wantStr)) {
					t.Errorf("printLocalProjects() output missing %q, got:\n%s", wantStr, output)
				}
			}
		})
	}
}
