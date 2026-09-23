// Package clip 剪辑助手插件系统：一个工具一个文件，实现 Tool 接口并在
// init 中 Register 即可接入，路由与前端外壳无需改动。
package clip

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tool 插件接口。r 的 multipart 表单已由 server 统一解析，插件内直接用
// r.FormFile / r.FormValue 取输入。
//
// Run 返回的 result 会原样并入响应 JSON，约定键：
//   - text     主输出预览内容（前端展示并作为下载内容）
//   - filename 下载文件名
//   其余键前端会以 key-value 形式展示。output 为执行日志。
type Tool interface {
	ID() string
	Name() string
	Description() string
	CheckEnv() *Env
	Run(ctx context.Context, r *http.Request) (result map[string]interface{}, output string, err error)
}

var registry = map[string]Tool{}

// Register 注册工具，ID 冲突直接 panic——这类错误要在启动时暴露。
func Register(t Tool) {
	if _, dup := registry[t.ID()]; dup {
		panic("clip: 重复注册工具 " + t.ID())
	}
	registry[t.ID()] = t
}

// GetTool 按 ID 取工具。
func GetTool(id string) Tool { return registry[id] }

// ListTools 返回所有已注册工具，按 ID 排序保证顺序稳定。
func ListTools() []Tool {
	ids := make([]string, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	tools := make([]Tool, 0, len(ids))
	for _, id := range ids {
		tools = append(tools, registry[id])
	}
	return tools
}

// ---------------------------------------------------------------------------
// 共享环境探测：目前是 whisperx 对齐环境，各插件 CheckEnv 里直接调用。
// ---------------------------------------------------------------------------

const (
	envPython = "SHOPMIND_WHISPERX_PYTHON"
	envScript = "SHOPMIND_WHISPERX_ALIGN_SCRIPT"

	// 各机器 skillbox 内对齐脚本的相对路径。
	relAlignScript = "skillbox/skills/whisperx-align/scripts/align.py"
)

// Env 工具环境状态。
type Env struct {
	PythonReady bool   `json:"python_ready"`
	PythonPath  string `json:"python_path"`
	ScriptReady bool   `json:"script_ready"`
	ScriptPath  string `json:"script_path"`
	Ready       bool   `json:"ready"`
	Message     string `json:"message"`
}

var (
	resolveOnce sync.Once
	cachedPython string
	cachedScript string
)

// resolve 解析本机可用的 whisperx python 与对齐脚本，进程内只探测一次。
func resolve() (python, script string) {
	resolveOnce.Do(func() {
		cachedPython = findPython()
		cachedScript = findScript()
	})
	return cachedPython, cachedScript
}

// whisperxEnv 检查本机 python 环境与对齐脚本是否就绪。
func whisperxEnv() *Env {
	python, script := resolve()
	st := &Env{
		PythonPath:  python,
		ScriptPath:  script,
		PythonReady: python != "",
		ScriptReady: script != "",
	}
	st.Ready = st.PythonReady && st.ScriptReady
	switch {
	case !st.PythonReady:
		st.Message = fmt.Sprintf(
			"未找到可用的 whisperx Python 环境（已探测 ~/.venvs/whisperx、~/whisperx-env 及 PATH，可用环境变量 %s 指定）",
			envPython)
	case !st.ScriptReady:
		st.Message = fmt.Sprintf(
			"未找到对齐脚本（期望 ~/%s，可用环境变量 %s 指定）",
			relAlignScript, envScript)
	default:
		st.Message = "已就绪"
	}
	return st
}

// findPython 依次按 环境变量 -> 常见 venv 位置 -> PATH 上的 python3 探测，
// 候选解释器必须能成功 import whisperx 才算可用。
func findPython() string {
	home, _ := os.UserHomeDir()

	if p := strings.TrimSpace(os.Getenv(envPython)); p != "" {
		// 显式指定时不静默回退，让错误直接暴露。
		p = expandHome(p, home)
		if pythonUsable(p) {
			return p
		}
		return ""
	}

	candidates := []string{
		filepath.Join(home, ".venvs", "whisperx", "bin", "python"),
		filepath.Join(home, "whisperx-env", "bin", "python"),
	}
	for _, c := range candidates {
		if pythonUsable(c) {
			return c
		}
	}
	// 退而求其次：PATH 上能 import whisperx 的 python3。
	if p, err := exec.LookPath("python3"); err == nil && pythonUsable(p) {
		return p
	}
	return ""
}

// findScript 依次按 环境变量 -> 本机 skillbox 常见位置 探测对齐脚本。
func findScript() string {
	home, _ := os.UserHomeDir()

	if p := strings.TrimSpace(os.Getenv(envScript)); p != "" {
		p = expandHome(p, home)
		if fileExists(p) {
			return p
		}
		return ""
	}

	p := filepath.Join(home, relAlignScript)
	if fileExists(p) {
		return p
	}
	return ""
}

func pythonUsable(path string) bool {
	if !fileExists(path) {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, path, "-c", "import whisperx").Run(); err != nil {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// 字幕文本清洗
// ---------------------------------------------------------------------------

// srtTimeLine 匹配 SRT/VTT 时间戳行，如 00:00:02,100 --> 00:00:03,850。
var srtTimeLine = regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}[,.]\d{3}\s*-->`)

// cleanCueLines 把输入整理成纯台词行（一行一句）。
// 检测到时间戳行时判定输入是 SRT/VTT，剥掉序号行、时间戳行和空行；
// 否则只去掉空行——纯数字台词不会被误删。
func cleanCueLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	isSRT := false
	for _, ln := range raw {
		if srtTimeLine.MatchString(strings.TrimSpace(ln)) {
			isSRT = true
			break
		}
	}
	out := make([]string, 0, len(raw))
	for _, ln := range raw {
		s := strings.TrimSpace(ln)
		if s == "" {
			continue
		}
		if isSRT {
			if srtTimeLine.MatchString(s) {
				continue
			}
			if _, err := strconv.Atoi(s); err == nil {
				continue // SRT 序号
			}
		}
		out = append(out, s)
	}
	return out
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func expandHome(path, home string) string {
	switch {
	case path == "~":
		return home
	case strings.HasPrefix(path, "~/"):
		return filepath.Join(home, path[2:])
	default:
		return path
	}
}
