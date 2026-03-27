// Package mcp implements the MCP (Model Context Protocol) binding format provider
// for OpenBindings.
//
// The provider handles:
//   - Discovering tools, resources, and prompts from MCP servers
//   - Converting MCP entities to OpenBindings interfaces
//   - Executing operations via the MCP JSON-RPC protocol
//
// Only the Streamable HTTP transport is supported. Source locations must be
// HTTP or HTTPS URLs pointing to an MCP-capable endpoint.
package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	openbindings "github.com/openbindings/openbindings-go"
)

// FormatToken is the format identifier for MCP sources.
// Targets the 2025-11-25 MCP spec revision. Supported features:
//   - tools/list, tools/call (incl. structuredContent and outputSchema)
//   - resources/list, resources/read
//   - resources/templates/list
//   - prompts/list, prompts/get
//
// Not yet supported: resource subscriptions, sampling, icons, elicitation.
const FormatToken = "mcp@2025-11-25"

const DefaultTimeout = 30 * time.Second

type Provider struct {
	clientVersion string
}

type Option func(*Provider)

func WithClientVersion(v string) Option {
	return func(p *Provider) {
		p.clientVersion = v
	}
}

func New(opts ...Option) *Provider {
	p := &Provider{
		clientVersion: "0.0.0",
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) Formats() []string {
	return []string{FormatToken}
}

func (p *Provider) ExecuteBinding(ctx context.Context, in *openbindings.BindingExecutionInput) (*openbindings.ExecuteOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	if in.Store != nil {
		key := normalizeEndpoint(in.Source.Location)
		if key != "" {
			if stored, err := in.Store.Get(ctx, key); err == nil && len(stored) > 0 {
				if len(in.Context) == 0 {
					in.Context = stored
				} else {
					merged := make(map[string]any, len(stored)+len(in.Context))
					for k, v := range stored {
						merged[k] = v
					}
					for k, v := range in.Context {
						merged[k] = v
					}
					in.Context = merged
				}
			}
		}
	}

	headers := buildHTTPHeaders(in.Context, in.Options)
	result := execute(ctx, p.clientVersion, in.Source.Location, in.Ref, in.Input, headers)

	if result.Error != nil && isMCPAuthError(result.Error) {
		if resolveMCPContext(ctx, in) {
			headers = buildHTTPHeaders(in.Context, in.Options)
			result = execute(ctx, p.clientVersion, in.Source.Location, in.Ref, in.Input, headers)
		}
	}

	return result, nil
}

func isMCPAuthError(e *openbindings.ExecuteError) bool {
	if e == nil {
		return false
	}
	msg := strings.ToLower(e.Message)
	return strings.Contains(msg, "401") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "unauthenticated")
}

func resolveMCPContext(ctx context.Context, in *openbindings.BindingExecutionInput) bool {
	if in.Callbacks == nil || in.Callbacks.Prompt == nil {
		return false
	}

	endpoint := normalizeEndpoint(in.Source.Location)
	if endpoint == "" {
		endpoint = in.Source.Location
	}

	value, err := in.Callbacks.Prompt(ctx, fmt.Sprintf("Enter bearer token for %s", endpoint), &openbindings.PromptOptions{
		Label:  "bearerToken",
		Secret: true,
	})
	if err != nil || value == "" {
		return false
	}

	if in.Context == nil {
		in.Context = make(map[string]any)
	}
	in.Context["bearerToken"] = value

	if in.Store != nil {
		_ = in.Store.Set(ctx, endpoint, in.Context)
	}

	return true
}

func (p *Provider) CreateInterface(ctx context.Context, in *openbindings.CreateInput) (*openbindings.Interface, error) {
	if len(in.Sources) == 0 {
		return nil, &openbindings.ExecuteError{Code: "no_sources", Message: "no sources provided"}
	}
	src := in.Sources[0]

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	disc, err := discover(ctx, p.clientVersion, src.Location)
	if err != nil {
		return nil, fmt.Errorf("MCP discovery: %w", err)
	}

	iface, err := convertToInterface(disc, src.Location)
	if err != nil {
		return nil, fmt.Errorf("MCP convert: %w", err)
	}

	if in.Name != "" {
		iface.Name = in.Name
	}
	if in.Version != "" {
		iface.Version = in.Version
	}
	if in.Description != "" {
		iface.Description = in.Description
	}

	return iface, nil
}

// buildHTTPHeaders constructs HTTP headers from binding context credentials
// and execution options for the MCP Streamable HTTP transport.
func buildHTTPHeaders(bindCtx map[string]any, opts *openbindings.ExecutionOptions) map[string]string {
	headers := map[string]string{}

	if token := openbindings.ContextBearerToken(bindCtx); token != "" {
		headers["Authorization"] = "Bearer " + token
	} else if key := openbindings.ContextAPIKey(bindCtx); key != "" {
		headers["Authorization"] = "ApiKey " + key
	}

	if opts != nil {
		for k, v := range opts.Headers {
			headers[k] = v
		}
	}

	if len(headers) == 0 {
		return nil
	}
	return headers
}

// normalizeEndpoint extracts scheme + host from an MCP endpoint URL
// and normalizes it to a stable context store key.
func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return openbindings.NormalizeContextKey(endpoint)
	}
	return openbindings.NormalizeContextKey(u.Scheme + "://" + u.Host)
}
