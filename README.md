# Ago.io
Welcome to Ago.io, an implementation/spin-off of agar.io. The server is written in Go (unlike C++ as in the original), but the client is written in JS using the canvas element (the same pattern as the original). 

**Play it live:** https://agoio.justind.dev

## To run:
Project uses Go 1.27+

1. Start the server: cd agoio -> go run .
2. Join from any web browser on http://localhost:8090. Test multiple clients by just opening a new tab.

## To test:
E2E tests use Playwright (Chromium). They start their own server on port 8092 so they don't collide with a running game.

1. Install: `npm install`, then `npx playwright install chromium`
2. Run: `npx playwright test`

## Plans:
This project is a work in progress. In the future I plan to host it in the cloud with scalable server infrastructure. I also plan to make the project open-source to accept feature additions at a later date.