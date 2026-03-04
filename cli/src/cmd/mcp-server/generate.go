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
	"net/url"
	"os"
	"path/filepath"
	"strings"

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

# Generate with browser OAuth challenge support (PRM + WWW-Authenticate)
ap mcp-server generate -d ./my-arazzo-folder --enable-browser-auth --auth-server-url https://api.mycompany.com/oauth2/token`
)

var (
	generateFolder            string
	generatePort              int
	generateOutputDir         string
	generateEnableBrowserAuth bool
	generateAuthServerURL     string
)

var generateCmd = &cobra.Command{
	Use:   GenerateCmdLiteral,
	Short: "Generate an MCP server Docker image from an Arazzo specification",
	Long: `Generate a Docker image containing a Python MCP server from an Arazzo specification.

The command reads an Arazzo file and its referenced OpenAPI spec files from the
provided folder, generates Python MCP server code from workflows, and builds a
Docker image ready to run.

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
      --enable-browser-auth Enable browser OAuth challenge flow (401 with
                             WWW-Authenticate and PRM endpoint)
      --auth-server-url     Authorization server URL used in PRM response.
                             Required when --enable-browser-auth is set.
      --output-dir string   Directory to save generated build artifacts.
                             If not set, a temporary directory is used and
                             cleaned up automatically.

What Gets Generated:
  - Legacy mode (default):
    - mcp_server.py    Python MCP server with one @mcp.tool() per workflow
    - Dockerfile       Builds a Python 3.11 image with fastmcp and arazzo-runner
    - arazzo/          Copy of all spec files from the input folder

  - Browser-auth mode (--enable-browser-auth):
    - src/main.py, src/auth.py, src/tools.py, src/__init__.py
    - requirements.txt
    - Dockerfile running "python -m src.main"
    - arazzo/ copy of all spec files from the input folder

After a successful build, the command prints the Docker image name and the
exact 'docker run' command to start the server.`,
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
	utils.AddBoolFlag(generateCmd, utils.FlagEnableBrowserAuth, &generateEnableBrowserAuth, false, "Enable browser OAuth challenge flow (PRM + WWW-Authenticate)")
	utils.AddStringFlag(generateCmd, utils.FlagAuthServerURL, &generateAuthServerURL, "", "OAuth authorization server URL used in PRM response")

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
	if generateEnableBrowserAuth && generateAuthServerURL == "" {
		return fmt.Errorf("--%s is required when --%s is enabled", utils.FlagAuthServerURL, utils.FlagEnableBrowserAuth)
	}
	if generateEnableBrowserAuth {
		authURL := strings.TrimSpace(generateAuthServerURL)
		parsed, parseErr := url.ParseRequestURI(authURL)
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("--%s must be an absolute URL (for example: https://idp.example.com/oauth2/token)", utils.FlagAuthServerURL)
		}
		generateAuthServerURL = authURL
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

	// Step 3.5: Collect credential env vars across all workflows (legacy mode only)
	var credentialEnvVars []string
	if !generateEnableBrowserAuth {
		credentialSet := make(map[string]bool)
		for _, wf := range spec.Workflows {
			classified := mcpserver.ClassifyInputs(wf)
			for inputName := range classified.CredentialInputs {
				envVar := mcpserver.CredentialEnvVarName(spec.Info.Title, inputName)
				if !credentialSet[envVar] {
					credentialEnvVars = append(credentialEnvVars, envVar)
					credentialSet[envVar] = true
				}
			}
		}
		if len(credentialEnvVars) > 0 {
			fmt.Printf("Detected %d credential input(s) - will use environment variables\n", len(credentialEnvVars))
		}
	}

	// Step 4: Generate Python MCP server artifacts
	fmt.Println("Generating MCP server code...")
	var serverCode string
	var generatedFiles map[string]string
	if generateEnableBrowserAuth {
		generatedFiles, err = mcpserver.GenerateBrowserAuthProject(spec, arazzoFileName, generatePort, generateAuthServerURL)
		if err != nil {
			return fmt.Errorf("failed to generate browser-auth server project: %w", err)
		}
	} else {
		serverCode, err = mcpserver.GenerateServerCode(spec, arazzoFileName, generatePort)
		if err != nil {
			return fmt.Errorf("failed to generate server code: %w", err)
		}
	}

	// Step 5: Generate the Dockerfile
	dockerfileCode := mcpserver.GenerateDockerfile(generatePort, credentialEnvVars, generateEnableBrowserAuth)

	// Step 6: Build the Docker image
	fmt.Println("Building Docker image...")
	config := mcpserver.MCPServerBuildConfig{
		FolderPath:        absFolder,
		Port:              generatePort,
		ArazzoSpec:        spec,
		ArazzoFileName:    arazzoFileName,
		ServerCode:        serverCode,
		GeneratedFiles:    generatedFiles,
		DockerfileCode:    dockerfileCode,
		OutputDir:         generateOutputDir,
		EnableBrowserAuth: generateEnableBrowserAuth,
		CredentialEnvVars: credentialEnvVars,
	}

	if err := mcpserver.BuildMCPServerImage(config); err != nil {
		return err
	}

	return nil
}
