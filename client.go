package mcp

import (
	"context"
	"fmt"
	"strings"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Discovery holds the results of discovering an MCP server's capabilities.
type Discovery struct {
	Tools             []*gomcp.Tool
	Resources         []*gomcp.Resource
	ResourceTemplates []*gomcp.ResourceTemplate
	Prompts           []*gomcp.Prompt
	ServerInfo        *gomcp.Implementation
}

func clientInfo(version string) *gomcp.Implementation {
	return &gomcp.Implementation{
		Name:    "ob",
		Version: version,
	}
}

func discover(ctx context.Context, clientVersion string, url string) (*Discovery, error) {
	session, err := connect(ctx, clientVersion, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer func() { _ = session.Close() }()

	result := &Discovery{}

	initResult := session.InitializeResult()
	if initResult != nil {
		result.ServerInfo = initResult.ServerInfo
	}

	if initResult != nil && initResult.Capabilities.Tools != nil {
		for tool, err := range session.Tools(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list tools: %w", err)
			}
			result.Tools = append(result.Tools, tool)
		}
	}

	if initResult != nil && initResult.Capabilities.Resources != nil {
		for resource, err := range session.Resources(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list resources: %w", err)
			}
			result.Resources = append(result.Resources, resource)
		}

		for tmpl, err := range session.ResourceTemplates(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list resource templates: %w", err)
			}
			result.ResourceTemplates = append(result.ResourceTemplates, tmpl)
		}
	}

	if initResult != nil && initResult.Capabilities.Prompts != nil {
		for prompt, err := range session.Prompts(ctx, nil) {
			if err != nil {
				return nil, fmt.Errorf("list prompts: %w", err)
			}
			result.Prompts = append(result.Prompts, prompt)
		}
	}

	return result, nil
}

func callTool(ctx context.Context, clientVersion string, url string, toolName string, args map[string]any) (*gomcp.CallToolResult, error) {
	session, err := connect(ctx, clientVersion, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("call tool %q: %w", toolName, err)
	}
	return result, nil
}

func readResource(ctx context.Context, clientVersion string, url string, uri string) (*gomcp.ReadResourceResult, error) {
	session, err := connect(ctx, clientVersion, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.ReadResource(ctx, &gomcp.ReadResourceParams{
		URI: uri,
	})
	if err != nil {
		return nil, fmt.Errorf("read resource %q: %w", uri, err)
	}
	return result, nil
}

func getPrompt(ctx context.Context, clientVersion string, url string, promptName string, args map[string]string) (*gomcp.GetPromptResult, error) {
	session, err := connect(ctx, clientVersion, url)
	if err != nil {
		return nil, fmt.Errorf("connect to MCP server: %w", err)
	}
	defer func() { _ = session.Close() }()

	result, err := session.GetPrompt(ctx, &gomcp.GetPromptParams{
		Name:      promptName,
		Arguments: args,
	})
	if err != nil {
		return nil, fmt.Errorf("get prompt %q: %w", promptName, err)
	}
	return result, nil
}

func connect(ctx context.Context, clientVersion string, url string) (*gomcp.ClientSession, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("MCP source location must be an HTTP or HTTPS URL, got %q", url)
	}

	transport := &gomcp.StreamableClientTransport{Endpoint: url}
	client := gomcp.NewClient(clientInfo(clientVersion), nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return session, nil
}
