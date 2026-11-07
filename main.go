package main

import (
	"log"
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

var counter atomic.Int64

// const pelletRadius = 30

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

	wsUpgrader := websocket.Upgrader{}

	// Used by handlers to send client conn updates to ticker loop
	tickerInputChan := make(chan ClientMsg)

	wsHandler := func(w http.ResponseWriter, r *http.Request) {

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
		for {
			select {
			case thisClientUpdate, ok := <-thisUpdateChan:
				{
					if !ok {
						return
					}
					err := conn.WriteJSON(thisClientUpdate)
					if err != nil {
						log.Println("Error writing ClientUpdate: ", err)
						return
					}
				}
			}
		}
	}

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
	// Same player ids mapped to cursor + blob positions
	playerMap := make(map[int64]player)

	for {
		select {
		case msg := <-inputChan:
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

		case <-ticker.C:

			// First step updates every player blob position
			for player := range playerMap {
				var thisPlayerBlob posUpdate

				thisPlayer := playerMap[player]
				// Move playerBlob (server controls the blob) toward thisClientPos (client cursor location), write back out to client
				xDist := thisPlayer.cursorTarget.X - thisPlayer.blobPosition.X
				yDist := thisPlayer.cursorTarget.Y - thisPlayer.blobPosition.Y
				// Get the distance using pythagorean
				dist := math.Hypot(float64(xDist), float64(yDist))

				// If distance is below threshold for step size, just snap blob to client cursor
				if dist < stepSize {
					thisPlayerBlob = posUpdate{
						thisPlayer.cursorTarget.X,
						thisPlayer.cursorTarget.Y,
						thisPlayer.blobPosition.R,
					}
				} else {
					// The trip is this many steps long
					stepsToCursor := dist / stepSize
					thisPlayerBlob = posUpdate{
						thisPlayer.blobPosition.X + int(math.Round(float64(xDist)/stepsToCursor)),
						thisPlayer.blobPosition.Y + int(math.Round(float64(yDist)/stepsToCursor)),
						thisPlayer.blobPosition.R,
					}
				}
				clampPlayerBlob(&thisPlayerBlob)
				thisPlayer.blobPosition = thisPlayerBlob
				playerMap[player] = thisPlayer
			}

			AllBlobs := make(map[int64]posUpdate)
			// Second step builds map of position updates and sends to ALL players
			for player, value := range playerMap {
				thisPosUpdate := posUpdate{
					value.blobPosition.X,
					value.blobPosition.Y,
					value.blobPosition.R,
				}
				AllBlobs[player] = thisPosUpdate
			}

			for player, value := range playerMap {
				thisClientUpdate := ClientUpdate{
					pelletMap,
					value.blobPosition,
					AllBlobs,
				}

				thisPlayerChan := updateChansMap[player]
				select {
				case thisPlayerChan <- thisClientUpdate:

				default:
				}
			}

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
