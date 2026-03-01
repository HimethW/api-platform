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

	"github.com/spf13/cobra"
	"github.com/wso2/api-platform/cli/internal/mcpserver"
	"github.com/wso2/api-platform/cli/utils"
)

const (
	GenerateCmdLiteral = "generate"
	GenerateCmdExample = `# Generate an MCP server Docker image from an Arazzo spec folder
ap mcp-server generate -d ./my-arazzo-folder

# Generate with a custom port
ap mcp-server generate -d ./my-arazzo-folder -p 8080

# Generate and save build artifacts to a directory for inspection or manual editing
ap mcp-server generate -d ./my-arazzo-folder --output-dir ./my-output

# Generate and auto-proxy through the active WSO2 API Platform gateway
ap mcp-server generate -d ./my-arazzo-folder --proxy

# Generate, proxy, and specify a custom gateway context path
ap mcp-server generate -d ./my-arazzo-folder --proxy --context /petstore-mcp`
)

var (
	generateFolder    string
	generatePort      int
	generateOutputDir string
	generateProxy     bool
	generateContext   string
)

var generateCmd = &cobra.Command{
	Use:   GenerateCmdLiteral,
	Short: "Generate an MCP server Docker image from an Arazzo specification",
	Long: `Generate a Docker image containing a Python MCP server from an Arazzo specification.

The command reads an Arazzo file and its referenced OpenAPI spec files from the
provided folder, generates a Python MCP server that exposes each workflow as an
MCP tool, and builds a Docker image ready to run.

Input Folder Requirements:
  - Must contain exactly one Arazzo specification file (.yaml or .yml)
  - All OpenAPI files referenced in sourceDescriptions must be present
  - The Arazzo file must have a valid 'arazzo' version key, 'info.title',
    and at least one workflow defined

Flags:
  -d, --folder string       (required) Path to folder containing the Arazzo and
                             OpenAPI spec files
  -p, --port int            Port the MCP server will listen on inside the
                             container and mapped to localhost (default: 5000)
      --output-dir string   Directory to save generated build artifacts
                             (Dockerfile, mcp_server.py, arazzo specs). Files
                             persist after the build for inspection or manual
                             editing. If not set, a temporary directory is used
                             and cleaned up automatically.
      --proxy               After building the Docker image, automatically
                             register the MCP server as a proxy on the active
                             WSO2 API Platform gateway. This starts a temporary
                             container, introspects the MCP server (tools, prompts,
                             resources), generates the gateway config, and deploys
                             it. Requires a configured and healthy gateway.
      --context string      Gateway context path for the MCP proxy (e.g. /petstore).
                             Only used with --proxy. If not set, a default is
                             derived from the Arazzo spec title.

What Gets Generated:
  - mcp_server.py    Python MCP server with one @mcp.tool() per workflow
  - Dockerfile       Builds a Python 3.11 image with fastmcp and arazzo-runner
  - arazzo/          Copy of all spec files from the input folder

After a successful build, the command prints the Docker image name and the
exact 'docker run' command to start the server. With --proxy, it also prints
the gateway MCP endpoint URL.`,
	Example: GenerateCmdExample,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGenerateCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	utils.AddStringFlag(generateCmd, utils.FlagFolder, &generateFolder, "", "Path to folder containing Arazzo and OpenAPI spec files (required)")
	utils.AddIntFlag(generateCmd, utils.FlagPort, &generatePort, utils.DefaultMCPServerPort, "Port the MCP server will listen on")
	utils.AddStringFlag(generateCmd, utils.FlagOutputDir, &generateOutputDir, "", "Output directory to save generated files (Dockerfile, server code, specs)")
	utils.AddBoolFlag(generateCmd, utils.FlagProxy, &generateProxy, false, "Auto-proxy the MCP server through the active WSO2 API Platform gateway")
	utils.AddStringFlag(generateCmd, utils.FlagContext, &generateContext, "", "Gateway context path for the MCP proxy (only used with --proxy)")

	generateCmd.MarkFlagRequired(utils.FlagFolder)
}

func runGenerateCommand() error {
	// Resolve folder to absolute path
	absFolder, err := filepath.Abs(generateFolder)
	if err != nil {
		return fmt.Errorf("failed to resolve folder path: %w", err)
	}

	// Verify folder exists and is a directory
	info, err := os.Stat(absFolder)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("folder does not exist: %s", absFolder)
		}
		return fmt.Errorf("failed to access folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absFolder)
	}

	// Step 1: Find the Arazzo file
	fmt.Println("Validating input folder...")
	arazzoFilePath, err := mcpserver.FindArazzoFile(absFolder)
	if err != nil {
		return err
	}
	arazzoFileName := filepath.Base(arazzoFilePath)

	// Step 2: Parse the Arazzo file
	spec, err := mcpserver.ParseArazzoFile(arazzoFilePath)
	if err != nil {
		return err
	}

	// Step 3: Validate that all referenced OpenAPI files exist
	if err := mcpserver.ValidateSourceDescriptions(spec, absFolder); err != nil {
		return err
	}
	fmt.Printf("Found Arazzo spec: %s with %d workflow(s)\n", spec.Info.Title, len(spec.Workflows))

	// Step 4: Generate the Python MCP server code
	fmt.Println("Generating MCP server code...")
	serverCode, err := mcpserver.GenerateServerCode(spec, arazzoFileName, generatePort)
	if err != nil {
		return fmt.Errorf("failed to generate server code: %w", err)
	}

	// Step 5: Generate the Dockerfile
	dockerfileCode := mcpserver.GenerateDockerfile(generatePort)

	// Step 6: Build the Docker image
	fmt.Println("Building Docker image...")
	config := mcpserver.MCPServerBuildConfig{
		FolderPath:     absFolder,
		Port:           generatePort,
		ArazzoSpec:     spec,
		ArazzoFileName: arazzoFileName,
		ServerCode:     serverCode,
		DockerfileCode: dockerfileCode,
		OutputDir:      generateOutputDir,
	}

	if err := mcpserver.BuildMCPServerImage(config); err != nil {
		return err
	}

	// Step 7: Auto-proxy if --proxy flag is set
	if generateProxy {
		fmt.Println()
		fmt.Println("Setting up gateway proxy...")

		// Determine context path: user-provided or auto-derived from Arazzo title
		contextPath := generateContext
		if contextPath == "" {
			contextPath = mcpserver.DefaultContext(spec.Info.Title)
			fmt.Printf("Using default context: %s (override with --context)\n", contextPath)
		}

		imageName := mcpserver.SanitizeImageName(spec.Info.Title)
		proxyConfig := mcpserver.ProxyConfig{
			ImageName:  imageName,
			Port:       generatePort,
			ArazzoSpec: spec,
			Context:    contextPath,
			OutputDir:  generateOutputDir,
		}
		if err := mcpserver.AutoProxy(proxyConfig); err != nil {
			return fmt.Errorf("proxy setup failed: %w", err)
		}
	}

	return nil
}
