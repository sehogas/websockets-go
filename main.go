package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var (
	/**
	websocket Upgrader se utiliza para actualizar las solicitudes HTTP entrantes en una conexión websocket persistente
	*/
	websocketUpgrader = websocket.Upgrader{
		CheckOrigin:     checkOrigin,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	ErrEventNotSupported = errors.New("este tipo de evento no es compatible")
)

// checkOrigin comprobará el origen y devolverá verdadero si está permitido
func checkOrigin(r *http.Request) bool {
	// Toma el origen de la solicitud
	origin := r.Header.Get("Origin")

	switch origin {
	case "https://localhost:8080":
		return true
	default:
		return false
	}
}

func main() {
	// Cree un ctx raíz y un CancelFunc que se puede usar para cancelar la gorutina de retentionMap
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	defer cancel()

	setupAPI(ctx)

	err := http.ListenAndServeTLS(":8080", "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

// setupAPI iniciará todas las rutas y sus controladores
func setupAPI(ctx context.Context) {

	// Crear una instancia de Manager utilizada para manejar conexiones WebSocket
	manager := NewManager(ctx)

	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/login", manager.loginHandler)
	http.HandleFunc("/ws", manager.serveWS)

	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, len(manager.clients))
	})
}
