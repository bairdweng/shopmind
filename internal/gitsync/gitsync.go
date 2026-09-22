// Package gitsync 管理 data/ 数据仓的 Git 同步：
// 本地强制覆盖远端（push -f），远端覆盖本地（reset --hard）。
package gitsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"shopmind/internal/config"
)

type Status struct {
	GitMissing bool   `json:"git_missing"`
	IsRepo     bool   `json:"is_repo"`
	Branch     string `json:"branch"`
	Remote     string `json:"remote"`
	Dirty      bool   `json:"dirty"`
	Changed    int    `json:"changed"`
	Ahead      *int   `json:"ahead"`
	Behind     *int   `json:"behind"`
	LastCommit string `json:"last_commit"`
}

func run(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func dataDir() string {
	dir := config.DataDir()
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func isRepo(dir string) bool {
	// 必须是 data/ 自己的仓库；若向上找到主仓 .git 则视为未初始化，
	// 避免误操作主代码仓。
	out, err := run(context.Background(), dir, "rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	gd := strings.TrimSpace(out)
	if gd == "" {
		return false
	}
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(dir, gd)
	}
	return filepath.Clean(gd) == filepath.Clean(filepath.Join(dir, ".git"))
}

func Check(ctx context.Context) Status {
	var st Status
	if _, err := exec.LookPath("git"); err != nil {
		st.GitMissing = true
		return st
	}
	dir := dataDir()
	if !isRepo(dir) {
		return st
	}
	st.IsRepo = true

	if out, err := run(ctx, dir, "symbolic-ref", "--short", "HEAD"); err == nil {
		st.Branch = strings.TrimSpace(out)
	}
	if out, err := run(ctx, dir, "remote", "get-url", "origin"); err == nil {
		st.Remote = strings.TrimSpace(out)
	}
	if out, err := run(ctx, dir, "status", "--porcelain"); err == nil {
		lines := nonEmptyLines(out)
		st.Changed = len(lines)
		st.Dirty = len(lines) > 0
	}
	if out, err := run(ctx, dir, "log", "-1", "--format=%h %s（%ci）"); err == nil {
		st.LastCommit = strings.TrimSpace(out)
	}

	// 领先/落后需要先 fetch，失败不阻断状态展示
	_, _ = run(ctx, dir, "fetch", "origin")
	branch := st.Branch
	if branch == "" {
		branch = "main"
	}
	if out, err := run(ctx, dir, "rev-list", "--left-right", "--count", branch+"...origin/"+branch); err == nil {
		parts := strings.Fields(strings.TrimSpace(out))
		if len(parts) == 2 {
			a, err1 := strconv.Atoi(parts[0])
			b, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				st.Ahead = &a
				st.Behind = &b
			}
		}
	}
	return st
}

// InitRepo 初始化数据仓（已初始化则跳过）。
func InitRepo(ctx context.Context) (string, error) {
	dir := dataDir()
	if isRepo(dir) {
		return "数据仓已初始化", nil
	}
	return initRepo(ctx, dir)
}

func initRepo(ctx context.Context, dir string) (string, error) {
	out, err := run(ctx, dir, "init", "-b", "main")
	if err != nil {
		return out, fmt.Errorf("git init 失败: %s", strings.TrimSpace(out))
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		return out, err
	}
	return out, nil
}

// SetRemote 初始化（如未初始化）并设置 origin 远端地址。
func SetRemote(ctx context.Context, url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("远端地址不能为空")
	}
	dir := dataDir()
	if !isRepo(dir) {
		if _, err := initRepo(ctx, dir); err != nil {
			return "", err
		}
	}
	if out, err := run(ctx, dir, "remote", "get-url", "origin"); err == nil && strings.TrimSpace(out) == url {
		return "远端地址未变化", nil
	}
	if _, err := run(ctx, dir, "remote", "get-url", "origin"); err == nil {
		out, err := run(ctx, dir, "remote", "set-url", "origin", url)
		return out, err
	}
	out, err := run(ctx, dir, "remote", "add", "origin", url)
	return out, err
}

// PushForce 提交本地数据并强制覆盖远端。
func PushForce(ctx context.Context) (string, error) {
	dir := dataDir()
	if !isRepo(dir) {
		return "", fmt.Errorf("数据仓未初始化，请先保存远端地址")
	}
	var log strings.Builder
	if out, err := run(ctx, dir, "add", "-A"); err != nil {
		return out, fmt.Errorf("git add 失败: %w", err)
	} else if out != "" {
		log.WriteString(out)
	}
	if dirty(dir) {
		msg := fmt.Sprintf("shopmind 数据同步 %s", time.Now().Format("2006-01-02 15:04:05"))
		out, err := run(ctx, dir, "commit", "-m", msg)
		log.WriteString(out)
		if err != nil {
			return log.String(), fmt.Errorf("git commit 失败: %w", err)
		}
	} else if _, err := run(ctx, dir, "rev-parse", "HEAD"); err != nil {
		out, err := run(ctx, dir, "commit", "--allow-empty", "-m", "shopmind 数据同步（初始）")
		log.WriteString(out)
		if err != nil {
			return log.String(), fmt.Errorf("git commit 失败: %w", err)
		}
	}
	out, err := run(ctx, dir, "push", "-f", "origin", "HEAD:main")
	log.WriteString(out)
	if err != nil {
		return log.String(), fmt.Errorf("git push 失败: %w", err)
	}
	return log.String(), nil
}

// OverwriteLocal 用远端内容覆盖本地（fetch + reset --hard + clean）。
func OverwriteLocal(ctx context.Context) (string, error) {
	dir := dataDir()
	if !isRepo(dir) {
		return "", fmt.Errorf("数据仓未初始化，请先保存远端地址")
	}
	var log strings.Builder
	out, err := run(ctx, dir, "fetch", "origin")
	log.WriteString(out)
	if err != nil {
		return log.String(), fmt.Errorf("git fetch 失败: %w", err)
	}
	out, err = run(ctx, dir, "reset", "--hard", "FETCH_HEAD")
	log.WriteString(out)
	if err != nil {
		return log.String(), fmt.Errorf("git reset 失败: %w", err)
	}
	out, err = run(ctx, dir, "clean", "-fdx")
	log.WriteString(out)
	if err != nil {
		return log.String(), fmt.Errorf("git clean 失败: %w", err)
	}
	return log.String(), nil
}

func dirty(dir string) bool {
	out, err := run(context.Background(), dir, "status", "--porcelain")
	if err != nil {
		return false
	}
	return len(nonEmptyLines(out)) > 0
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
