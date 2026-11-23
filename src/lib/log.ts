type Fields = Record<string, unknown>;

const emit = (level: string, msg: string, f: Fields = {}) => {
  const { err, ...rest } = f;
  const line = JSON.stringify({ t: new Date().toISOString(), level, msg, ...rest, ...(err ? { err: err instanceof Error ? { message: err.message, stack: err.stack } : err } : {}) });
  (level === "error" ? console.error : console.log)(line);
};

export const log = {
  info: (msg: string, f?: Fields) => emit("info", msg, f),
  warn: (msg: string, f?: Fields) => emit("warn", msg, f),
  error: (msg: string, f?: Fields) => emit("error", msg, f),
};
