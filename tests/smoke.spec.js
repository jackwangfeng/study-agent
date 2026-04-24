// @ts-check
//
// Smoke test for the StudyAgent PWA frontend. Verifies:
//   1. Page loads, header text shows.
//   2. Alpine binds (button is enabled once input has text).
//   3. Send fires a request to the backend chem/solve endpoint.
//   4. Both bubbles (user + assistant) render after the round-trip.
//
// Backend is mocked via page.route so the test doesn't depend on a real
// DashScope key — the actual upstream is exercised manually / in dev.

const { test, expect } = require("@playwright/test");

test.describe.serial("study-agent PWA smoke", () => {
  test("page loads and header shows", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/StudyAgent/);
    await expect(page.getByText("化学辅导 · 高二")).toBeVisible();
  });

  test("alpine binds — send button enables once text is typed", async ({ page }) => {
    await page.goto("/");
    const send = page.getByRole("button", { name: "发送" });
    // Empty textarea → button disabled
    await expect(send).toBeDisabled();
    await page.locator("textarea").fill("乙醇被氧化成乙醛？");
    // Filling the textarea should flip the disabled binding
    await expect(send).toBeEnabled();
  });

  test("send round-trip renders user + assistant bubbles", async ({ page }) => {
    // Mock the backend so we don't need a live DashScope account during CI.
    await page.route("**/v1/chem/solve", async (route) => {
      const body = JSON.parse(route.request().postData() || "{}");
      // Quick echo-style verification that the frontend posted the question.
      expect(body.question).toBeTruthy();
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          answer:
            "第一级提示：先想想这是什么类型的反应？\n\n\\( \\ce{2CH3CH2OH + O2 -> 2CH3CHO + 2H2O} \\)",
        }),
      });
    });

    await page.goto("/");
    await page.locator("textarea").fill("乙醇被氧化成乙醛");
    await page.getByRole("button", { name: "发送" }).click();

    // User bubble shows the message
    await expect(page.getByText("乙醇被氧化成乙醛")).toBeVisible();
    // Assistant bubble shows the hint text — KaTeX may rewrite the formula
    // chunk so we only assert on the prose-side text.
    await expect(page.getByText(/第一级提示/)).toBeVisible({ timeout: 10_000 });
  });

  test("camera button hosts a hidden file input", async ({ page }) => {
    await page.goto("/");
    // The button is a <label> wrapping an <input type=file accept=image>.
    const fileInput = page.locator("input[type=file]");
    await expect(fileInput).toHaveAttribute("accept", "image/*");
    await expect(fileInput).toHaveAttribute("capture", "environment");
  });

  test("manifest + sw paths are reachable (PWA installability)", async ({ page }) => {
    const manifest = await page.request.get("/manifest.webmanifest");
    expect(manifest.status()).toBe(200);
    const data = await manifest.json();
    expect(data.icons.length).toBeGreaterThanOrEqual(2);

    const sw = await page.request.get("/sw.js");
    expect(sw.status()).toBe(200);
  });
});
