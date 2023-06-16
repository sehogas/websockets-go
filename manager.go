package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// El administrador se utiliza para mantener referencias a todos los clientes registrados y de transmisión, etc.
type Manager struct {
	clients ClientList

	// Usar syncMutex aquí para poder bloquear el estado antes de editar clientes.
	// También podría usar canales para bloquear
	sync.RWMutex
	// los handlers son funciones que se usan para manejar eventos
	handlers map[string]EventHandler
	// otps es un mapa de OTP permitido para aceptar conexiones
	otps RetentionMap
}

// NewManager se usa para inicializar todos los valores dentro del administrador
func NewManager(ctx context.Context) *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),
		// Crear un nuevo mapa de retención que elimine Otps de más de 5 segundos
		otps: NewRetentionMap(ctx, 5*time.Second),
	}
	m.setupEventHandlers()
	return m
}

// routeEvent se usa para asegurarse de que el evento correcto entre en el controlador correcto
func (m *Manager) routeEvent(event Event, c *Client) error {
	// Comprobar si el controlador está presente en el mapa
	if handler, ok := m.handlers[event.Type]; ok {
		// Ejecuta el controlador y devuelve cualquier error
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return ErrEventNotSupported
	}
}

// setupEventHandlers configura y agrega todos los controladores
func (m *Manager) setupEventHandlers() {
	m.handlers[EventSendMessage] = SendMessageHandler
	m.handlers[EventChangeRoom] = ChatRoomHandler
}

// loginHandler se utiliza para verificar la autenticación de un usuario y devolver una contraseña de un solo uso
func (m *Manager) loginHandler(w http.ResponseWriter, r *http.Request) {

	type userLoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req userLoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Autenticar usuario / Verificar token de acceso, cualquiera que sea el método de autenticación que use
	if req.Username == "shogas" && req.Password == "123" {
		// formato para devolver otp al frontend
		type response struct {
			OTP string `json:"otp"`
		}

		// agregar una nueva OTP
		otp := m.otps.NewOTP()

		resp := response{
			OTP: otp.Key,
		}

		data, err := json.Marshal(resp)
		if err != nil {
			log.Println(err)
			return
		}
		// Devuelve una respuesta al usuario autenticado con el OTP
		w.WriteHeader(http.StatusOK)
		w.Write(data)
		return
	}

	// Fallo en la autenticación
	w.WriteHeader(http.StatusUnauthorized)
}

// serveWS es un controlador HTTP que tiene el administrador que permite las conexiones
func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {

	otp := r.URL.Query().Get("otp")
	if otp == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Verifica que exista el OTP
	if !m.otps.VerifyOTP(otp) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Println("Nueva conexión")
	// Comienza actualizando la solicitud HTTP
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Crea nuevo Cliente
	client := NewClient(conn, m)
	// Agregar el cliente recién creado al administrador
	m.addClient(client)

	// Iniciar los procesos de lectura/escritura
	go client.readMessages()
	go client.writeMessages()

}

// addClient agregará clientes a nuestra lista de clientes
func (m *Manager) addClient(client *Client) {
	// Lock so we can manipulate
	m.Lock()
	defer m.Unlock()

	// Add Client
	m.clients[client] = true
}

// removeClient eliminará al cliente y limpiará
func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}
