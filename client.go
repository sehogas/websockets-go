package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// pongWait es el tiempo que esperaremos una respuesta pong del cliente
	pongWait = 10 * time.Second
	// pingInterval tiene que ser menor que pongEspera, no podemos multiplicar por 0.9 para obtener el 90% del tiempo
	// porque eso puede hacer decimales, así que en su lugar *9/10 para obtener 90%
	// La razón por la que tiene que ser menor que PingRequency es porque, de lo contrario, enviará un nuevo Ping antes de obtener una respuesta.
	pingInterval = (pongWait * 9) / 10
)

// ClientList es un mapa utilizado para ayudar a administrar un mapa de clientes
type ClientList map[*Client]bool

// Client es un cliente websocket, básicamente un visitante frontend
type Client struct {
	// La conexión websocket
	connection *websocket.Conn
	// manager es el administrador utilizado para administrar el cliente
	manager *Manager
	// egress se usa para evitar escrituras simultáneas en el WebSocket
	egress chan Event
	// chatroom se utiliza para saber en qué sala está el usuario
	chatroom string
}

// NewClient se usa para inicializar un nuevo Cliente con todos los valores requeridos inicializados
func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		connection: conn,
		manager:    manager,
		egress:     make(chan Event),
	}
}

// readMessages iniciará el cliente para leer mensajes y manejarlos apropiadamente.
// Se supone que esto debe ejecutarse como una gorutina
func (c *Client) readMessages() {
	defer func() {
		// Cierra la conexión una ver que termina esta función
		c.manager.removeClient(c)
	}()

	// Se Establece el tamaño máximo de los mensajes en bytes
	c.connection.SetReadLimit(512)
	// Configure el tiempo de espera para la respuesta Pong, use la hora actual + pongWait
	// Esto debe hacerse aquí para configurar el primer temporizador inicial.
	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}
	// Configurar cómo manejar las respuestas Pong
	c.connection.SetPongHandler(c.pongHandler)

	// Ciclo infinito
	for {
		// ReadMessage se usa para leer el siguiente mensaje en cola en la conexión
		_, payload, err := c.connection.ReadMessage()

		if err != nil {
			// Si la conexión está cerrada, recibiremos un error aquí
			// Solo queremos registrar errores extraños, pero no una simple desconexión
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error leyendo mensaje: %v", err)
			}
			break // Rompe el ciclo para cerrar conn & Cleanup
		}
		// Ordena los datos entrantes en una estructura de evento
		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("error clasificando mensaje: %v", err)
			//break // Romper la conexión aquí puede ser duro xD
		} else {
			// Enruta el evento
			if err := c.manager.routeEvent(request, c); err != nil {
				log.Println("Error ruteando mensaje: ", err)
			}
		}
	}
}

// pongHandler se usa para manejar PongMessages para el Cliente
func (c *Client) pongHandler(pongMsg string) error {
	// Current time + Pong Wait time
	//log.Println("pong")
	return c.connection.SetReadDeadline(time.Now().Add(pongWait))
}

// writeMessages es un proceso que escucha nuevos mensajes para enviarlos al Cliente
func (c *Client) writeMessages() {
	// Crea un ticker que activa un ping en un intervalo dado
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		// Cierre elegante si esto desencadena un cierre
		c.manager.removeClient(c)
	}()

	// Ciclo infinito
	for {
		select {
		case message, ok := <-c.egress:
			// Ok será falso en caso de que el canal de salida esté cerrado
			if !ok {
				// El administrador ha cerrado este canal de conexión, así que comunique eso a la interfaz
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					// Registrar que la conexión está cerrada y el motivo
					log.Println("conexión cerrada: ", err)
				}
				// Regresar para cerrar la gorutina
				return
			}
			data, err := json.Marshal(message)
			if err != nil {
				log.Println(err)
				return // cierra la conexión, ¿deberíamos realmente?
			}
			// Escribir un mensaje de texto normal a la conexión
			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println(err)
			}
			log.Println("sent message")

		case <-ticker.C:
			//log.Println("ping")
			// Enviar el ping
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println("writemsg: ", err)
				return // volver para interrumpir con limpieza de activación de goroutine
			}
		}
	}
}
