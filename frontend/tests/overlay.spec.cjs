const { test, expect } = require("@playwright/test");

test("preview runs the full pattern, stops, and can be cancelled", async ({
  page,
}) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  await expect(page.locator("#status")).toContainText("Browser preview");
  await page.locator("#voice-toggle").uncheck();
  await page.locator("#preview").click();
  await expect(page.locator("#preview")).toContainText("Stop");
  await expect(page.locator("#left")).toHaveClass(/active/, { timeout: 1700 });
  await expect(page.locator("#elapsed")).toHaveText("2.21 / 2.21 s", {
    timeout: 3000,
  });
  await expect(page.locator("#preview")).toContainText("Preview");
  await page.locator("#preview").click();
  await page.locator("#preview").click();
  await expect(page.locator("#elapsed")).toHaveText("0.00 / 2.21 s");
  await page.waitForTimeout(750);
  await expect(page.locator("#elapsed")).toHaveText("0.00 / 2.21 s");
  expect(errors).toEqual([]);
});

test("timeline visibility preserves arrows; extreme spacing stays clear of controls", async ({
  page,
}) => {
  await page.goto("/");
  const before = await page.locator(".arrows").boundingBox();
  await page.locator("#timeline-toggle").uncheck();
  await expect(page.locator("#timeline")).toBeHidden();
  expect(await page.locator(".arrows").boundingBox()).toEqual(before);
  await page.locator("#timeline-toggle").check();
  for (const [id, value] of [
    ["arrowSize", "72"],
    ["timelineOffset", "100"],
    ["gap", "300"],
  ]) {
    await page.locator("#" + id).fill(value);
    await page.locator("#" + id).dispatchEvent("input");
  }
  const timeline = await page.locator("#timeline").boundingBox();
  const controls = await page.locator(".controls").boundingBox();
  expect(timeline.y + timeline.height).toBeLessThan(controls.y);
  const arrows = await page.locator(".arrows").boundingBox();
  expect(arrows.x).toBeGreaterThanOrEqual(0);
  await page.setViewportSize({ width: 520, height: 560 });
  const narrow = await page.locator("#timeline").boundingBox();
  const below = await page.locator(".controls").boundingBox();
  expect(narrow.y + narrow.height).toBeLessThan(below.y);
  await expect(page.locator("#preview")).toBeEnabled();
});

test("default layout renders and practice keeps the same arrow anchor", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.locator("#preview")).toBeVisible();
  const arrows = await page.locator(".arrows").boundingBox();
  const helpers = await page.locator(".helper").boundingBox();
  const footer = await page.locator("footer").boundingBox();
  expect(helpers.y + helpers.height).toBeLessThan(footer.y);
  await page.screenshot({ path: "test-results/edit-mode.png" });
  // Visual state only: this does not claim to test native click-through.
  await page.evaluate(() => document.body.classList.add("practice"));
  expect(await page.locator(".arrows").boundingBox()).toEqual(arrows);
  await expect(page.locator(".controls")).toBeHidden();
  await page.screenshot({
    path: "test-results/practice-mode.png",
    omitBackground: true,
  });
});
