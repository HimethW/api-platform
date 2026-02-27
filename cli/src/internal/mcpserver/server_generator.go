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
	"strings"
	"unicode"
)

// GenerateServerCode produces the Python MCP server script from a parsed Arazzo spec.
// The generated server uses fastmcp and arazzo_runner to expose each workflow as a tool.
func GenerateServerCode(spec *ArazzoSpec, arazzoFileName string, port int) (string, error) {
	if len(spec.Workflows) == 0 {
		return "", fmt.Errorf("no workflows found in Arazzo spec to generate tools from")
	}

	var b strings.Builder

	// Imports
	b.WriteString("from fastmcp import FastMCP\n")
	b.WriteString("from arazzo_runner import ArazzoRunner\n")
	b.WriteString("\n")

	// Initialize FastMCP server
	b.WriteString(fmt.Sprintf("# Initialize FastMCP server\n"))
	b.WriteString(fmt.Sprintf("mcp = FastMCP(%q)\n", spec.Info.Title))
	b.WriteString("\n")

	// Load the Arazzo file
	b.WriteString("# Load the Arazzo file\n")
	b.WriteString(fmt.Sprintf("runner = ArazzoRunner.from_arazzo_path(\"./arazzo/%s\")\n", arazzoFileName))
	b.WriteString("\n")

	// Generate a tool for each workflow
	for i, wf := range spec.Workflows {
		if i > 0 {
			b.WriteString("\n")
		}

		funcName := camelToSnake(wf.WorkflowID)
		docstring := workflowDocstring(wf)
		params := buildParams(wf)
		inputDict := buildInputDict(wf)

		b.WriteString(fmt.Sprintf("# ── Tool %d: %s workflow\n", i+1, wf.WorkflowID))
		b.WriteString("@mcp.tool()\n")
		b.WriteString(fmt.Sprintf("async def %s(%s) -> str:\n", funcName, params))
		b.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n", docstring))
		b.WriteString("    try:\n")
		b.WriteString(fmt.Sprintf("        result = runner.execute_workflow(%q, {%s})\n", wf.WorkflowID, inputDict))
		b.WriteString("        if result.outputs:\n")
		b.WriteString("            return f\"Workflow Success. Outputs: {result.outputs}\"\n")
		b.WriteString("        return f\"Workflow Result: {result}\"\n")
		b.WriteString("    except Exception as e:\n")
		b.WriteString("        return f\"Workflow Error: {str(e)}\"\n")
	}

	// Main entry point
	b.WriteString("\n")
	b.WriteString("\nif __name__ == \"__main__\":\n")
	b.WriteString(fmt.Sprintf("    mcp.run(transport=\"http\", host=\"0.0.0.0\", port=%d, stateless_http=True)\n", port))

	return b.String(), nil
}

// camelToSnake converts a camelCase or PascalCase string to snake_case.
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					result.WriteRune('_')
				} else if unicode.IsUpper(prev) && i+1 < len(s) && unicode.IsLower(rune(s[i+1])) {
					result.WriteRune('_')
				}
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// arazzoTypeToPython maps Arazzo/JSON Schema types to Python type hints.
func arazzoTypeToPython(t string) string {
	switch strings.ToLower(t) {
	case "integer":
		return "int"
	case "number":
		return "float"
	case "string":
		return "str"
	case "boolean":
		return "bool"
	default:
		return "str"
	}
}

// workflowDocstring returns the docstring for a workflow tool function.
func workflowDocstring(wf Workflow) string {
	if wf.Summary != "" {
		return wf.Summary
	}
	if wf.Description != "" {
		return wf.Description
	}
	return fmt.Sprintf("Execute the %s workflow", wf.WorkflowID)
}

// buildParams generates the Python function parameter list from workflow inputs.
// e.g. "pet_id: int, pet_name: str"
func buildParams(wf Workflow) string {
	if wf.Inputs == nil || len(wf.Inputs.Properties) == 0 {
		return ""
	}

	// Collect parameters in a deterministic order by iterating the map
	var parts []string
	for name, prop := range wf.Inputs.Properties {
		pyType := arazzoTypeToPython(prop.Type)
		parts = append(parts, fmt.Sprintf("%s: %s", name, pyType))
	}
	return strings.Join(parts, ", ")
}

// buildInputDict generates the Python dict literal mapping original param names
// to their snake_case function argument names.
// e.g. `"petId": pet_id, "petName": pet_name`
func buildInputDict(wf Workflow) string {
	if wf.Inputs == nil || len(wf.Inputs.Properties) == 0 {
		return ""
	}

	var parts []string
	for name := range wf.Inputs.Properties {
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}
	return strings.Join(parts, ", ")
}
