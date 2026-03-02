package commands

import "testing"

func TestParserParse(t *testing.T) {
	t.Parallel()

	allowlist := map[string]struct{}{"api": {}, "web": {}}
	parser := NewParser(allowlist)

	tests := []struct {
		name    string
		input   []string
		want    Request
		wantErr bool
	}{
		{
			name:  "deploy verb",
			input: []string{"deploy"},
			want:  Request{Action: ActionDeploy},
		},
		{
			name:  "deploy raw docker compose",
			input: []string{"docker", "compose", "pull", "&&", "docker", "compose", "up"},
			want:  Request{Action: ActionDeploy},
		},
		{
			name:  "ps verb",
			input: []string{"ps"},
			want:  Request{Action: ActionPS},
		},
		{
			name:  "ps raw docker compose",
			input: []string{"docker", "compose", "ps"},
			want:  Request{Action: ActionPS},
		},
		{
			name:  "logs verb",
			input: []string{"logs", "api"},
			want:  Request{Action: ActionLogs, Service: "api"},
		},
		{
			name:  "logs raw docker compose",
			input: []string{"docker", "compose", "logs", "web"},
			want:  Request{Action: ActionLogs, Service: "web"},
		},
		{
			name:    "reject unknown command",
			input:   []string{"docker", "compose", "rm", "-f"},
			wantErr: true,
		},
		{
			name:    "reject logs unknown service",
			input:   []string{"logs", "db"},
			wantErr: true,
		},
		{
			name:    "reject logs with invalid chars",
			input:   []string{"logs", "api;rm"},
			wantErr: true,
		},
		{
			name:    "reject extra args",
			input:   []string{"ps", "--all"},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := parser.Parse(testCase.input)
			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Fatalf("unexpected request: got=%+v want=%+v", got, testCase.want)
			}
		})
	}
}

func TestParserLogsDisabled(t *testing.T) {
	t.Parallel()

	parser := NewParser(map[string]struct{}{})
	if _, err := parser.Parse([]string{"logs", "api"}); err == nil {
		t.Fatalf("expected an error when logs allowlist is empty")
	}
}
