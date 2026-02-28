package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/1800agents/saki/tools/contracts"
	"github.com/1800agents/saki/tools/docker"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDeployToolDefinition_DescribesFullWorkflow(t *testing.T) {
	tool := deployToolDefinition()
	description := strings.ToLower(tool.Description)

	requiredPhrases := []string{
		"calling agent must clone/customize the app first",
		"docker build/push",
		"ask follow-up questions in plain language",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(description, phrase) {
			t.Fatalf("tool description must include %q, got: %q", phrase, tool.Description)
		}
	}
}

func TestDeployToolDefinition_RequiresAppDir(t *testing.T) {
	tool := deployToolDefinition()
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("input schema must be map[string]any, got %T", tool.InputSchema)
	}
	requiredAny, ok := schema["required"].([]string)
	if !ok {
		t.Fatalf("required schema must be []string, got %T", schema["required"])
	}
	hasAppDir := false
	for _, item := range requiredAny {
		if item == "app_dir" {
			hasAppDir = true
			break
		}
	}
	if !hasAppDir {
		t.Fatal("expected app_dir in required input fields")
	}
}

func TestDeployWorkflowResourceDefinition(t *testing.T) {
	res := deployWorkflowResourceDefinition()
	if res.URI != resourceURIWorkflow {
		t.Fatalf("expected resource URI %q, got %q", resourceURIWorkflow, res.URI)
	}
	if res.Name != resourceNameWorkflow {
		t.Fatalf("expected resource name %q, got %q", resourceNameWorkflow, res.Name)
	}
	if res.MIMEType != "text/markdown" {
		t.Fatalf("expected markdown MIME type, got %q", res.MIMEType)
	}
}

func TestDeployWorkflowResourceHandler_ReturnsDocument(t *testing.T) {
	result, err := deployWorkflowResourceHandler(context.Background(), &sdkmcp.ReadResourceRequest{
		Params: &sdkmcp.ReadResourceParams{URI: resourceURIWorkflow},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Contents) != 1 {
		t.Fatalf("expected one content item, got %d", len(result.Contents))
	}
	doc := result.Contents[0].Text
	requiredPhrases := []string{
		"Clone the template repository",
		"Agent-side preparation steps",
		"Tool-side execution steps",
		"app_dir",
		"docker build",
		"docker push",
		"POST /apps/prepare",
		"POST /apps",
		"https://github.com/1800agents/saki-app-template",
	}
	for _, phrase := range requiredPhrases {
		if !strings.Contains(doc, phrase) {
			t.Fatalf("expected workflow doc to include %q", phrase)
		}
	}
}

func TestFormatDeployErrorForMCP_DockerError(t *testing.T) {
	in := contracts.DeployAppInput{
		Name:   "my-app",
		AppDir: "/tmp/my-app",
	}
	baseErr := &docker.CommandError{
		Op:       "build",
		Command:  "docker build -t registry/app:tag .",
		ExitCode: 1,
		Stderr:   "failed to solve: missing Dockerfile",
		Err:      errors.New("exit status 1"),
	}

	err := formatDeployErrorForMCP(in, baseErr)
	msg := err.Error()
	required := []string{
		`docker build failed`,
		`app_dir="/tmp/my-app"`,
		`app="my-app"`,
		`command="docker build -t registry/app:tag ."`,
		`exit_code=1`,
		`missing Dockerfile`,
	}
	for _, part := range required {
		if !strings.Contains(msg, part) {
			t.Fatalf("expected formatted error to include %q, got %q", part, msg)
		}
	}
}

func TestDeployErrorFields_IncludeDockerDetails(t *testing.T) {
	in := contracts.DeployAppInput{Name: "my-app", AppDir: "/tmp/my-app"}
	baseErr := &docker.CommandError{
		Op:       "push",
		Command:  "docker push registry/app:tag",
		ExitCode: 2,
		Stderr:   "denied",
		Err:      errors.New("exit status 2"),
	}

	fields := deployErrorFields(in, baseErr)
	if fields["docker_op"] != "push" {
		t.Fatalf("expected docker_op push, got %v", fields["docker_op"])
	}
	if fields["command"] != "docker push registry/app:tag" {
		t.Fatalf("unexpected command field: %v", fields["command"])
	}
	if fields["exit_code"] != 2 {
		t.Fatalf("unexpected exit_code field: %v", fields["exit_code"])
	}
	if fields["stderr"] != "denied" {
		t.Fatalf("unexpected stderr field: %v", fields["stderr"])
	}
}

func TestEnvEnabledOrDefault(t *testing.T) {
	t.Setenv("SAKI_TOOLS_MCP_DEBUG", "")
	if !envEnabledOrDefault("SAKI_TOOLS_MCP_DEBUG", true) {
		t.Fatal("expected default true when env is unset")
	}

	t.Setenv("SAKI_TOOLS_MCP_DEBUG", "0")
	if envEnabledOrDefault("SAKI_TOOLS_MCP_DEBUG", true) {
		t.Fatal("expected false when env is 0")
	}

	t.Setenv("SAKI_TOOLS_MCP_DEBUG", "true")
	if !envEnabledOrDefault("SAKI_TOOLS_MCP_DEBUG", false) {
		t.Fatal("expected true when env is true")
	}
}
