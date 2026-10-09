const { test, expect } = require("@playwright/test");

test("the weapon menu picks a weapon and its firing modes", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.locator("#weapon-label")).toHaveText("R-301");
  await expect(page.locator("#modes")).toBeHidden();
  await page.locator("#weapon-button").click();
  await expect(page.locator("#weapon-menu h3")).toHaveText([
    "Assault rifles",
    "SMGs",
  ]);
  await page.locator(".weapon-option", { hasText: "HAVOC" }).click();
  await expect(page.locator("#weapon-menu")).toBeHidden();
  await expect(page.locator("#weapon-label")).toHaveText("HAVOC");
  await expect(page.locator("#modes button")).toHaveText([
    "Normal",
    "Turbocharged",
  ]);
  await page.locator("#modes button", { hasText: "Turbocharged" }).click();
  await expect(page.locator("#modes .active")).toHaveText("Turbocharged");
  const picker = await page.locator("#weapon-button").boundingBox();
  const modes = await page.locator("#modes").boundingBox();
  expect([modes.y, modes.height]).toEqual([picker.y, picker.height]);
  await page.screenshot({ path: "test-results/modes.png" });
  await page.locator("#weapon-button").click();
  await page.keyboard.press("Escape");
  await expect(page.locator("#weapon-menu")).toBeHidden();
});

test("sliders take typed values, clamped to their range", async ({ page }) => {
  await page.goto("/");
  const typed = page.locator("#timelineOffset-number");
  await typed.fill("250");
  await typed.blur();
  await expect(page.locator("#timelineOffset")).toHaveValue("250");
  await typed.fill("9999");
  await typed.blur();
  await expect(typed).toHaveValue("800");
  await expect(page.locator("#timelineOffset")).toHaveValue("800");
  await page.locator("#gap").fill("120");
  await expect(page.locator("#gap-number")).toHaveValue("120");
  // Switching a category off greys out its settings without losing them.
  await page.locator("#timeline-toggle").uncheck();
  await expect(typed).toBeDisabled();
  await expect(page.locator(".card.timeline")).toHaveClass(/off/);
  await expect(page.locator(".card.arrows")).not.toHaveClass(/off/);
  await page.screenshot({ path: "test-results/category-off.png" });
  await expect(typed).toHaveValue("800");
  await page.locator("#voiceStyle").selectOption("tones");
  await expect(page.locator("#voiceStyle")).toHaveValue("tones");
  await page.locator("#theme").selectOption("purple");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "purple");
  await page.screenshot({ path: "test-results/theme-purple.png" });
  await page.locator("#theme").selectOption("blue");
  await page.screenshot({ path: "test-results/theme-blue.png" });
});

test("reset defaults asks twice, then restores every setting", async ({
  page,
}) => {
  await page.goto("/");
  await page.locator("#gap-number").fill("200");
  await page.locator("#gap-number").blur();
  await page.locator("#theme").selectOption("blue");
  const reset = page.locator("#reset");
  await reset.click();
  await expect(reset).toHaveText("Click again to reset");
  await expect(page.locator("#gap")).toHaveValue("200");
  await reset.click();
  await expect(reset).toHaveText("Reset defaults");
  await expect(page.locator("#gap")).toHaveValue("100");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "green");
  // Left alone, the first click expires.
  await reset.click();
  await expect(reset).toHaveText("Reset defaults", { timeout: 4000 });
  await expect(page.locator("#gap")).toHaveValue("100");
});

test("an error stays until it is clicked away", async ({ page }) => {
  await page.goto("/");
  const banner = page.locator("#error");
  await expect(banner).toBeHidden();
  // Practice needs the Windows app: in a browser its button is disabled, and
  // the stand-in answers with an error. Force it, to have an error to show.
  await page.locator("#lock").evaluate((button) => (button.disabled = false));
  await page.locator("#lock").click();
  await expect(banner).toContainText("Open the Windows app");
  await page.locator("#gap-number").fill("120");
  await page.locator("#gap-number").blur();
  await expect(banner).toBeVisible();
  await banner.click();
  await expect(banner).toBeHidden();
});

test("controls fit the fixed window", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/");
  const controls = page.locator(".controls");
  expect(
    await controls.evaluate((el) => el.scrollHeight <= el.clientHeight),
  ).toBe(true);
  await expect(page.locator("#minimise")).toBeVisible();
  const close = await page.locator("#quit").boundingBox();
  const bar = await page.locator(".toolbar").boundingBox();
  expect(
    Math.abs(close.y + close.height / 2 - (bar.y + bar.height / 2)),
  ).toBeLessThan(1);
  await page.screenshot({ path: "test-results/edit-mode.png" });
  await page.locator("#weapon-button").click();
  await page.screenshot({ path: "test-results/weapon-menu.png" });
  expect(errors).toEqual([]);
});
