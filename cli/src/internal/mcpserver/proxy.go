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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wso2/api-platform/cli/internal/gateway"
	mcpgen "github.com/wso2/api-platform/cli/internal/mcp"
	"github.com/wso2/api-platform/cli/utils"
)

// ProxyConfig holds all parameters needed for the auto-proxy flow.
type ProxyConfig struct {
	ImageName  string      // Docker image name to run
	Port       int         // Port the MCP server listens on
	ArazzoSpec *ArazzoSpec // Parsed Arazzo specification
	Context    string      // Gateway context path (e.g. "/my-mcp"). If empty, derived from Arazzo title
	OutputDir  string      // Directory to write generated gateway config (optional)
}

// AutoProxy orchestrates the full auto-proxy flow:
//  1. Validate gateway prerequisites (active gateway configured and healthy)
//  2. Start the MCP server container temporarily in detached mode
//  3. Wait for the MCP server to become ready
//  4. Introspect the server using the existing MCP generate logic (tools/list, resources/list, prompts/list)
//  5. Apply the generated MCP proxy config to the active gateway
//  6. Stop and remove the temporary container
func AutoProxy(config ProxyConfig) error {
	containerName := fmt.Sprintf("mcp-proxy-temp-%d", time.Now().UnixMilli())
	mcpURL := fmt.Sprintf("http://localhost:%d/mcp", config.Port)

	// ── Step 1: Check gateway prerequisites ────────────────────────────
	fmt.Println("Checking gateway connection...")
	gwClient, err := gateway.NewClientForActive()
	if err != nil {
		return fmt.Errorf("cannot connect to gateway: %w\n\n"+
			"Please configure a gateway first:\n"+
			"  ap gateway add -n <name> -s <server-url>\n"+
			"  ap gateway use -n <name>", err)
	}

	if err := checkGatewayHealth(gwClient); err != nil {
		return fmt.Errorf("gateway is not healthy: %w\n\n"+
			"Please ensure the gateway is running and accessible", err)
	}
	fmt.Println("Gateway is healthy ✓")

	// ── Step 2: Start the container ────────────────────────────────────
	fmt.Printf("Starting temporary container '%s'...\n", containerName)
	startArgs := []string{
		"run", "-d",
		"-p", fmt.Sprintf("%d:%d", config.Port, config.Port),
		"--name", containerName,
		config.ImageName,
	}
	startCmd := exec.Command("docker", startArgs...)
	startCmd.Stderr = os.Stderr
	if output, err := startCmd.Output(); err != nil {
		return fmt.Errorf("failed to start container: %w\n\n"+
			"If port %d is already in use, either:\n"+
			"  1. Stop the process using port %d\n"+
			"  2. Use a different port: ap mcp-server generate -d ./folder -p <other-port> --proxy",
			config.Port, config.Port, err)
	} else {
		containerID := strings.TrimSpace(string(output))
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		fmt.Printf("Container started: %s\n", containerID)
	}

	// Ensure cleanup on any exit path
	defer func() {
		fmt.Println("Cleaning up temporary container...")
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Stdout = nil
		stopCmd.Stderr = nil
		stopCmd.Run()

		rmCmd := exec.Command("docker", "rm", containerName)
		rmCmd.Stdout = nil
		rmCmd.Stderr = nil
		rmCmd.Run()
	}()

	// ── Step 3: Wait for the server to be ready ────────────────────────
	fmt.Println("Waiting for MCP server to be ready...")
	if err := waitForServer(mcpURL, 30*time.Second); err != nil {
		return fmt.Errorf("MCP server did not become ready in 30 seconds: %w\n\n"+
			"Check container logs: docker logs %s", err, containerName)
	}
	fmt.Println("MCP server is ready ✓")

	// ── Step 4: Introspect and generate gateway config ─────────────────
	fmt.Println("Introspecting MCP server...")

	// Determine output directory for the generated config
	outputDir := config.OutputDir
	cleanupOutputDir := false
	if outputDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		baseDir := filepath.Join(homeDir, ".wso2ap", ".tmp")
		if mkErr := os.MkdirAll(baseDir, 0755); mkErr != nil {
			return fmt.Errorf("failed to create temp base directory: %w", mkErr)
		}
		tmpDir, err := os.MkdirTemp(baseDir, "mcp-proxy-config-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		outputDir = tmpDir
		cleanupOutputDir = true
	}

	if cleanupOutputDir {
		defer os.RemoveAll(outputDir)
	}

	// Reuse the existing MCP generate logic from internal/mcp/generator.go
	// This sends initialize, tools/list, prompts/list, resources/list and writes the YAML
	if err := mcpgen.Generate(mcpURL, outputDir, "", ""); err != nil {
		return fmt.Errorf("failed to introspect MCP server: %w", err)
	}

	// ── Step 5: Apply the generated config to the gateway ──────────────
	fmt.Println("Applying MCP proxy config to gateway...")

	// Find the generated YAML config file
	configFilePath, err := findGeneratedConfig(outputDir)
	if err != nil {
		return fmt.Errorf("failed to find generated config: %w", err)
	}

	// Read the generated YAML
	yamlContent, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read generated config: %w", err)
	}

	// If the user specified a custom context, patch the YAML before applying
	if config.Context != "" {
		yamlContent = patchContextInYAML(yamlContent, config.Context)
	}

	// Apply the MCP proxy config to the gateway
	if err := applyMCPProxyConfig(gwClient, yamlContent); err != nil {
		return err
	}

	// ── Step 6: Print proxy summary ────────────────────────────────────
	gatewayServer := gwClient.GetBaseURL()
	contextPath := config.Context
	if contextPath == "" {
		contextPath = "/generated" // default from the MCP generate command
	}

	fmt.Println()
	utils.PrintBoxedMessage([]string{
		"✅ MCP proxy configured on gateway!",
		"",
		fmt.Sprintf("Gateway MCP endpoint: %s%s/mcp", gatewayServer, contextPath),
		"",
		"Important: The MCP server container must be running for the proxy to work.",
		fmt.Sprintf("Start it with: docker run -p %d:%d %s", config.Port, config.Port, config.ImageName),
	})

	return nil
}

// DefaultContext derives a default gateway context path from the Arazzo spec title.
// e.g. "Independent Pet Workflows" → "/independent-pet-workflows"
func DefaultContext(title string) string {
	name := SanitizeImageName(title)
	name = strings.TrimSuffix(name, "-mcp-server")
	if name == "" {
		name = "mcp"
	}
	return "/" + name
}

// waitForServer polls the MCP server URL until it responds or the timeout is reached.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Post(url, "application/json", nil)
		if err == nil {
			resp.Body.Close()
			// Any response (even 4xx) means the server is up
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("server at %s did not respond within %s", url, timeout)
}

// checkGatewayHealth verifies the gateway is accessible by calling GET /health.
func checkGatewayHealth(client *gateway.Client) error {
	resp, err := client.Get(utils.GatewayHealthPath)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway health check returned status %d", resp.StatusCode)
	}
	return nil
}

// findGeneratedConfig finds the YAML config file in the output directory.
func findGeneratedConfig(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory '%s': %w", dir, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			return filepath.Join(dir, name), nil
		}
	}
	return "", fmt.Errorf("no YAML config file found in %s", dir)
}

// patchContextInYAML replaces the context field value in the generated MCP YAML.
// The generator always writes `context: /generated`, so we replace it with the
// user-specified or auto-derived context.
func patchContextInYAML(yamlContent []byte, newContext string) []byte {
	// Ensure context starts with /
	if !strings.HasPrefix(newContext, "/") {
		newContext = "/" + newContext
	}

	// Simple line-based replacement for the context field
	lines := strings.Split(string(yamlContent), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "context:") {
			// Preserve the original indentation
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			lines[i] = fmt.Sprintf("%scontext: %s", indent, newContext)
			break
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// applyMCPProxyConfig sends the MCP proxy YAML to the gateway controller.
// It reuses the gateway client and resource handler pattern from the apply command.
func applyMCPProxyConfig(client *gateway.Client, yamlContent []byte) error {
	handler := gateway.GetResourceHandler(gateway.ResourceKindMCP)
	if handler == nil {
		return fmt.Errorf("MCP resource handler not found (internal error)")
	}

	// Extract metadata.name from the YAML to use as the resource handle
	handle, err := extractMetadataName(yamlContent)
	if err != nil {
		return fmt.Errorf("failed to extract resource name from generated config: %w", err)
	}

	// Check if the resource already exists
	exists, existsErr := resourceExists(client, handler, handle)
	if existsErr != nil {
		return fmt.Errorf("failed to check if MCP proxy already exists: %w", existsErr)
	}

	var resp *http.Response
	var operation string

	if exists {
		operation = "update"
		endpoint := handler.UpdateEndpoint(handle)
		resp, err = client.PutYAML(endpoint, bytes.NewReader(yamlContent))
	} else {
		operation = "create"
		endpoint := handler.CreateEndpoint()
		resp, err = client.PostYAML(endpoint, bytes.NewReader(yamlContent))
	}

	if err != nil {
		return fmt.Errorf("failed to %s MCP proxy on gateway: %w", operation, err)
	}
	defer resp.Body.Close()

	// Read and display the response
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		// Apply succeeded but couldn't read response
		fmt.Printf("MCP proxy %sd successfully\n", operation)
		return nil
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		fmt.Printf("MCP proxy %sd successfully\n", operation)
		return nil
	}

	if msg, ok := responseData["message"].(string); ok {
		fmt.Printf("Gateway: %s\n", msg)
	} else {
		fmt.Printf("MCP proxy %sd successfully\n", operation)
	}

	return nil
}

// resourceExists checks if a resource exists on the gateway by doing a GET request.
func resourceExists(client *gateway.Client, handler gateway.ResourceHandler, handle string) (bool, error) {
	endpoint := handler.GetEndpoint(handle)
	resp, err := client.Get(endpoint)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("unexpected status %d when checking MCP proxy existence", resp.StatusCode)
}

// extractMetadataName parses the YAML to get metadata.name.
func extractMetadataName(yamlContent []byte) (string, error) {
	// Simple line-based extraction to avoid importing yaml just for this
	lines := strings.Split(string(yamlContent), "\n")
	inMetadata := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}
		if inMetadata && strings.HasPrefix(trimmed, "name:") {
			name := strings.TrimPrefix(trimmed, "name:")
			name = strings.TrimSpace(name)
			// Remove surrounding quotes if any
			name = strings.Trim(name, "\"'")
			if name != "" {
				return name, nil
			}
		}
		// If we hit another top-level key after metadata, stop
		if inMetadata && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && trimmed != "" {
			break
		}
	}
	return "", fmt.Errorf("metadata.name not found in YAML")
}
