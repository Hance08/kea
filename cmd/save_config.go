// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026  Hance Chin

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/hance08/kea/internal/config"
)

// saveDisplayHideDecimals persists the current display.hide_decimals value to
// the on-disk YAML config without leaking viper-registered defaults (server.*,
// database.*) into the file. We avoid viper.WriteConfig because it serializes
// the entire merged in-memory state — including defaults the user never wrote
// — which would silently grow the user's config file with keys they did not
// type. Instead, we round-trip the YAML as a generic map and update only the
// single key in place. Unrelated keys (and their comments) are preserved
// verbatim; a comment trailing the rewritten hide_decimals value itself is
// dropped, since we rebuild that leaf node.
func saveDisplayHideDecimals(cfg *config.Config) error {
	return rewriteYAMLKey(cfg.ConfigPath, []string{"display", "hide_decimals"},
		"!!bool", boolToYAML(cfg.Display.HideDecimals))
}

// saveDefaultCurrency persists the current defaults.currency value to the
// on-disk YAML config using the same leak-free in-place rewrite as
// saveDisplayHideDecimals, for the same reason: viper.WriteConfig would
// silently write registered server.* defaults (including Docker env-var
// overrides like KEA_SERVER_HOST/KEA_SERVER_PORT) into the user's file.
func saveDefaultCurrency(cfg *config.Config) error {
	return rewriteYAMLKey(cfg.ConfigPath, []string{"defaults", "currency"},
		"!!str", cfg.Defaults.Currency)
}

// rewriteYAMLKey updates a single scalar key in path within the YAML file at
// configPath, in place, preserving every other key verbatim.
func rewriteYAMLKey(configPath string, path []string, tag, value string) error {
	if configPath == "" {
		return fmt.Errorf("save %s: config path is empty", strings.Join(path, "."))
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config %q: %w", configPath, err)
	}

	root := &yaml.Node{}
	if err := yaml.Unmarshal(raw, root); err != nil {
		return fmt.Errorf("parse config %q: %w", configPath, err)
	}

	if err := setYAMLScalar(root, path, tag, value); err != nil {
		return fmt.Errorf("update %s: %w", strings.Join(path, "."), err)
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0o600); err != nil {
		return fmt.Errorf("write config %q: %w", configPath, err)
	}
	return nil
}

// setYAMLScalar walks a yaml.Node tree following path and sets the leaf to a
// scalar with the given tag (e.g. "!!bool", "!!str") and value. Intermediate
// maps are created if missing. The root may be a DocumentNode (the result of
// yaml.Unmarshal into a *yaml.Node) or a bare MappingNode (empty document).
func setYAMLScalar(root *yaml.Node, path []string, tag, value string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}

	var mapping *yaml.Node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		}
		mapping = root.Content[0]
	} else {
		mapping = root
	}

	if mapping.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind %d", mapping.Kind)
	}

	for i, key := range path {
		isLeaf := i == len(path)-1
		valNode := mappingChild(mapping, key)
		if valNode == nil {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			if isLeaf {
				valNode = &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
			} else {
				valNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			mapping.Content = append(mapping.Content, keyNode, valNode)
		} else if isLeaf {
			valNode.Kind = yaml.ScalarNode
			valNode.Tag = tag
			valNode.Value = value
			valNode.Style = 0
		}
		if !isLeaf {
			if valNode.Kind != yaml.MappingNode {
				return fmt.Errorf("path component %q is not a mapping", key)
			}
			mapping = valNode
		}
	}
	return nil
}

// mappingChild returns the value node associated with key in a MappingNode, or
// nil if absent. MappingNode.Content alternates key, value, key, value, ...
func mappingChild(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func boolToYAML(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
