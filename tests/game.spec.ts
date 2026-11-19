import { expect, Page, test } from "@playwright/test"

// Same types from Go server
type PosUpdate = {
    X: number,
    Y: number,
    R: number,
}
type Pellet = {
    Id: number,
    X: number,
    Y: number,
}
type ClientUpdate = {
    Pellets: Record<string, Pellet>,
    PlayerPos: PosUpdate,
    AllBlobs: Record<string, PosUpdate>, // JSON object keys are always strings
    IsDead: boolean,
}
type Pos = {
    X: number,
    Y: number,
}

// Helper to fetch the latest server update (sent by server and saved by client), all positions are world coords
async function getLastUpdate(page: Page): Promise<ClientUpdate> {
    return await page.evaluate(() => (window as any).lastUpdate)
}

// Fetches PlayerPos from window.lastUpdate
async function getLastPlayerPos(page: Page) {
    return (await getLastUpdate(page)).PlayerPos
}

// Returns the pellet nearest to the player blob
async function getNearestPellet(page: Page, update: ClientUpdate) {
    const playerPos = update.PlayerPos
    const pellets = Object.values(update.Pellets)

    // Find the nearest pellet to blob
    let target = pellets[0]
    let targetDist = Infinity
    for (const pellet of pellets) {
        const dist = Math.hypot(pellet.X - playerPos.X, pellet.Y - playerPos.Y)
        if (dist < targetDist) {
            target = pellet
            targetDist = dist
        }
    }

    return target
}

// Moves the mouse in the direction of the line going through the target
async function aimAt(page: Page, playerPos: Pos, target: Pos) {
    // Calculate distance to target pellet and move mouse 300px toward it
    // dx/dist and dy/dist are the direction to the pellet, dividing by dist gives the x and y parts of the unit vector
    // * 300 puts mouse 300 px from the blob (which is at (640, 360)) in the direction of the line going THROUGH the pellet
    const dx = target.X - playerPos.X
    const dy = target.Y - playerPos.Y
    const dist = Math.hypot(dx, dy)

    // Avoid 0 division
    if (dist === 0) {
        return
    }

    await page.mouse.move(640 + 300 * dx / dist, 360 + 300 * dy / dist)
}

// Continue steering the player blob to the target pellet until it no longer exists in the latest server update
async function waitUntilPelletEaten(page: Page, target: Pellet) {
    await expect(async() => {
        const update = await getLastUpdate(page)

        // As long as the target pellet still exists, keep steering towards it
        if (update.Pellets[target.Id]) {
            await aimAt(page, update.PlayerPos, target)
        }

        expect(update.Pellets[target.Id]).toBeUndefined()
    }).toPass({timeout: 15_000, intervals: [100]})
}

// Helper to join game plus handle chromium mousemove on page load
async function joinGame(page: Page) {
    // baseURL in playwright config autofills host and port
    await page.goto("/")

    // Wait for first playerpos update from server
    await page.waitForFunction(() => (window as any).lastUpdate)

    // Chromium has a weird behavior where it emits a mousemove event at (0, 0) on page load, which steers the blob up-left for a tick
    // Recenter the mouse and let a few ticks pass so we know blob is still 
    await page.mouse.move(640, 360)
    await page.waitForTimeout(100)
}

test("blob moves toward cursor", async({page}) => {
    await joinGame(page)

    // Read start position
    const startPos = await getLastPlayerPos(page)

    // Initial viewport center is at (640, 360)
    await page.mouse.move(940, 360)
    await page.waitForTimeout(1000)

    // Read end position
    const endPos = await getLastPlayerPos(page)

    // Every 33ms tick we move 5 pixels (const stepSize = 5.0), ~150px/s. + 100 leaves room for tick timing jitter
    expect(endPos.X).toBeGreaterThanOrEqual(startPos.X + 100)
    // Mouse is level with the blob (screen Y 360), so yDist is 0 and Y never changes
    expect(endPos.Y).toBe(startPos.Y)
})

test("blob stops at map edge", async({page}) => {
    await joinGame(page)

    const startPos = await getLastPlayerPos(page)

    await page.mouse.move(940, 360)

    // The trip from spawn world coords (x = 1000) to map border (x = 2000) is ~150 px/s so ~6.7 seconds
    await expect.poll(
        () => getLastUpdate(page).then(update => update.PlayerPos.X),
        {timeout: 10000},
    ).toBe(2000)

    // Wait some more after hitting edge of the map, pos.X should stay the same and not go outside map
    await page.waitForTimeout(500)

    const endPos = await getLastPlayerPos(page)

    // X should still be at 2000, Y never changes
    expect(endPos.X).toBe(2000)
    expect(endPos.Y).toBe(startPos.Y)
})

test("eating pellet removes it and grows blob", async({page}) => {
    await joinGame(page)

    const update = await getLastUpdate(page)
    const playerPos = update.PlayerPos

    const target = await getNearestPellet(page, update)

    // Wait until target pellet's ID is gone (was eaten)
    await waitUntilPelletEaten(page, target)

    // R should be larger after eating the pellet
    const endPos = await getLastPlayerPos(page)
    expect(endPos.R).toBeGreaterThan(playerPos.R)
})

test("eaten pellet gets replaced", async({page}) => {
    await joinGame(page)

    const update = await getLastUpdate(page)
    const startIds = Object.keys(update.Pellets)

    const target = await getNearestPellet(page, update)
    
    // Wait until target pellet's ID is gone like in grows blob test
    await waitUntilPelletEaten(page, target)

    const end = await getLastUpdate(page)
    const endIds = Object.keys(end.Pellets)

    // pellet map should still be length 50
    expect(endIds.length).toBe(50)
    // new pellet map should no longer contain the original target pellet's ID
    expect(endIds).not.toContain(String(target.Id))
    // new pellet map has a pellet with ID that DNE in the original
    expect(endIds.some(id => !startIds.includes(id))).toBe(true)
})

