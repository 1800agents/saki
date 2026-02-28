package contracts

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	maxNameLength        = 63
	maxDescriptionLength = 300
)

var dnsSafeNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// DeployAppInput is the request payload for the saki_deploy_app tool call.
type DeployAppInput struct {
	SakiControlPlaneURL string `json:"saki_control_plane_url"`
	Name                string `json:"name"`
	Description         string `json:"description"`
}

// DeployAppOutput is the response payload for the saki_deploy_app tool call.
type DeployAppOutput struct {
	AppID        string `json:"app_id"`
	DeploymentID string `json:"deployment_id"`
	Image        string `json:"image"`
	URL          string `json:"url"`
	Status       string `json:"status"`
}

func (in DeployAppInput) Validate() error {
	if err := validateName(in.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}

	if err := validateDescription(in.Description); err != nil {
		return fmt.Errorf("invalid description: %w", err)
	}

	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("must not be empty")
	}

	if len(name) > maxNameLength {
		return fmt.Errorf("must be %d characters or fewer", maxNameLength)
	}

	if !dnsSafeNamePattern.MatchString(name) {
		return fmt.Errorf("must be a DNS-safe slug (lowercase letters, digits, and hyphens, starting/ending with alphanumeric)")
	}

	return nil
}

func validateDescription(description string) error {
	description = strings.TrimSpace(description)
	if description == "" {
		return fmt.Errorf("must not be empty")
	}

	if len(description) > maxDescriptionLength {
		return fmt.Errorf("must be %d characters or fewer", maxDescriptionLength)
	}

	return nil
}
