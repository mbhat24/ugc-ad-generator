// Package captions burns timed subtitles into videos with ffmpeg.
package captions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func stamp(t float64) string {
	ms := int(t*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// SRT splits text into cues of wordsPerCue words spread evenly over duration seconds.
func SRT(text string, duration float64, wordsPerCue int) string {
	words := strings.Fields(text)
	var cues []string
	for i := 0; i < len(words); i += wordsPerCue {
		end := min(i+wordsPerCue, len(words))
		cues = append(cues, strings.Join(words[i:end], " "))
	}
	step := duration / float64(max(len(cues), 1))
	var b strings.Builder
	for i, c := range cues {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, stamp(float64(i)*step), stamp(float64(i+1)*step), c)
	}
	return b.String()
}

// Burn writes dir/out.mp4 with captions rendered from text over dir/in.mp4.
func Burn(ctx context.Context, dir, text string) error {
	ctx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()
	probe := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", "in.mp4")
	probe.Dir = dir
	o, err := probe.Output()
	if err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}
	dur, _ := strconv.ParseFloat(strings.TrimSpace(string(o)), 64)
	if dur <= 0 {
		dur = 15
	}
	if err := os.WriteFile(filepath.Join(dir, "captions.srt"), []byte(SRT(text, dur, 4)), 0o600); err != nil {
		return err
	}
	style := "Alignment=2,FontSize=11,Bold=1,Outline=2,MarginV=70"
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", "in.mp4", "-vf", "subtitles=captions.srt:force_style='"+style+"'", "-c:a", "copy", "-movflags", "+faststart", "out.mp4")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(out[max(0, len(out)-300):])))
	}
	return nil
}
