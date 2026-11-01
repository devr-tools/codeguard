package prompts

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// mcpServerContainers are the keys MCP clients nest server definitions under:
// mcpServers (Claude Code, Claude Desktop, Cursor, Gemini CLI), servers (VS
// Code), context_servers (Zed), mcp_servers (Codex-style YAML).
var mcpServerContainers = []string{"mcpServers", "servers", "context_servers", "mcp_servers"}

// mcpRemoteURLKeys are the fields that make a server remote rather than a
// locally spawned process.
var mcpRemoteURLKeys = []string{"url", "serverUrl", "httpUrl"}

// wildcardToolKeys and blanketApprovalKeys are the per-server and global
// knobs that pre-approve tool calls; a "*" or true there removes the human
// from every call the server exposes.
var (
	wildcardToolKeys    = map[string]struct{}{"allow": {}, "allowed_tools": {}, "allowedTools": {}, "tools": {}, "permissions": {}, "tool_permissions": {}, "trusted_tools": {}, "alwaysAllow": {}, "autoApprove": {}}
	blanketApprovalKeys = map[string]struct{}{"allowAllTools": {}, "autoApprove": {}, "skipApproval": {}, "skip_approval": {}, "bypassApproval": {}, "bypassApprovals": {}, "trust": {}}
	shellCommands       = map[string]struct{}{"bash": {}, "sh": {}, "zsh": {}, "dash": {}, "fish": {}, "cmd": {}, "powershell": {}, "pwsh": {}}
)

type mcpServer struct {
	name string
	node *configNode
}

// mcpStructuredFindings evaluates a parsed MCP config. Each server is
// checked for shell wrapping, unpinned package launches, literal secrets,
// and remote transport; the whole tree is checked for blanket approvals.
func mcpStructuredFindings(env support.Context, file string, root *configNode) []core.Finding {
	findings := blanketApprovalFindings(env, file, root, 0)
	for _, server := range collectMCPServers(root) {
		findings = append(findings, mcpServerFindings(env, file, server)...)
	}
	return findings
}

func collectMCPServers(root *configNode) []mcpServer {
	servers := make([]mcpServer, 0)
	var walk func(node *configNode, depth int)
	walk = func(node *configNode, depth int) {
		if node == nil || node.kind != nodeObject || depth > 4 {
			return
		}
		for _, container := range mcpServerContainers {
			group := node.field(container)
			if group == nil || group.kind != nodeObject {
				continue
			}
			for _, name := range group.keys {
				if child := group.fields[name]; looksLikeMCPServer(child) {
					servers = append(servers, mcpServer{name: name, node: child})
				}
			}
		}
		for _, key := range node.keys {
			if !isMCPServerContainer(key) {
				walk(node.fields[key], depth+1)
			}
		}
	}
	walk(root, 0)
	if len(servers) == 0 && looksLikeMCPServer(root) {
		servers = append(servers, mcpServer{name: "(root)", node: root})
	}
	return servers
}

func isMCPServerContainer(key string) bool {
	for _, container := range mcpServerContainers {
		if key == container {
			return true
		}
	}
	return false
}

func looksLikeMCPServer(node *configNode) bool {
	if node == nil || node.kind != nodeObject {
		return false
	}
	if _, ok := node.field("command").str(); ok {
		return true
	}
	return remoteURL(node) != nil
}

func remoteURL(node *configNode) *configNode {
	for _, key := range mcpRemoteURLKeys {
		if _, ok := node.field(key).str(); ok {
			return node.field(key)
		}
	}
	return nil
}

func mcpServerFindings(env support.Context, file string, server mcpServer) []core.Finding {
	findings := make([]core.Finding, 0)
	owner := fmt.Sprintf("MCP server %q", server.name)
	if commandNode := server.node.field("command"); commandNode != nil {
		command, _ := commandNode.str()
		tokens := append(strings.Fields(command), server.node.field("args").stringItems()...)
		if finding, risky := shellWrappedFinding(env, file, commandNode.line, owner, tokens); risky {
			findings = append(findings, finding)
		}
		if finding, unpinned := unpinnedServerFinding(env, file, commandNode.line, owner, tokens); unpinned {
			findings = append(findings, finding)
		}
	}
	findings = append(findings, envSecretFindings(env, file, owner, server.node.field("env"), "env")...)
	findings = append(findings, envSecretFindings(env, file, owner, server.node.field("headers"), "header")...)
	if urlNode := remoteURL(server.node); urlNode != nil {
		findings = append(findings, remoteServerFindings(env, file, owner, server.node, urlNode)...)
	}
	return findings
}

func shellWrappedFinding(env support.Context, file string, line int, owner string, tokens []string) (core.Finding, bool) {
	if len(tokens) == 0 {
		return core.Finding{}, false
	}
	var message string
	_, isShell := shellCommands[commandBase(tokens[0])]
	switch {
	case mcpPipeToShell.MatchString(strings.Join(tokens, " ")):
		message = fmt.Sprintf("%s downloads and pipes a script into a shell at launch; install a pinned, verified binary instead", owner)
	case isShell && len(tokens) > 1 && isShellEvalFlag(tokens[1]):
		message = fmt.Sprintf("%s runs through a shell (%s %s), so its real command line is opaque to review and open to injection; run the server binary directly", owner, commandBase(tokens[0]), tokens[1])
	default:
		return core.Finding{}, false
	}
	return settingsFinding(env, file, line, "prompts.mcp-config-risk", "fail", core.ConfidenceHigh, message), true
}

func isShellEvalFlag(flag string) bool {
	switch strings.ToLower(flag) {
	case "-c", "-lc", "-ic", "/c", "-command", "-encodedcommand":
		return true
	}
	return false
}

func unpinnedServerFinding(env support.Context, file string, line int, owner string, tokens []string) (core.Finding, bool) {
	launch, ok := detectPackageLaunch(tokens)
	if !ok || launch.pinned {
		return core.Finding{}, false
	}
	confidence := core.ConfidenceMedium
	detail := "without a pinned version"
	if launch.floatingTag {
		confidence = core.ConfidenceHigh
		detail = "at a floating tag"
	}
	message := fmt.Sprintf("%s launches %s package %q %s, so whatever is published next runs with the agent's privileges; pin an exact version", owner, launch.runner, launch.spec, detail)
	if launch.autoInstall {
		message += " (" + launch.autoInstallFlag + " also skips the install prompt)"
	}
	return settingsFinding(env, file, line, "prompts.mcp-unpinned-server", "warn", confidence, message), true
}

func remoteServerFindings(env support.Context, file string, owner string, server *configNode, urlNode *configNode) []core.Finding {
	raw, _ := urlNode.str()
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || isLoopbackHost(parsed.Hostname()) {
		return nil
	}
	if strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "ws") {
		return []core.Finding{settingsFinding(env, file, urlNode.line, "prompts.mcp-insecure-transport", "fail", core.ConfidenceHigh,
			fmt.Sprintf("%s connects to %s over unencrypted %s; tool calls, results and any credentials can be read or rewritten in transit — use https", owner, parsed.Host, strings.ToLower(parsed.Scheme)))}
	}
	if !requireRemoteAuth(env) || hasAuthHeader(server) {
		return nil
	}
	return []core.Finding{settingsFinding(env, file, urlNode.line, "prompts.mcp-remote-unauthenticated", "warn", core.ConfidenceLow,
		fmt.Sprintf("%s connects to remote host %s with no Authorization or API-key header; confirm the server enforces OAuth, or add an auth header that references an environment variable", owner, parsed.Host))}
}

func requireRemoteAuth(env support.Context) bool {
	if env.Config.Checks.PromptRules.RequireMCPRemoteAuth == nil {
		return true
	}
	return *env.Config.Checks.PromptRules.RequireMCPRemoteAuth
}

func hasAuthHeader(server *configNode) bool {
	if server.field("oauth") != nil || server.field("auth") != nil || server.field("authProviderType") != nil {
		return true
	}
	for _, key := range []string{"headers", "requestInit"} {
		headers := server.field(key)
		if key == "requestInit" {
			headers = headers.field("headers")
		}
		if headers == nil || headers.kind != nodeObject {
			continue
		}
		for _, name := range headers.keys {
			lower := strings.ToLower(name)
			if strings.Contains(lower, "auth") || strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "cookie") {
				return true
			}
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// blanketApprovalFindings walks the whole document for wildcard tool
// allowlists and approval-bypass booleans, wherever the client nests them.
func blanketApprovalFindings(env support.Context, file string, node *configNode, depth int) []core.Finding {
	if node == nil || depth > maxConfigDepth {
		return nil
	}
	findings := make([]core.Finding, 0)
	switch node.kind {
	case nodeObject:
		for _, key := range node.keys {
			child := node.fields[key]
			if _, ok := wildcardToolKeys[key]; ok && child.kind == nodeArray {
				for _, item := range child.items {
					if value, _ := item.str(); value == "*" {
						findings = append(findings, settingsFinding(env, file, item.line, "prompts.mcp-config-risk", "fail", core.ConfidenceHigh,
							fmt.Sprintf("%q allows every MCP tool without approval; list the specific tools the agent needs", key)))
					}
				}
			}
			if _, ok := blanketApprovalKeys[key]; ok && child.isTrue() {
				findings = append(findings, settingsFinding(env, file, child.line, "prompts.mcp-config-risk", "fail", core.ConfidenceHigh,
					fmt.Sprintf("%q is true, so MCP tool calls run without per-call approval", key)))
			}
			findings = append(findings, blanketApprovalFindings(env, file, child, depth+1)...)
		}
	case nodeArray:
		for _, item := range node.items {
			findings = append(findings, blanketApprovalFindings(env, file, item, depth+1)...)
		}
	}
	return findings
}

func commandBase(command string) string {
	base := strings.ToLower(path.Base(strings.ReplaceAll(command, "\\", "/")))
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".ps1"} {
		base = strings.TrimSuffix(base, suffix)
	}
	return base
}
