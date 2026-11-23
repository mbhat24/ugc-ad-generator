import { test } from "node:test";
import assert from "node:assert/strict";
import { writeFile, mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const live = !!process.env.S3_BUCKET && !!process.env.S3_ENDPOINT;

test("storage: upload, signed download, delete prefix", { skip: !live }, async () => {
  const { uploadVideo, signedVideoUrl, deletePrefix } = await import("../src/lib/storage");
  const f = join(await mkdtemp(join(tmpdir(), "st-")), "v.mp4");
  await writeFile(f, "fake-video-bytes");
  const prefix = `videos/test-${Date.now()}/`;
  const key = await uploadVideo(f, `${prefix}a.mp4`);
  assert.equal(key, `${prefix}a.mp4`);
  const res = await fetch(await signedVideoUrl(key));
  assert.equal(await res.text(), "fake-video-bytes");
  await deletePrefix(prefix);
  assert.equal((await fetch(await signedVideoUrl(key))).status, 404);
});
