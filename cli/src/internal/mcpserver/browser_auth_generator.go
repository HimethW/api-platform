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
	"sort"
	"strings"
)

// GenerateBrowserAuthProject generates a multi-file Python MCP server project
// with browser OAuth challenge support and PRM endpoint support.
func GenerateBrowserAuthProject(spec *ArazzoSpec, arazzoFileName string, port int, authServerURL string) (map[string]string, error) {
	if len(spec.Workflows) == 0 {
		return nil, fmt.Errorf("no workflows found in Arazzo spec to generate tools from")
	}
	if strings.TrimSpace(authServerURL) == "" {
		return nil, fmt.Errorf("auth server URL cannot be empty in browser-auth mode")
	}

	toolsCode, err := generateBrowserAuthToolsCode(spec, arazzoFileName)
	if err != nil {
		return nil, err
	}
	authCode := generateBrowserAuthMiddlewareCode(port, authServerURL)
	mainCode := generateBrowserAuthMainCode(port, authServerURL)
	requirements := generateBrowserAuthRequirements()

	return map[string]string{
		"requirements.txt": requirements,
		"src/__init__.py":  "",
		"src/tools.py":     toolsCode,
		"src/auth.py":      authCode,
		"src/main.py":      mainCode,
	}, nil
}

func generateBrowserAuthRequirements() string {
	return strings.Join([]string{
		"fastmcp",
		"arazzo-runner",
		"fastapi",
		"uvicorn",
		"requests",
		"urllib3",
		"",
	}, "\n")
}

func generateBrowserAuthMainCode(port int, authServerURL string) string {
	var b strings.Builder
	b.WriteString("from .tools import mcp\n")
	b.WriteString("from .auth import configure_browser_auth\n")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("APP_PORT = %d\n", port))
	b.WriteString(fmt.Sprintf("AUTH_SERVER_URL = %q\n", authServerURL))
	b.WriteString("\n")
	b.WriteString("HTTP_MIDDLEWARE = configure_browser_auth(mcp, APP_PORT, AUTH_SERVER_URL)\n")
	b.WriteString("\n")
	b.WriteString("if __name__ == \"__main__\":\n")
	b.WriteString("    mcp.run(\n")
	b.WriteString("        transport=\"http\",\n")
	b.WriteString("        host=\"0.0.0.0\",\n")
	b.WriteString("        port=APP_PORT,\n")
	b.WriteString("        stateless_http=True,\n")
	b.WriteString("        middleware=HTTP_MIDDLEWARE,\n")
	b.WriteString("    )\n")
	return b.String()
}

func generateBrowserAuthMiddlewareCode(port int, authServerURL string) string {
	var b strings.Builder
	b.WriteString("from fastapi import Request\n")
	b.WriteString("from fastapi.responses import JSONResponse, Response\n")
	b.WriteString("from starlette.middleware import Middleware\n")
	b.WriteString("from starlette.middleware.base import BaseHTTPMiddleware\n")
	b.WriteString("\n")
	b.WriteString("from fastmcp import FastMCP\n")
	b.WriteString("\n")
	b.WriteString("from .tools import reset_request_bearer_token, set_request_bearer_token\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("PRM_SUFFIX = \"/.well-known/oauth-protected-resource\"\n")
	b.WriteString("PROTECTED_PATH_PARTS = {\"mcp\", \"sse\", \"events\"}\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _first_header_value(value: str) -> str:\n")
	b.WriteString("    if not value:\n")
	b.WriteString("        return \"\"\n")
	b.WriteString("    return value.split(\",\", 1)[0].strip()\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _normalize_prefix(prefix: str) -> str:\n")
	b.WriteString("    prefix = (prefix or \"\").strip()\n")
	b.WriteString("    if not prefix or prefix == \"/\":\n")
	b.WriteString("        return \"\"\n")
	b.WriteString("    if not prefix.startswith(\"/\"):\n")
	b.WriteString("        prefix = \"/\" + prefix\n")
	b.WriteString("    return prefix.rstrip(\"/\")\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _split_path_parts(path: str) -> list[str]:\n")
	b.WriteString("    return [part for part in (path or \"\").split(\"/\") if part]\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _extract_prefix_from_path(path: str) -> str:\n")
	b.WriteString("    parts = _split_path_parts(path)\n")
	b.WriteString("    for idx, part in enumerate(parts):\n")
	b.WriteString("        if part in PROTECTED_PATH_PARTS:\n")
	b.WriteString("            return \"\" if idx == 0 else \"/\" + \"/\".join(parts[:idx])\n")
	b.WriteString("        if part == \".well-known\" and idx + 1 < len(parts) and parts[idx + 1] == \"oauth-protected-resource\":\n")
	b.WriteString("            return \"\" if idx == 0 else \"/\" + \"/\".join(parts[:idx])\n")
	b.WriteString("    return \"\"\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _public_origin(request: Request, fallback_port: int) -> str:\n")
	b.WriteString("    forwarded_proto = _first_header_value(request.headers.get(\"X-Forwarded-Proto\", \"\"))\n")
	b.WriteString("    forwarded_host = _first_header_value(request.headers.get(\"X-Forwarded-Host\", \"\"))\n")
	b.WriteString("    forwarded_prefix = _normalize_prefix(_first_header_value(request.headers.get(\"X-Forwarded-Prefix\", \"\")))\n")
	b.WriteString("\n")
	b.WriteString("    scheme = forwarded_proto or request.url.scheme or \"http\"\n")
	b.WriteString("    host = forwarded_host or request.headers.get(\"host\", \"\") or request.url.netloc\n")
	b.WriteString("    if not host:\n")
	b.WriteString("        host = f\"localhost:{fallback_port}\"\n")
	b.WriteString("\n")
	b.WriteString("    prefix = forwarded_prefix or _extract_prefix_from_path(request.url.path or \"\")\n")
	b.WriteString("    return f\"{scheme}://{host}{prefix}\"\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _prm_url_for_request(request: Request, fallback_port: int) -> str:\n")
	b.WriteString("    return _public_origin(request, fallback_port) + PRM_SUFFIX\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _resource_url_for_request(request: Request, fallback_port: int) -> str:\n")
	b.WriteString("    return _public_origin(request, fallback_port) + \"/mcp\"\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _extract_bearer_token(authorization_header: str) -> str:\n")
	b.WriteString("    if not authorization_header:\n")
	b.WriteString("        return \"\"\n")
	b.WriteString("    parts = authorization_header.split(\" \", 1)\n")
	b.WriteString("    if len(parts) != 2:\n")
	b.WriteString("        return \"\"\n")
	b.WriteString("    if parts[0].lower() != \"bearer\":\n")
	b.WriteString("        return \"\"\n")
	b.WriteString("    return parts[1].strip()\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _is_protected_mcp_request(path: str) -> bool:\n")
	b.WriteString("    for part in _split_path_parts(path):\n")
	b.WriteString("        if part in PROTECTED_PATH_PARTS:\n")
	b.WriteString("            return True\n")
	b.WriteString("    return False\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _is_prm_request(path: str) -> bool:\n")
	b.WriteString("    return (path or \"\") == PRM_SUFFIX or (path or \"\").endswith(PRM_SUFFIX)\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("class BrowserAuthMiddleware(BaseHTTPMiddleware):\n")
	b.WriteString("    def __init__(self, app, fallback_port: int, auth_server_url: str):\n")
	b.WriteString("        super().__init__(app)\n")
	b.WriteString("        self.fallback_port = fallback_port\n")
	b.WriteString("        self.auth_server_url = auth_server_url\n")
	b.WriteString("\n")
	b.WriteString("    async def dispatch(self, request: Request, call_next):\n")
	b.WriteString("        path = request.url.path or \"\"\n")
	b.WriteString("\n")
	b.WriteString("        if _is_prm_request(path):\n")
	b.WriteString("            if request.method != \"GET\":\n")
	b.WriteString("                return Response(status_code=405)\n")
	b.WriteString("            return JSONResponse({\n")
	b.WriteString("                \"resource\": _resource_url_for_request(request, self.fallback_port),\n")
	b.WriteString("                \"authorization_servers\": [self.auth_server_url],\n")
	b.WriteString("            })\n")
	b.WriteString("\n")
	b.WriteString("        if not _is_protected_mcp_request(path):\n")
	b.WriteString("            return await call_next(request)\n")
	b.WriteString("\n")
	b.WriteString("        token = _extract_bearer_token(request.headers.get(\"Authorization\", \"\"))\n")
	b.WriteString("        if not token:\n")
	b.WriteString("            prm_url = _prm_url_for_request(request, self.fallback_port)\n")
	b.WriteString("            return Response(\n")
	b.WriteString("                status_code=401,\n")
	b.WriteString("                headers={\n")
	b.WriteString("                    \"WWW-Authenticate\": f'Bearer realm=\"mcp\", resource_metadata=\"{prm_url}\"'\n")
	b.WriteString("                },\n")
	b.WriteString("            )\n")
	b.WriteString("\n")
	b.WriteString("        state = set_request_bearer_token(token)\n")
	b.WriteString("        try:\n")
	b.WriteString("            return await call_next(request)\n")
	b.WriteString("        finally:\n")
	b.WriteString("            reset_request_bearer_token(state)\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def configure_browser_auth(mcp: FastMCP, port: int, auth_server_url: str):\n")
	b.WriteString("    return [Middleware(BrowserAuthMiddleware, fallback_port=port, auth_server_url=auth_server_url)]\n")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("# Defaults generated for this build: port=%d, auth_server_url=%q\n", port, authServerURL))
	return b.String()
}

func generateBrowserAuthToolsCode(spec *ArazzoSpec, arazzoFileName string) (string, error) {
	if len(spec.Workflows) == 0 {
		return "", fmt.Errorf("no workflows found in Arazzo spec to generate tools from")
	}

	workflowClassified := make(map[string]ClassifiedInputs)
	for _, wf := range spec.Workflows {
		workflowClassified[wf.WorkflowID] = ClassifyInputs(wf)
	}

	var b strings.Builder
	b.WriteString("import contextvars\n")
	b.WriteString("import requests\n")
	b.WriteString("import urllib3\n")
	b.WriteString("from urllib.parse import urlparse\n")
	b.WriteString("\n")
	b.WriteString("from fastmcp import FastMCP\n")
	b.WriteString("from arazzo_runner import ArazzoRunner\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("_current_bearer_token: contextvars.ContextVar[str] = contextvars.ContextVar(\n")
	b.WriteString("    \"current_bearer_token\", default=\"\"\n")
	b.WriteString(")\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def set_request_bearer_token(token: str):\n")
	b.WriteString("    return _current_bearer_token.set(token or \"\")\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def reset_request_bearer_token(state) -> None:\n")
	b.WriteString("    _current_bearer_token.reset(state)\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString("def _get_request_bearer_token() -> str:\n")
	b.WriteString("    return _current_bearer_token.get()\n")
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("mcp = FastMCP(%q)\n", spec.Info.Title))
	b.WriteString("\n")
	b.WriteString("_http = requests.Session()\n")
	b.WriteString("_http.verify = False\n")
	b.WriteString("urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)\n")
	b.WriteString(fmt.Sprintf("runner = ArazzoRunner.from_arazzo_path(\"./arazzo/%s\", http_client=_http)\n", arazzoFileName))
	b.WriteString("\n")

	if hasRemoteSourceDescriptions(spec) {
		b.WriteString("# Resolve relative server URLs in remote source descriptions\n")
		for _, sd := range spec.SourceDescriptions {
			if strings.HasPrefix(sd.URL, "http://") || strings.HasPrefix(sd.URL, "https://") {
				b.WriteString(fmt.Sprintf("if %q in runner.source_descriptions:\n", sd.Name))
				b.WriteString(fmt.Sprintf("    _parsed = urlparse(%q)\n", sd.URL))
				b.WriteString("    _base = f\"{_parsed.scheme}://{_parsed.netloc}\"\n")
				b.WriteString(fmt.Sprintf("    for _srv in runner.source_descriptions[%q].get(\"servers\", []):\n", sd.Name))
				b.WriteString("        if _srv.get(\"url\", \"\") and not _srv[\"url\"].startswith(\"http\"):\n")
				b.WriteString("            _srv[\"url\"] = _base + _srv[\"url\"]\n")
			}
		}
		b.WriteString("\n")
	}

	for i, wf := range spec.Workflows {
		if i > 0 {
			b.WriteString("\n")
		}
		classified := workflowClassified[wf.WorkflowID]
		params := buildParamsFromMap(classified.RegularInputs)
		inputDict := buildInputDictWithRuntimeToken(classified.RegularInputs, classified.CredentialInputs)

		b.WriteString(fmt.Sprintf("# Tool %d: %s workflow\n", i+1, wf.WorkflowID))
		b.WriteString("@mcp.tool()\n")
		b.WriteString(fmt.Sprintf("async def %s(%s) -> str:\n", camelToSnake(wf.WorkflowID), params))
		b.WriteString(fmt.Sprintf("    \"\"\"%s\"\"\"\n", workflowDocstring(wf)))
		b.WriteString("    try:\n")
		b.WriteString("        _token = _get_request_bearer_token()\n")
		b.WriteString(fmt.Sprintf("        result = runner.execute_workflow(%q, {%s})\n", wf.WorkflowID, inputDict))
		b.WriteString("        if result.outputs:\n")
		b.WriteString("            return f\"Workflow Success. Outputs: {result.outputs}\"\n")
		b.WriteString("        return f\"Workflow Result: {result}\"\n")
		b.WriteString("    except Exception as e:\n")
		b.WriteString("        return f\"Workflow Error: {str(e)}\"\n")
	}

	return b.String(), nil
}

func buildInputDictWithRuntimeToken(regular map[string]InputProperty, credentials map[string]InputProperty) string {
	var parts []string
	for name := range regular {
		parts = append(parts, fmt.Sprintf("%q: %s", name, name))
	}
	for name := range credentials {
		parts = append(parts, fmt.Sprintf("%q: _token", name))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}
