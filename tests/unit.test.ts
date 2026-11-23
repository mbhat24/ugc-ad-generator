import { test } from "node:test";
import assert from "node:assert/strict";
import { buildSrt } from "../src/lib/captions";
import { scrapeProduct } from "../src/lib/scrape";

test("buildSrt splits words into evenly timed cues", () => {
  const srt = buildSrt("one two three four five six seven eight", 8);
  const cues = srt.trim().split("\n\n");
  assert.equal(cues.length, 2);
  assert.match(cues[0], /^1\n00:00:00,000 --> 00:00:04,000\none two three four$/);
  assert.match(cues[1], /^2\n00:00:04,000 --> 00:00:08,000\nfive six seven eight$/);
});

test("scraper blocks private and non-http targets", async () => {
  for (const u of ["http://127.0.0.1/", "http://localhost:5432", "http://10.0.0.5/", "http://169.254.169.254/latest/meta-data", "http://[::1]/", "http://[::ffff:127.0.0.1]/", "http://192.168.1.1/", "ftp://example.com/"]) {
    await assert.rejects(scrapeProduct(u), /public website|http/, u);
  }
});
