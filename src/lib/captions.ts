import { execFile } from "child_process";
import { promisify } from "util";
import { writeFile } from "fs/promises";

const run = promisify(execFile);

const stamp = (t: number) => {
  const ms = Math.round(t * 1000);
  const p = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${p(Math.floor(ms / 3600000))}:${p(Math.floor(ms / 60000) % 60)}:${p(Math.floor(ms / 1000) % 60)},${p(ms % 1000, 3)}`;
};

export function buildSrt(text: string, duration: number, wordsPerCue = 4) {
  const words = text.split(/\s+/).filter(Boolean);
  const cues: string[] = [];
  for (let i = 0; i < words.length; i += wordsPerCue) cues.push(words.slice(i, i + wordsPerCue).join(" "));
  const step = duration / Math.max(cues.length, 1);
  return cues.map((c, i) => `${i + 1}\n${stamp(i * step)} --> ${stamp((i + 1) * step)}\n${c}\n`).join("\n");
}

export async function burnCaptions(dir: string, input: string, output: string, text: string) {
  const { stdout } = await run("ffprobe", ["-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", input], { cwd: dir });
  const duration = Number(stdout.trim()) || 15;
  await writeFile(`${dir}/captions.srt`, buildSrt(text, duration));
  const style = "Alignment=2,FontSize=11,Bold=1,Outline=2,MarginV=70";
  await run("ffmpeg", ["-y", "-i", input, "-vf", `subtitles=captions.srt:force_style='${style}'`, "-c:a", "copy", "-movflags", "+faststart", output], { cwd: dir, timeout: 120000 });
}
