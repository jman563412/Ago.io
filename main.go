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

type posUpdate struct {
	X,
	Y,
	R int
}

type ClientUpdate struct {
	Pellets   map[int]pellet
	PlayerPos posUpdate
}

type ClientMsg struct {
	id         int64
	isJoining  bool
	updateChan chan ClientUpdate
}

func main() {
	initializePelletMap()

	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", fileServer)

	wsUpgrader := websocket.Upgrader{}

	// Used by handlers to send client conn updates to ticker loop
	tickerInputChan := make(chan ClientMsg)

	wsHandler := func(w http.ResponseWriter, r *http.Request) {
		// ~30 ticks/s
		ticker := time.NewTicker(time.Millisecond * 33)
		defer ticker.Stop()

		log.Println("WS path hit")
		conn, err := wsUpgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Print("Something wrong with ws connection: ", err)
			return
		}
		thisId := counter.Add(1)
		thisUpdateChan := make(chan ClientUpdate, 5)
		joinMsg := ClientMsg{
			thisId,
			true,
			thisUpdateChan,
		}
		tickerInputChan <- joinMsg

		defer conn.Close()

		thisClientChan := make(chan posUpdate)
		go clientPosReadLoop(conn, thisClientChan)

		// Min threshold for snapping to cursor
		stepSize := 5.0

		playerBlob := posUpdate{
			1000,
			1000,
			initialPlayerRadius,
		}
		thisClientPos := playerBlob
		var ok bool
		for {
			select {
			case <-ticker.C:
				{
					// Move playerBlob (server controls the blob) toward thisClientPos (client cursor location), write back out to client
					xDist := thisClientPos.X - playerBlob.X
					yDist := thisClientPos.Y - playerBlob.Y
					// Get the distance using pythagorean
					dist := math.Hypot(float64(xDist), float64(yDist))

					// If distance is below threshold for step size, just snap blob to client cursor
					if dist < stepSize {
						playerBlob = posUpdate{
							thisClientPos.X,
							thisClientPos.Y,
							playerBlob.R,
						}
					} else {
						// The trip is this many steps long
						stepsToCursor := dist / stepSize

						// Move 1 step in the x and y direction
						playerBlob = posUpdate{
							playerBlob.X + int(math.Round(float64(xDist)/stepsToCursor)),
							playerBlob.Y + int(math.Round(float64(yDist)/stepsToCursor)),
							playerBlob.R,
						}
					}
					clampPlayerBlob(&playerBlob)

					thisUpdate := ClientUpdate{
						pelletMap,
						playerBlob,
					}

					err = conn.WriteJSON(thisUpdate)
					if err != nil {
						log.Print("Error writing new blob pos to client...", err)
						return
					}
				}

			case thisClientPos, ok = <-thisClientChan:
				{
					// !ok represents tab closed case, otherwise we'd keep receiving {0, 0, 0} indefinitely
					if !ok {
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
	// map of client id -> update chan is for blob location + pellet updates (sending)
	updateChansMap := make(map[int64]chan ClientUpdate)

	for {
		select {
		case msg := <-inputChan:
			if msg.isJoining {
				updateChansMap[msg.id] = msg.updateChan
				log.Println("Ticker Loop chans map has ", len(updateChansMap), " conns")
			}

		}
	}
}

func clientPosReadLoop(conn *websocket.Conn, channel chan posUpdate) {
	// Sender closes the channel
	defer close(channel)
	for {
		thisPosUpdate := posUpdate{}

		err := conn.ReadJSON(&thisPosUpdate)
		if err != nil {
			log.Print("Error receiving client position: ", err)
			return
		}

		channel <- thisPosUpdate
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
