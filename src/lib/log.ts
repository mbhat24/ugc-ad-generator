type Fields = Record<string, unknown>;

let lastAlert = 0;
function alert(msg: string, f: Fields) {
  const url = process.env.ERROR_WEBHOOK_URL;
  if (!url || Date.now() - lastAlert < 30000) return;
  lastAlert = Date.now();
  const detail = f.err instanceof Error ? f.err.message : "";
  fetch(url, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ text: `AdReel error: ${msg} ${detail}`.slice(0, 500) }), signal: AbortSignal.timeout(5000) }).catch(() => {});
}

const emit = (level: string, msg: string, f: Fields = {}) => {
  const { err, ...rest } = f;
  const line = JSON.stringify({ t: new Date().toISOString(), level, msg, ...rest, ...(err ? { err: err instanceof Error ? { message: err.message, stack: err.stack } : err } : {}) });
  (level === "error" ? console.error : console.log)(line);
  if (level === "error") alert(msg, f);
};

export const log = {
  info: (msg: string, f?: Fields) => emit("info", msg, f),
  warn: (msg: string, f?: Fields) => emit("warn", msg, f),
  error: (msg: string, f?: Fields) => emit("error", msg, f),
};
