package httpcore

import (
	"reflect"
	"testing"

	"volt/internal/model"
)

func envWith(vars ...model.Variable) model.Environment {
	return model.Environment{
		ID:        "env-1",
		Name:      "Test",
		Variables: vars,
		Active:    true,
	}
}

func TestInterpolateString(t *testing.T) {
	tests := []struct {
		name           string
		in             string
		env            model.Environment
		wantOut        string
		wantUnresolved []string
	}{
		{
			name:    "empty input",
			in:      "",
			env:     envWith(model.Variable{Name: "host", Value: "api.example.com"}),
			wantOut: "",
		},
		{
			name:    "no tokens",
			in:      "https://api.example.com/users",
			env:     envWith(model.Variable{Name: "host", Value: "api.example.com"}),
			wantOut: "https://api.example.com/users",
		},
		{
			name:    "single resolved token",
			in:      "https://{{host}}/users",
			env:     envWith(model.Variable{Name: "host", Value: "api.example.com"}),
			wantOut: "https://api.example.com/users",
		},
		{
			name: "multiple resolved tokens including adjacent",
			in:   "{{scheme}}://{{host}}{{path}}",
			env: envWith(
				model.Variable{Name: "scheme", Value: "https"},
				model.Variable{Name: "host", Value: "api.example.com"},
				model.Variable{Name: "path", Value: "/v1"},
			),
			wantOut: "https://api.example.com/v1",
		},
		{
			name:           "unresolved token passed through literally",
			in:             "https://{{host}}/{{missing}}",
			env:            envWith(model.Variable{Name: "host", Value: "api.example.com"}),
			wantOut:        "https://api.example.com/{{missing}}",
			wantUnresolved: []string{"{{missing}}"},
		},
		{
			name:           "no active environment variables leaves all tokens",
			in:             "{{a}}/{{b}}",
			env:            envWith(),
			wantOut:        "{{a}}/{{b}}",
			wantUnresolved: []string{"{{a}}", "{{b}}"},
		},
		{
			name:           "case sensitive match",
			in:             "{{Host}} vs {{host}}",
			env:            envWith(model.Variable{Name: "host", Value: "lower"}),
			wantOut:        "{{Host}} vs lower",
			wantUnresolved: []string{"{{Host}}"},
		},
		{
			name:           "duplicate unresolved token reported once",
			in:             "{{x}}-{{x}}-{{x}}",
			env:            envWith(),
			wantOut:        "{{x}}-{{x}}-{{x}}",
			wantUnresolved: []string{"{{x}}"},
		},
		{
			name:    "repeated resolved token",
			in:      "{{t}}{{t}}",
			env:     envWith(model.Variable{Name: "t", Value: "ab"}),
			wantOut: "abab",
		},
		{
			name:    "empty value resolves to empty string",
			in:      "x={{empty}}",
			env:     envWith(model.Variable{Name: "empty", Value: ""}),
			wantOut: "x=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, gotUnresolved := InterpolateString(tt.in, tt.env)
			if gotOut != tt.wantOut {
				t.Errorf("out = %q, want %q", gotOut, tt.wantOut)
			}
			if !reflect.DeepEqual(gotUnresolved, tt.wantUnresolved) {
				t.Errorf("unresolved = %#v, want %#v", gotUnresolved, tt.wantUnresolved)
			}
		})
	}
}
