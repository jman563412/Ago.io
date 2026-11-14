package main

import (
	"log"
	"maps"
	"math"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const worldSize = 2000
const pelletCount = 50
const initialPlayerRadius = 50

// Min threshold for snapping to cursor
const stepSize = 5.0

// Client ID generator
var counter atomic.Int64

// Used by handlers to send client conn updates to ticker loop
var tickerInputChan = make(chan ClientMsg)

var wsUpgrader = websocket.Upgrader{}

type pellet struct {
	Id,
	X,
	Y int
}

var pelletMap = make(map[int]pellet)

func initializePelletMap() {
	for i := range pelletCount {
		thisPellet := pellet{
			i,
			rand.Intn(worldSize),
			rand.Intn(worldSize),
		}
		pelletMap[i] = thisPellet
	}
	log.Println("Created pelletMap")
}

type player struct {
	cursorTarget,
	blobPosition posUpdate
	isDead bool
}

type posUpdate struct {
	X,
	Y,
	R int
}

type ClientUpdate struct {
	Pellets   map[int]pellet
	PlayerPos posUpdate
	AllBlobs  map[int64]posUpdate
	IsDead    bool
}

type ClientMsg struct {
	id         int64
	isJoining  bool
	isLeaving  bool
	updateChan chan ClientUpdate
	cursorPos  posUpdate
}

func main() {
	initializePelletMap()

	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", fileServer)
	http.HandleFunc("/ws", wsHandler)

	go GameTickerLoop(tickerInputChan)
	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		log.Fatalf("Server error: %s", err)
	}
}

// inputChan: for managing client joins/leaves and cursor position updates (receiving)
func GameTickerLoop(inputChan <-chan ClientMsg) {
	// ~30 ticks/s
	ticker := time.NewTicker(time.Millisecond * 33)

	// map of client id -> update chan is for blob location + pellet updates (sending)
	updateChansMap := make(map[int64]chan ClientUpdate)

	// Subsequent pellet IDs should start after last value from the initial group
	nextPelletId := pelletCount

	// Same player ids mapped to cursor + blob positions
	playerMap := make(map[int64]player)

	for {
		select {
		case msg := <-inputChan:
			handleClientMsg(msg, playerMap, updateChansMap)
		case <-ticker.C:
			// Track pellets eaten per tick so we can replace them
			pelletsEaten := 0

			// First step updates every player blob position
			for playerId, thisPlayer := range playerMap {
				// Dead players don't update positions
				if thisPlayer.isDead {
					continue
				}

				thisPlayerBlob := moveBlob(thisPlayer)
				pelletsEaten += eatPellets(&thisPlayerBlob)

				thisPlayer.blobPosition = thisPlayerBlob
				playerMap[playerId] = thisPlayer
			}

			nextPelletId = respawnPellets(pelletsEaten, nextPelletId)

			eatPlayers(playerMap)

			// Avoid data race from concurrent pelletmap deletes and writes to clients by just giving clients a copy
			pelletMapCopy := make(map[int]pellet)
			maps.Copy(pelletMapCopy, pelletMap)

			AllBlobs := buildBlobsMap(playerMap)

			sendPlayerUpdates(playerMap, updateChansMap, pelletMapCopy, AllBlobs)

		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("WS path hit")
	conn, err := wsUpgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Print("Something wrong with ws connection: ", err)
		return
	}

	thisId := counter.Add(1)
	thisUpdateChan := make(chan ClientUpdate, 5)
	joinMsg := ClientMsg{
		id:         thisId,
		isJoining:  true,
		updateChan: thisUpdateChan,
	}
	tickerInputChan <- joinMsg

	defer conn.Close()

	go clientPosReadLoop(conn, thisId, tickerInputChan)

	for thisClientUpdate := range thisUpdateChan {
		err := conn.WriteJSON(thisClientUpdate)
		if err != nil {
			log.Println("Error writing ClientUpdate: ", err)
			return
		}
	}
}

// Adds a joining player, removes a leaving player + closes their update channel, or update's the player's cursor target
func handleClientMsg(msg ClientMsg, playerMap map[int64]player, updateChansMap map[int64]chan ClientUpdate) {
	if msg.isJoining {
		updateChansMap[msg.id] = msg.updateChan
		initialPos := posUpdate{
			1000,
			1000,
			initialPlayerRadius,
		}
		playerMap[msg.id] = player{
			initialPos,
			initialPos,
			false,
		}
		log.Println("Ticker Loop chans map has ", len(updateChansMap), " players remaining")
	} else if msg.isLeaving {
		close(updateChansMap[msg.id])
		delete(updateChansMap, msg.id)
		delete(playerMap, msg.id)
		log.Println("Player ID ", msg.id, " left, ", len(playerMap), " players remaining")
	} else {
		// Update player's cursor position in map
		thisPlayer := playerMap[msg.id]
		thisPlayer.cursorTarget = msg.cursorPos
		playerMap[msg.id] = thisPlayer
	}
}

// Move player's blob towards their cursor position
func moveBlob(p player) posUpdate {
	xDist := p.cursorTarget.X - p.blobPosition.X
	yDist := p.cursorTarget.Y - p.blobPosition.Y
	// Get the distance using pythagorean
	dist := math.Hypot(float64(xDist), float64(yDist))

	var thisPlayerBlob posUpdate

	// If distance is below threshold for step size, just snap blob to client cursor
	if dist < stepSize {
		thisPlayerBlob = posUpdate{
			p.cursorTarget.X,
			p.cursorTarget.Y,
			p.blobPosition.R,
		}
	} else {
		// The trip is this many steps long
		stepsToCursor := dist / stepSize
		thisPlayerBlob = posUpdate{
			p.blobPosition.X + int(math.Round(float64(xDist)/stepsToCursor)),
			p.blobPosition.Y + int(math.Round(float64(yDist)/stepsToCursor)),
			p.blobPosition.R,
		}
	}
	clampPlayerBlob(&thisPlayerBlob)

	return thisPlayerBlob
}

// Eat every pellet within distance of this client blob's radius and increase the client's radius size
func eatPellets(blob *posUpdate) int {
	pelletsEaten := 0

	for pelletId, thisPellet := range pelletMap {
		dist := distance(blob.X, blob.Y, thisPellet.X, thisPellet.Y)

		if dist < float64(blob.R) {
			delete(pelletMap, pelletId)
			blob.R += 1
			pelletsEaten += 1
		}
	}

	return pelletsEaten
}

// Replace each of the eaten pellets with new one in random position
func respawnPellets(pelletsEaten, nextPelletId int) int {
	for range pelletsEaten {
		nextPellet := pellet{
			nextPelletId,
			rand.Intn(worldSize),
			rand.Intn(worldSize),
		}
		pelletMap[nextPelletId] = nextPellet
		nextPelletId += 1
	}

	return nextPelletId
}

// Updates player state in the playerMap when conditions are met for larger blob eating smaller
func eatPlayers(playerMap map[int64]player) {
	// Larger playerBlob eats smaller playerBlob
	for eaterId := range playerMap {
		for preyId := range playerMap {
			thisEater := playerMap[eaterId]
			thisPrey := playerMap[preyId]

			// A blob can't eat itself, skip dead players
			if preyId == eaterId || thisEater.isDead || thisPrey.isDead {
				continue
			}

			// If blob 1's radius is larger than blob 2's radius
			if thisEater.blobPosition.R > thisPrey.blobPosition.R {
				dist := distance(thisEater.blobPosition.X, thisEater.blobPosition.Y, thisPrey.blobPosition.X, thisPrey.blobPosition.Y)

				// AND if distance between blob 1 and blob 2 is less than blob 1's radius, eat it
				if dist < float64(thisEater.blobPosition.R) {
					thisPrey.isDead = true

					// Eater gains half of prey's size
					thisEater.blobPosition.R += thisPrey.blobPosition.R / 2

					playerMap[eaterId] = thisEater
					playerMap[preyId] = thisPrey
				}
			}
		}
	}
}

// Builds map of every player's position for ClientUpdate
func buildBlobsMap(playerMap map[int64]player) map[int64]posUpdate {
	AllBlobs := make(map[int64]posUpdate)
	for playerId, player := range playerMap {

		// Dead players aren't drawn for anyone so they're omitted from the AllBlobs in the clients' updates
		if player.isDead {
			continue
		}

		thisPosUpdate := posUpdate{
			player.blobPosition.X,
			player.blobPosition.Y,
			player.blobPosition.R,
		}
		AllBlobs[playerId] = thisPosUpdate
	}

	return AllBlobs
}

// Sends the updated players to each player's channel
func sendPlayerUpdates(playerMap map[int64]player, updateChansMap map[int64]chan ClientUpdate, pelletMapCopy map[int]pellet, allBlobs map[int64]posUpdate) {
	for playerId, player := range playerMap {
		thisClientUpdate := ClientUpdate{
			pelletMapCopy,
			player.blobPosition,
			allBlobs,
			player.isDead,
		}

		thisPlayerChan := updateChansMap[playerId]
		select {
		case thisPlayerChan <- thisClientUpdate:

		default:
		}
	}
}

func clientPosReadLoop(conn *websocket.Conn, thisPlayerId int64, updateChan chan<- ClientMsg) {
	for {
		thisPosUpdate := posUpdate{}

		err := conn.ReadJSON(&thisPosUpdate)
		if err != nil {
			log.Print("Error receiving client position: ", err)
			cliLeavingUpdate := ClientMsg{
				id:        thisPlayerId,
				isJoining: false,
				isLeaving: true,
			}
			updateChan <- cliLeavingUpdate
			return
		}

		cliUpdate := ClientMsg{
			id:        thisPlayerId,
			isJoining: false,
			cursorPos: thisPosUpdate,
		}
		updateChan <- cliUpdate
		// log.Print("Client msg received: ", thisPosUpdate)
	}
}

// Clamp blob position to map borders for X, Y
func clampPlayerBlob(playerBlob *posUpdate) {
	if playerBlob.X < 0 {
		playerBlob.X = 0
	} else if playerBlob.X > worldSize {
		playerBlob.X = worldSize
	}

	if playerBlob.Y < 0 {
		playerBlob.Y = 0
	} else if playerBlob.Y > worldSize {
		playerBlob.Y = worldSize
	}
}

// Calculates distance between 2 points using pythagorean
func distance(x1, y1, x2, y2 int) float64 {
	xDist := x2 - x1
	yDist := y2 - y1
	return math.Hypot(float64(xDist), float64(yDist))
}
