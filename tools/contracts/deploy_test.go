package contracts

import (
	"strings"
	"testing"
)

func TestDeployAppInputValidate_Success(t *testing.T) {
	in := DeployAppInput{
		SakiControlPlaneURL: "https://saki.internal?token=11111111-1111-1111-1111-111111111111",
		Name:                "my-app-1",
		Description:         "internal app",
	}

	if err := in.Validate(); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}

func TestDeployAppInputValidate_InvalidName(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "uppercase", value: "My-app"},
		{name: "underscores", value: "my_app"},
		{name: "starts with hyphen", value: "-my-app"},
		{name: "ends with hyphen", value: "my-app-"},
		{name: "too long", value: strings.Repeat("a", 64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := DeployAppInput{
				Name:        tt.value,
				Description: "valid description",
			}

			if err := in.Validate(); err == nil {
				t.Fatalf("expected validation error for name %q", tt.value)
			}
		})
	}
}

func TestDeployAppInputValidate_InvalidDescription(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "whitespace", value: "   "},
		{name: "too long", value: strings.Repeat("a", 301)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := DeployAppInput{
				Name:        "valid-app",
				Description: tt.value,
			}

			if err := in.Validate(); err == nil {
				t.Fatalf("expected validation error for description")
			}
		})
	}
}
