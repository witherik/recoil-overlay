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
  await expect(page.locator("#preview")).toContainText("Preview", {
    timeout: 3000,
  });
  await page.locator("#preview").click();
  await expect(page.locator("#preview")).toContainText("Stop");
  await page.locator("#preview").click();
  await expect(page.locator("#preview")).toContainText("Preview");
  expect(errors).toEqual([]);
});

test("the overlay can be hidden and shown; a preview brings it back", async ({
  page,
}) => {
  await page.goto("/");
  const toggle = page.locator("#showOverlay");
  await expect(toggle).toHaveText("Hide");
  await page.locator("#move").click();
  await expect(page.locator("#move")).toHaveText("Done");
  await toggle.click();
  await expect(toggle).toHaveText("Show");
  await expect(page.locator("#move")).toHaveText("Move");
  await page.locator("#voice-toggle").uncheck();
  await page.locator("#preview").click();
  await expect(toggle).toHaveText("Hide");
  await toggle.click();
  await expect(page.locator("#preview")).toContainText("Preview");
});

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
  await expect(page.locator("#version")).toContainText("HAVOC");
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
  await expect(typed).toHaveValue("400");
  await expect(page.locator("#timelineOffset")).toHaveValue("400");
  await page.locator("#gap").fill("120");
  await expect(page.locator("#gap-number")).toHaveValue("120");
  // Switching a category off greys out its settings without losing them.
  await page.locator("#timeline-toggle").uncheck();
  await expect(typed).toBeDisabled();
  await expect(page.locator(".card.timeline")).toHaveClass(/off/);
  await expect(page.locator(".card.arrows")).not.toHaveClass(/off/);
  await page.screenshot({ path: "test-results/category-off.png" });
  await expect(typed).toHaveValue("400");
  await page.locator("#voiceStyle").selectOption("tones");
  await expect(page.locator("#voiceStyle")).toHaveValue("tones");
  await page.locator("#theme").selectOption("violet");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "violet");
  await page.screenshot({ path: "test-results/theme-violet.png" });
  await page.locator("#theme").selectOption("ember");
  await page.waitForTimeout(300); // let the colour transitions settle
  await page.screenshot({ path: "test-results/theme-ember.png" });
});

test("reset defaults asks twice, then restores every setting", async ({
  page,
}) => {
  await page.goto("/");
  await page.locator("#gap-number").fill("200");
  await page.locator("#gap-number").blur();
  await page.locator("#theme").selectOption("ocean");
  const reset = page.locator("#reset");
  await reset.click();
  await expect(reset).toHaveText("Click again to reset");
  await expect(page.locator("#gap")).toHaveValue("200");
  await reset.click();
  await expect(reset).toHaveText("Reset defaults");
  await expect(page.locator("#gap")).toHaveValue("100");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "mint");
  // Left alone, the first click expires.
  await reset.click();
  await expect(reset).toHaveText("Reset defaults", { timeout: 4000 });
  await expect(page.locator("#gap")).toHaveValue("100");
});

test("keys can be rebound and the labels follow", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#lock-key")).toHaveText("F8");
  await page.locator("#bind-start").click();
  await expect(page.locator("#bind-start")).toHaveText("press a key…");
  await page.locator("#bind-start").click();
  await expect(page.locator("#bind-start")).toHaveText("F8");
  await expect(page.locator("#bind-pause")).toHaveText("F9");
});

test("controls fit the default window and stay reachable at the minimum", async ({
  page,
}) => {
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
  for (const size of [640, 520]) {
    await page.setViewportSize({ width: size, height: 760 });
    const sound = await page.locator("#voiceStyle").boundingBox();
    expect(sound.x + sound.width).toBeLessThan(size - 22);
  }
  await page.setViewportSize({ width: 640, height: 760 });
  await page.screenshot({ path: "test-results/edit-mode.png" });
  await page.locator("#weapon-button").click();
  await page.screenshot({ path: "test-results/weapon-menu.png" });
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 520, height: 440 });
  await page.locator("#lock").scrollIntoViewIfNeeded();
  for (const id of ["showOverlay", "voiceStyle", "bind-pause", "lock"]) {
    const box = await page.locator("#" + id).boundingBox();
    expect(box.x + box.width).toBeLessThanOrEqual(520);
  }
  await page.screenshot({ path: "test-results/minimum-size.png" });
});
