// Package clip 剪辑助手：调用本机 whisperx-align 技能完成文本转字幕。
package clip

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	pythonBin   = "/Users/bairdweng/whisperx-env/bin/python"
	alignScript = "/Users/bairdweng/skillbox/skills/whisperx-align/scripts/align.py"
)

// Env whisperx 对齐环境状态。
type Env struct {
	PythonReady bool   `json:"python_ready"`
	PythonPath  string `json:"python_path"`
	ScriptReady bool   `json:"script_ready"`
	ScriptPath  string `json:"script_path"`
	Ready       bool   `json:"ready"`
	Message     string `json:"message"`
}

// CheckEnv 检查本机 python 环境与对齐脚本是否就绪。
func CheckEnv() *Env {
	st := &Env{PythonPath: pythonBin, ScriptPath: alignScript}
	if _, err := os.Stat(pythonBin); err == nil {
		st.PythonReady = true
	}
	if _, err := os.Stat(alignScript); err == nil {
		st.ScriptReady = true
	}
	st.Ready = st.PythonReady && st.ScriptReady
	switch {
	case !st.PythonReady:
		st.Message = "未找到 whisperx Python 环境（~/whisperx-env）"
	case !st.ScriptReady:
		st.Message = "未找到对齐脚本（~/skillbox/skills/whisperx-align）"
	default:
		st.Message = "已就绪"
	}
	return st
}

// TextToSRT 把一行一句的文本对齐到音频，返回 SRT 内容与脚本输出。
func TextToSRT(ctx context.Context, audioPath, text, language string) (string, string, error) {
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
	}
	audioPath = strings.TrimSpace(audioPath)
	if audioPath == "" {
		return "", "", fmt.Errorf("音频文件不能为空")
	}
	if strings.TrimSpace(text) == "" {
		return "", "", fmt.Errorf("字幕文本不能为空")
	}
	if language == "" {
		language = "zh"
	}

	env := CheckEnv()
	if !env.Ready {
		return "", "", errors.New(env.Message)
	}

	dir, err := os.MkdirTemp("", "shopmind-clip-")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(dir)

	txtPath := filepath.Join(dir, "subtitle.txt")
	if err := os.WriteFile(txtPath, []byte(strings.ReplaceAll(text, "\r\n", "\n")), 0o644); err != nil {
		return "", "", err
	}
	srtPath := filepath.Join(dir, "aligned.srt")

	cmd := exec.CommandContext(ctx, pythonBin, alignScript,
		audioPath, "-s", txtPath, "-o", srtPath, "--language", language)
	out, _ := cmd.CombinedOutput()

	srt, readErr := os.ReadFile(srtPath)
	if readErr != nil {
		if len(strings.TrimSpace(string(out))) == 0 {
			return "", "", fmt.Errorf("对齐未生成字幕文件: %v", readErr)
		}
		return "", string(out), fmt.Errorf("对齐失败，请查看输出")
	}
	return string(srt), string(out), nil
}
