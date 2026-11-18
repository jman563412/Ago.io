import { expect, test } from "@playwright/test"

test("blob moves toward cursor", async({page}) => {
    // baseURL in playwright config autofills host and port
    await page.goto("/")

    // Wait for first playerpos update from server
    await page.waitForFunction(() => (window as any).lastUpdate)

    // Read start position
    let startPos = await page.evaluate(() => (window as any).lastUpdate.PlayerPos)

    // Initial viewport center is at (640, 360)
    await page.mouse.move(940, 360)
    await page.waitForTimeout(1000)

    // Read end position
    let endPos = await page.evaluate(() => (window as any).lastUpdate.PlayerPos)

    // Every 33ms tick we move 5 pixels (const stepSize = 5.0), ~150px/s. + 100 leaves room for tick timing jitter
    expect(endPos.X).toBeGreaterThanOrEqual(startPos.X + 100)
    // Mouse is level with the blob (screen Y 360), so yDist is 0 and Y never changes
    expect(endPos.Y).toBe(startPos.Y)
})