package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	openbindings "github.com/openbindings/openbindings-go"
)

const (
	RefPrefixTools     = "tools/"
	RefPrefixResources = "resources/"
	RefPrefixPrompts   = "prompts/"
)

func execute(ctx context.Context, clientVersion string, url string, ref string, input any) *openbindings.ExecuteOutput {
	start := time.Now()

	entityType, name, err := ParseRef(ref)
	if err != nil {
		return openbindings.FailedOutput(start, "invalid_ref", err.Error())
	}

	// ParseRef guarantees entityType is one of "tools", "resources", or "prompts".
	var output *openbindings.ExecuteOutput
	switch entityType {
	case "tools":
		output = executeTool(ctx, clientVersion, url, name, input)
	case "resources":
		output = executeResource(ctx, clientVersion, url, name)
	case "prompts":
		output = executePrompt(ctx, clientVersion, url, name, input)
	}

	output.DurationMs = time.Since(start).Milliseconds()
	return output
}

// ParseRef extracts the entity type and name from an MCP ref.
// Returns (entityType, name, error).
// Examples:
//
//	"tools/get_weather"              → ("tools", "get_weather", nil)
//	"resources/file:///src/main.rs"  → ("resources", "file:///src/main.rs", nil)
//	"prompts/code_review"            → ("prompts", "code_review", nil)
func ParseRef(ref string) (entityType string, name string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("empty MCP ref")
	}

	for _, prefix := range []string{RefPrefixTools, RefPrefixResources, RefPrefixPrompts} {
		if strings.HasPrefix(ref, prefix) {
			name := strings.TrimPrefix(ref, prefix)
			if name == "" {
				return "", "", fmt.Errorf("empty name in MCP ref %q", ref)
			}
			entityType := strings.TrimSuffix(prefix, "/")
			return entityType, name, nil
		}
	}

	return "", "", fmt.Errorf("MCP ref %q must start with %q, %q, or %q",
		ref, RefPrefixTools, RefPrefixResources, RefPrefixPrompts)
}

func executeTool(ctx context.Context, clientVersion string, url string, toolName string, input any) *openbindings.ExecuteOutput {
	args, ok := openbindings.ToStringAnyMap(input)
	if input != nil && !ok {
		return &openbindings.ExecuteOutput{
			Status: 1,
			Error: &openbindings.ExecuteError{
				Code:    "invalid_input",
				Message: fmt.Sprintf("tool input must be an object, got %T", input),
			},
		}
	}
	// MCP servers expect an object for arguments, never null.
	if args == nil {
		args = map[string]any{}
	}

	result, err := callTool(ctx, clientVersion, url, toolName, args)
	if err != nil {
		return &openbindings.ExecuteOutput{
			Status: 1,
			Error: &openbindings.ExecuteError{
				Code:    "tool_call_failed",
				Message: err.Error(),
			},
		}
	}

	return callToolResultToOutput(result)
}

func executeResource(ctx context.Context, clientVersion string, url string, uri string) *openbindings.ExecuteOutput {
	result, err := readResource(ctx, clientVersion, url, uri)
	if err != nil {
		return &openbindings.ExecuteOutput{
			Status: 1,
			Error: &openbindings.ExecuteError{
				Code:    "resource_read_failed",
				Message: err.Error(),
			},
		}
	}

	return readResourceResultToOutput(result)
}

func executePrompt(ctx context.Context, clientVersion string, url string, promptName string, input any) *openbindings.ExecuteOutput {
	args, err := toStringStringMap(input)
	if err != nil {
		return &openbindings.ExecuteOutput{
			Status: 1,
			Error: &openbindings.ExecuteError{
				Code:    "invalid_input",
				Message: fmt.Sprintf("prompt arguments must be an object with string values: %v", err),
			},
		}
	}

	result, err := getPrompt(ctx, clientVersion, url, promptName, args)
	if err != nil {
		return &openbindings.ExecuteOutput{
			Status: 1,
			Error: &openbindings.ExecuteError{
				Code:    "prompt_get_failed",
				Message: err.Error(),
			},
		}
	}

	return getPromptResultToOutput(result)
}

func callToolResultToOutput(result *gomcp.CallToolResult) *openbindings.ExecuteOutput {
	status := 0
	if result.IsError {
		status = 1
	}

	// Prefer structured content if available.
	if result.StructuredContent != nil {
		switch sc := result.StructuredContent.(type) {
		case json.RawMessage:
			var structured any
			if json.Unmarshal(sc, &structured) == nil {
				return &openbindings.ExecuteOutput{Output: structured, Status: status}
			}
		default:
			return &openbindings.ExecuteOutput{Output: sc, Status: status}
		}
	}

	output := extractContent(result.Content)
	return &openbindings.ExecuteOutput{
		Output: output,
		Status: status,
	}
}

func readResourceResultToOutput(result *gomcp.ReadResourceResult) *openbindings.ExecuteOutput {
	if len(result.Contents) == 0 {
		return &openbindings.ExecuteOutput{Status: 0}
	}

	if len(result.Contents) == 1 {
		c := result.Contents[0]
		if c.Text != "" {
			var parsed any
			if json.Unmarshal([]byte(c.Text), &parsed) == nil {
				return &openbindings.ExecuteOutput{Output: parsed, Status: 0}
			}
			return &openbindings.ExecuteOutput{Output: c.Text, Status: 0}
		}
		return &openbindings.ExecuteOutput{Output: map[string]any{"uri": c.URI, "mimeType": c.MIMEType}, Status: 0}
	}

	var items []any
	for _, c := range result.Contents {
		items = append(items, map[string]any{
			"uri":      c.URI,
			"mimeType": c.MIMEType,
			"text":     c.Text,
		})
	}
	return &openbindings.ExecuteOutput{Output: items, Status: 0}
}

func getPromptResultToOutput(result *gomcp.GetPromptResult) *openbindings.ExecuteOutput {
	var messages []any
	for _, msg := range result.Messages {
		if msg == nil {
			continue
		}
		entry := map[string]any{
			"role": string(msg.Role),
		}
		if msg.Content != nil {
			entry["content"] = contentToMap(msg.Content)
		}
		messages = append(messages, entry)
	}

	output := map[string]any{
		"messages": messages,
	}
	if result.Description != "" {
		output["description"] = result.Description
	}

	return &openbindings.ExecuteOutput{Output: output, Status: 0}
}

func extractContent(content []gomcp.Content) any {
	if len(content) == 0 {
		return nil
	}

	if len(content) == 1 {
		if tc, ok := content[0].(*gomcp.TextContent); ok {
			var parsed any
			if json.Unmarshal([]byte(tc.Text), &parsed) == nil {
				return parsed
			}
			return tc.Text
		}
	}

	allText := true
	for _, c := range content {
		if _, ok := c.(*gomcp.TextContent); !ok {
			allText = false
			break
		}
	}
	if allText {
		var texts []string
		for _, c := range content {
			texts = append(texts, c.(*gomcp.TextContent).Text)
		}
		return strings.Join(texts, "\n")
	}

	var items []any
	for _, c := range content {
		items = append(items, contentToMap(c))
	}
	return items
}

func contentToMap(c gomcp.Content) map[string]any {
	switch v := c.(type) {
	case *gomcp.TextContent:
		return map[string]any{"type": "text", "text": v.Text}
	case *gomcp.ImageContent:
		return map[string]any{"type": "image", "mimeType": v.MIMEType, "data": string(v.Data)}
	case *gomcp.AudioContent:
		return map[string]any{"type": "audio", "mimeType": v.MIMEType, "data": string(v.Data)}
	case *gomcp.ResourceLink:
		m := map[string]any{"type": "resource_link", "uri": v.URI}
		if v.Name != "" {
			m["name"] = v.Name
		}
		if v.MIMEType != "" {
			m["mimeType"] = v.MIMEType
		}
		return m
	case *gomcp.EmbeddedResource:
		m := map[string]any{"type": "resource"}
		if v.Resource != nil {
			m["uri"] = v.Resource.URI
			if v.Resource.MIMEType != "" {
				m["mimeType"] = v.Resource.MIMEType
			}
			if v.Resource.Text != "" {
				m["text"] = v.Resource.Text
			}
		}
		return m
	default:
		return map[string]any{"type": "unknown"}
	}
}

func toStringStringMap(v any) (map[string]string, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected map[string]any, got %T", v)
	}
	result := make(map[string]string, len(m))
	for k, val := range m {
		result[k] = fmt.Sprintf("%v", val)
	}
	return result, nil
}
