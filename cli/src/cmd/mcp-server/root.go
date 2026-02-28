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
	"github.com/spf13/cobra"
)

const (
	McpServerCmdLiteral = "mcp-server"
	McpServerCmdExample = `# Generate an MCP server Docker image from an Arazzo spec folder
ap mcp-server generate -d ./my-arazzo-folder

# Generate with a custom port
ap mcp-server generate -d ./my-arazzo-folder -p 8080

# Generate and save build artifacts for manual editing
ap mcp-server generate -d ./my-arazzo-folder --output-dir ./my-output`
)

// McpServerCmd represents the mcp-server command
var McpServerCmd = &cobra.Command{
	Use:   McpServerCmdLiteral,
	Short: "MCP server operations",
	Long: `Generate and manage MCP (Model Context Protocol) servers from Arazzo specifications.

This command group allows you to generate a Docker image containing a Python MCP
server from an Arazzo specification. Each workflow defined in the Arazzo file is
exposed as an MCP tool, powered by fastmcp and arazzo-runner.

Prerequisites:
  - Docker must be installed and running
  - A folder containing a valid Arazzo specification file (.yaml/.yml)
  - All OpenAPI spec files referenced in the Arazzo sourceDescriptions

Available Commands:
  generate    Generate an MCP server Docker image from an Arazzo specification`,
	Example: McpServerCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	McpServerCmd.AddCommand(generateCmd)
}
