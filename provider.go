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

	return execute(ctx, p.clientVersion, in.Source.Location, in.Ref, in.Input), nil
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
