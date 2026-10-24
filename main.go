package main

import (
	"log"
	"net/http"

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

		for {
			thisPosUpdate := posUpdate{}

			err = conn.ReadJSON(&thisPosUpdate)
			if err != nil {
				log.Print("Error receiving client position: ", err)
				return
			}
			log.Print("Client msg received: ", thisPosUpdate)
		}

	}

	http.HandleFunc("/ws", wsHandler)

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		log.Fatalf("Server error: %s", err)
	}
}
