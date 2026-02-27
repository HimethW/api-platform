/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package mcpserver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ArazzoSpec represents the top-level structure of an Arazzo specification file.
type ArazzoSpec struct {
	Arazzo             string              `yaml:"arazzo"`
	Info               ArazzoInfo          `yaml:"info"`
	SourceDescriptions []SourceDescription `yaml:"sourceDescriptions"`
	Workflows          []Workflow          `yaml:"workflows"`
}

// ArazzoInfo contains metadata about the Arazzo specification.
type ArazzoInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

// SourceDescription describes an API source referenced by the Arazzo specification.
type SourceDescription struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	URL  string `yaml:"url"`
}

// Workflow represents a single workflow defined in the Arazzo specification.
type Workflow struct {
	WorkflowID  string         `yaml:"workflowId"`
	Summary     string         `yaml:"summary"`
	Description string         `yaml:"description"`
	Inputs      *WorkflowInput `yaml:"inputs"`
}

// WorkflowInput describes the input schema for a workflow.
type WorkflowInput struct {
	Type       string                   `yaml:"type"`
	Properties map[string]InputProperty `yaml:"properties"`
}

// InputProperty describes a single input parameter for a workflow.
type InputProperty struct {
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// FindArazzoFile scans the given folder for a YAML file that contains the
// top-level "arazzo" key. Returns the path to the Arazzo file.
// Returns an error if no Arazzo file is found or if multiple are found.
func FindArazzoFile(folderPath string) (string, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return "", fmt.Errorf("failed to read folder '%s': %w", folderPath, err)
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(folderPath, entry.Name())
		if isArazzoFile(filePath) {
			matches = append(matches, filePath)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no Arazzo specification file found in folder '%s'\n\nAn Arazzo file must be a .yaml or .yml file containing the top-level 'arazzo' key", folderPath)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple Arazzo specification files found in folder '%s':\n  %s\n\nPlease ensure only one Arazzo file exists in the folder", folderPath, strings.Join(matches, "\n  "))
	}

	return matches[0], nil
}

// isArazzoFile checks if a YAML file contains the top-level "arazzo" key.
func isArazzoFile(filePath string) bool {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}

	_, hasArazzo := raw["arazzo"]
	return hasArazzo
}

// ParseArazzoFile reads and parses an Arazzo specification YAML file.
func ParseArazzoFile(filePath string) (*ArazzoSpec, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Arazzo file '%s': %w", filePath, err)
	}

	var spec ArazzoSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse Arazzo file '%s': %w", filePath, err)
	}

	if spec.Arazzo == "" {
		return nil, fmt.Errorf("invalid Arazzo file '%s': missing 'arazzo' version field", filePath)
	}
	if spec.Info.Title == "" {
		return nil, fmt.Errorf("invalid Arazzo file '%s': missing 'info.title' field", filePath)
	}
	if len(spec.Workflows) == 0 {
		return nil, fmt.Errorf("invalid Arazzo file '%s': no workflows defined", filePath)
	}

	return &spec, nil
}

// ValidateSourceDescriptions checks that all OpenAPI spec files referenced in
// the Arazzo sourceDescriptions exist in the given folder.
func ValidateSourceDescriptions(spec *ArazzoSpec, folderPath string) error {
	var missing []string
	for i, sd := range spec.SourceDescriptions {
		if sd.Type != "openapi" {
			continue
		}
		// Resolve the URL relative to the folder
		resolvedPath := filepath.Join(folderPath, sd.URL)
		if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
			missing = append(missing, fmt.Sprintf("  sourceDescriptions[%d]: '%s' (name: '%s')", i, sd.URL, sd.Name))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing OpenAPI spec files referenced in Arazzo specification:\n%s\n\nPlease ensure all referenced files are present in the folder '%s'", strings.Join(missing, "\n"), folderPath)
	}

	return nil
}
