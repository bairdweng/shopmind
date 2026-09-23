package clip

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// 编译期检查：确保 Plain2SRT 实现了 Tool 接口。
var _ Tool = (*Plain2SRT)(nil)

// Plain2SRT 纯文本转字幕插件：无音频，按语速/总时长估算时间轴生成 SRT 草稿。
type Plain2SRT struct{}

func init() { Register(&Plain2SRT{}) }

func (t *Plain2SRT) ID() string          { return "plain2srt" }
func (t *Plain2SRT) Name() string        { return "纯文本转字幕" }
func (t *Plain2SRT) Description() string { return "无音频时按语速或总时长估算时间轴，生成 SRT 草稿；有音频后可用「文本转字幕」精确对齐" }

func (t *Plain2SRT) CheckEnv() *Env { return &Env{Ready: true, Message: "无需外部环境"} }

// Run 表单：text 文本、mode 计时方式（speed|total|fixed）、wpm 语速（字/分）、
// total 总时长秒、fixed_each 每条秒数、start 起始偏移秒、gap 条间隔秒。
func (t *Plain2SRT) Run(ctx context.Context, r *http.Request) (map[string]interface{}, string, error) {
	// 兼容误贴 SRT：剥掉序号与时间戳，只保留台词。
	lines := cleanCueLines(r.FormValue("text"))
	if len(lines) == 0 {
		return nil, "", fmt.Errorf("没有可用的字幕行")
	}

	mode := r.FormValue("mode")
	wpm := formFloat(r, "wpm", 240)
	total := formFloat(r, "total", 0)
	fixedEach := formFloat(r, "fixed_each", 3)
	start := formFloat(r, "start", 0)
	gap := formFloat(r, "gap", 0.1)
	if start < 0 {
		start = 0
	}
	if gap < 0 {
		gap = 0
	}

	// 每行显示时长。
	const minDur = 0.8 // 短于这个数会闪一下
	durs := make([]float64, len(lines))
	switch mode {
	case "total":
		// 总时长去掉偏移与间隔后，按各行字数加权分配。
		if total <= 0 {
			return nil, "", fmt.Errorf("请填写大于 0 的总时长")
		}
		avail := total - start - gap*float64(len(lines)-1)
		if avail <= 0 {
			return nil, "", fmt.Errorf("总时长太短，装不下 %d 行字幕", len(lines))
		}
		weights := make([]float64, len(lines))
		sum := 0.0
		for i, ln := range lines {
			weights[i] = float64(len([]rune(ln)))
			sum += weights[i]
		}
		if sum == 0 {
			for i := range weights {
				weights[i] = 1
			}
			sum = float64(len(lines))
		}
		for i := range durs {
			durs[i] = avail * weights[i] / sum
		}
	case "fixed":
		if fixedEach <= 0 {
			return nil, "", fmt.Errorf("每条秒数必须大于 0")
		}
		for i := range durs {
			durs[i] = fixedEach
		}
	default: // speed
		if wpm <= 0 {
			return nil, "", fmt.Errorf("语速必须大于 0")
		}
		for i, ln := range lines {
			durs[i] = float64(len([]rune(ln))) / wpm * 60
		}
	}
	for i := range durs {
		if durs[i] < minDur {
			durs[i] = minDur
		}
	}

	var b strings.Builder
	cursor := start
	for i, ln := range lines {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, srtTime(cursor), srtTime(cursor+durs[i]), ln)
		cursor += durs[i] + gap
	}

	return map[string]interface{}{
		"text":     strings.TrimRight(b.String(), "\n"),
		"filename": "subtitle.srt",
	}, "", nil
}

// formFloat 读数值表单项，空或非法时用默认值。
func formFloat(r *http.Request, key string, def float64) float64 {
	s := strings.TrimSpace(r.FormValue(key))
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

// srtTime 秒 -> SRT 时间戳 "HH:MM:SS,mmm"。
func srtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	ms := int(sec * 1000)
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
