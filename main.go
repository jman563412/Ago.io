package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type posUpdate struct {
	X,
	Y,
	R int
}

func main() {
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", fileServer)

	wsUpgrader := websocket.Upgrader{}

	wsHandler := func(w http.ResponseWriter, r *http.Request) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		log.Println("WS path hit")
		conn, err := wsUpgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Print("Something wrong with ws connection: ", err)
			return
		}
		defer conn.Close()

		err = conn.WriteJSON(posUpdate{
			X: 200,
			Y: 150,
			R: 30,
		})
		if err != nil {
			log.Print("Error sending client pos update: ", err)
			return
		}

		thisClientChan := make(chan posUpdate)
		go clientPosReadLoop(conn, thisClientChan)

		// for msg := range thisClientChan {
		// 	log.Print(msg)
		// }
		var thisClientPos posUpdate
		var ok bool
		for {
			select {
			case <-ticker.C:
				{
					log.Print(thisClientPos)

					// Step a server-owned blob a fixed distance toward thisClientPos and WriteJSON it
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

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		log.Fatalf("Server error: %s", err)
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
