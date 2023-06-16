package main

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	// EventSendMessage es el nombre del evento para los nuevos mensajes de chat enviados
	EventSendMessage = "send_message"
	// EventNewMessage es una respuesta a send_message
	EventNewMessage = "new_message"
	// EventChangeRoom es el evento al cambiar de sala
	EventChangeRoom = "change_room"
)

// El evento son los mensajes enviados a través del websocket
// Usado para diferenciar entre diferentes acciones
type Event struct {
	// Type es el tipo de mensaje enviado
	Type string `json:"type"`
	// Payload son los datos basados en el tipo
	Payload json.RawMessage `json:"payload"`
}

// EventHandler es una firma de función que se usa para afectar los mensajes en el socket y se activa según el tipo
type EventHandler func(event Event, c *Client) error

// SendMessageEvent es el payload enviado en el evento send_message
type SendMessageEvent struct {
	Message string `json:"message"`
	From    string `json:"from"`
}

// NewMessageEvent se devuelve al responder a send_message
type NewMessageEvent struct {
	SendMessageEvent
	Sent time.Time `json:"sent"`
}

// SendMessageHandler enviará un mensaje a todos los demás participantes en el chat
func SendMessageHandler(event Event, c *Client) error {
	var chatevent SendMessageEvent
	if err := json.Unmarshal(event.Payload, &chatevent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	// Preparar un mensaje saliente para otros
	var broadMessage NewMessageEvent

	broadMessage.Sent = time.Now()
	broadMessage.Message = chatevent.Message
	broadMessage.From = chatevent.From

	data, err := json.Marshal(broadMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %v", err)
	}

	// Coloca Payload en un evento
	var outgoingEvent Event
	outgoingEvent.Payload = data
	outgoingEvent.Type = EventNewMessage
	// Transmisión a todos los demás clientes
	for client := range c.manager.clients {
		// Enviar solo a clientes dentro de la misma sala de chat
		if client.chatroom == c.chatroom {
			client.egress <- outgoingEvent
		}
	}

	return nil
}

type ChangeRoomEvent struct {
	Name string `json:"name"`
}

// ChatRoomHandler manejará el cambio de salas de chat entre clientes
func ChatRoomHandler(event Event, c *Client) error {
	// Marshal Payload into wanted format
	var changeRoomEvent ChangeRoomEvent
	if err := json.Unmarshal(event.Payload, &changeRoomEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	// Agregar cliente a la sala de chat
	c.chatroom = changeRoomEvent.Name

	return nil
}
