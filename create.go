package mcp

import (
	"fmt"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	openbindings "github.com/openbindings/openbindings-go"
)

const DefaultSourceName = "mcpServer"

func convertToInterface(discovery *Discovery, sourceLocation string) (*openbindings.Interface, error) {
	if discovery == nil {
		return nil, fmt.Errorf("nil discovery result")
	}

	iface := openbindings.Interface{
		OpenBindings: openbindings.MaxTestedVersion,
		Operations:   map[string]openbindings.Operation{},
		Bindings:     map[string]openbindings.BindingEntry{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: {
				Format:   FormatToken,
				Location: sourceLocation,
			},
		},
	}

	if discovery.ServerInfo != nil {
		iface.Name = discovery.ServerInfo.Name
		iface.Version = discovery.ServerInfo.Version
		if discovery.ServerInfo.Title != "" {
			iface.Description = discovery.ServerInfo.Title
		}
	}

	usedKeys := map[string]string{}

	for _, tool := range discovery.Tools {
		opKey := openbindings.SanitizeKey(tool.Name)
		opKey = openbindings.ResolveKeyCollision(opKey, "tool", usedKeys)
		usedKeys[opKey] = "tool"

		desc := tool.Description
		if desc == "" {
			desc = tool.Title
		}

		op := openbindings.Operation{
			Description: desc,
		}

		if tool.InputSchema != nil {
			if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
				op.Input = schemaMap
			}
		}

		if tool.OutputSchema != nil {
			if schemaMap, ok := tool.OutputSchema.(map[string]any); ok {
				op.Output = schemaMap
			}
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixTools + tool.Name,
		}
	}

	for _, resource := range discovery.Resources {
		opKey := openbindings.SanitizeKey(resource.Name)
		opKey = openbindings.ResolveKeyCollision(opKey, "resource", usedKeys)
		usedKeys[opKey] = "resource"

		desc := resource.Description
		if desc == "" {
			desc = resource.Title
		}

		op := openbindings.Operation{
			Description: desc,
		}

		op.Input = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uri": map[string]any{
					"type":        "string",
					"const":       resource.URI,
					"description": "Resource URI",
				},
			},
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixResources + resource.URI,
		}
	}

	for _, tmpl := range discovery.ResourceTemplates {
		opKey := openbindings.SanitizeKey(tmpl.Name)
		opKey = openbindings.ResolveKeyCollision(opKey, "resource_template", usedKeys)
		usedKeys[opKey] = "resource_template"

		desc := tmpl.Description
		if desc == "" {
			desc = tmpl.Title
		}

		op := openbindings.Operation{
			Description: desc,
		}

		op.Input = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"uriTemplate": map[string]any{
					"type":        "string",
					"const":       tmpl.URITemplate,
					"description": "URI template (RFC 6570)",
				},
			},
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixResources + tmpl.URITemplate,
		}
	}

	for _, prompt := range discovery.Prompts {
		opKey := openbindings.SanitizeKey(prompt.Name)
		opKey = openbindings.ResolveKeyCollision(opKey, "prompt", usedKeys)
		usedKeys[opKey] = "prompt"

		desc := prompt.Description
		if desc == "" {
			desc = prompt.Title
		}

		op := openbindings.Operation{
			Description: desc,
		}

		if len(prompt.Arguments) > 0 {
			op.Input = promptArgsToSchema(prompt.Arguments)
		}

		iface.Operations[opKey] = op

		bindingKey := opKey + "." + DefaultSourceName
		iface.Bindings[bindingKey] = openbindings.BindingEntry{
			Operation: opKey,
			Source:    DefaultSourceName,
			Ref:       RefPrefixPrompts + prompt.Name,
		}
	}

	return &iface, nil
}

func promptArgsToSchema(args []*gomcp.PromptArgument) map[string]any {
	properties := map[string]any{}
	var required []string

	for _, arg := range args {
		prop := map[string]any{
			"type": "string",
		}
		if arg.Description != "" {
			prop["description"] = arg.Description
		}
		properties[arg.Name] = prop

		if arg.Required {
			required = append(required, arg.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}
