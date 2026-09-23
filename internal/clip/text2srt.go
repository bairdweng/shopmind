package clip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// 编译期检查：确保 Text2SRT 实现了 Tool 接口。
var _ Tool = (*Text2SRT)(nil)

// Text2SRT 文本转字幕插件。
type Text2SRT struct{}

func init() { Register(&Text2SRT{}) }

func (t *Text2SRT) ID() string          { return "text2srt" }
func (t *Text2SRT) Name() string        { return "文本转字幕" }
func (t *Text2SRT) Description() string { return "把一行一句的文案对齐到音频时间轴，生成 SRT 字幕文件" }

func (t *Text2SRT) CheckEnv() *Env { return whisperxEnv() }

// Run 接收已解析的 multipart 表单：audio 音频文件、text 字幕文本、language 语言。
func (t *Text2SRT) Run(ctx context.Context, r *http.Request) (map[string]interface{}, string, error) {
	file, fh, err := r.FormFile("audio")
	if err != nil {
		return nil, "", errors.New("缺少音频文件")
	}
	defer file.Close()

	suffix := ".mp3"
	if filepath.Ext(fh.Filename) != "" {
		suffix = strings.ToLower(filepath.Ext(fh.Filename))
	}
	tmp, err := os.CreateTemp("", "shopmind-audio-*"+suffix)
	if err != nil {
		return nil, "", err
	}
	audioPath := tmp.Name()
	defer os.Remove(audioPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		return nil, "", fmt.Errorf("保存音频失败: %w", err)
	}
	tmp.Close()

	text := strings.TrimSpace(r.FormValue("text"))
	// 兼容误贴 SRT：自动剥掉序号与时间戳，只对齐台词。
	if cues := cleanCueLines(text); len(cues) > 0 {
		text = strings.Join(cues, "\n")
	}
	language := strings.TrimSpace(r.FormValue("language"))

	srt, output, err := alignToSRT(ctx, audioPath, text, language)
	if err != nil {
		return nil, output, err
	}

	// 下载文件名沿用音频名，如 lecture.mp3 -> lecture.srt。
	filename := "subtitle.srt"
	if base := strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename)); base != "" {
		filename = base + ".srt"
	}
	return map[string]interface{}{"text": srt, "filename": filename}, output, nil
}

// alignToSRT 调用 whisperx 对齐脚本，把一行一句的文本对齐到音频生成 SRT。
func alignToSRT(ctx context.Context, audioPath, text, language string) (string, string, error) {
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

	python, script := resolve()
	env := whisperxEnv()
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

	cmd := exec.CommandContext(ctx, python, script,
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
