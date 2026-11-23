import { test } from "node:test";
import assert from "node:assert/strict";
import { checkProduct, scrubClaims } from "../src/lib/moderation";

const p = (title: string, description = "") => ({ title, description, images: [] });

test("moderation blocks restricted categories and allows normal products", () => {
  assert.match(checkProduct(p("Tactical rifle scope")) ?? "", /weapons/);
  assert.match(checkProduct(p("Premium CBD gummies")) ?? "", /drugs/);
  assert.match(checkProduct(p("Miracle cure serum", "cures cancer")) ?? "", /health/);
  assert.equal(checkProduct(p("Merino wool running shoes", "Soft, breathable, machine washable")), null);
});

test("scrubClaims removes prohibited claims from generated copy", () => {
  const c = scrubClaims({ hook: "This cures acne", script: "Guaranteed results in a week, FDA approved", caption: "ok" });
  assert.doesNotMatch(`${c.hook} ${c.script}`, /cures|guaranteed results|fda approved/i);
});
