package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"shopmind/internal/config"
	"shopmind/internal/vault"
)

const (
	binName       = "agent"
	apiKeyEnv     = "CURSOR_API_KEY"
	agentBinEnv   = "CURSOR_AGENT_BIN"
	defaultModel  = "auto"
	installScript = "curl https://cursor.com/install -fsS | bash"
	testPrompt    = "Reply with exactly: shopmind-ok"
)

type EnvStatus struct {
	AgentInstalled   bool   `json:"agent_installed"`
	AgentPath        string `json:"agent_path"`
	AgentVersion     string `json:"agent_version"`
	APIKeyConfigured bool   `json:"api_key_configured"`
	Ready            bool   `json:"ready"`
	Message          string `json:"message"`
}

func CheckEnv(p vault.Payload) *EnvStatus {
	st := &EnvStatus{
		APIKeyConfigured: apiKey(p) != "",
	}
	bin, err := ResolveBin(p)
	if err == nil {
		st.AgentInstalled = true
		st.AgentPath = bin
		st.AgentVersion = version(bin, p)
	}
	st.Ready = st.AgentInstalled && st.APIKeyConfigured
	switch {
	case !st.AgentInstalled:
		st.Message = "未安装 Cursor CLI，请点击「检测并安装」"
	case !st.APIKeyConfigured:
		st.Message = "请填写 Cursor API Key"
	default:
		st.Message = "已就绪"
	}
	return st
}

func InstallCLI(ctx context.Context) (string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", installScript)
	cmd.Env = buildEnv(vault.Payload{})
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return out, fmt.Errorf("安装失败: %w", err)
	}
	return out, nil
}

func TestConnection(ctx context.Context, p vault.Payload) (string, error) {
	bin, err := ResolveBin(p)
	if err != nil {
		return "", err
	}
	if apiKey(p) == "" {
		return "", fmt.Errorf("未配置 Cursor API Key")
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "--trust", "--yolo", "--model", defaultModel, testPrompt)
	cmd.Dir = config.Root()
	cmd.Env = buildEnv(p)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	if err != nil {
		if out == "" {
			out = err.Error()
		}
		return out, fmt.Errorf("连接测试失败: %w", err)
	}
	return out, nil
}

func ResolveBin(p vault.Payload) (string, error) {
	if bin := strings.TrimSpace(p.CursorAgentBin); bin != "" && isExec(bin) {
		return bin, nil
	}
	if bin := strings.TrimSpace(os.Getenv(agentBinEnv)); bin != "" && isExec(bin) {
		return bin, nil
	}
	pathEnv := os.Getenv("PATH")
	for _, e := range buildEnv(p) {
		if strings.HasPrefix(e, "PATH=") {
			pathEnv = strings.TrimPrefix(e, "PATH=")
			break
		}
	}
	for _, dir := range strings.Split(pathEnv, ":") {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, binName)
		if isExec(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到 cursor agent，请在设置页安装 Cursor CLI 或配置 agent 路径")
}

func apiKey(p vault.Payload) string {
	if key := strings.TrimSpace(p.CursorAPIKey); key != "" {
		return key
	}
	return strings.TrimSpace(os.Getenv(apiKeyEnv))
}

func buildEnv(p vault.Payload) []string {
	env := os.Environ()
	if key := apiKey(p); key != "" {
		env = append(env, apiKeyEnv+"="+key)
	}
	home, _ := os.UserHomeDir()
	localBin := filepath.Join(home, ".local", "bin")
	cursorBin := filepath.Join(home, ".cursor", "bin")
	path := os.Getenv("PATH")
	if path == "" {
		path = localBin + ":" + cursorBin
	} else {
		path = localBin + ":" + cursorBin + ":" + path
	}
	env = append(env, "PATH="+path)
	return env
}

func version(bin string, p vault.Payload) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "--version")
	cmd.Env = buildEnv(p)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isExec(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
