package realtime

import (
	"encoding/json"
	"fmt"
	"log"
	"muma/internal/helpers"
	"net/http"

	"github.com/google/uuid"
	"github.com/mattbaird/jsonpatch"
)

// Realtime supports http handlers. The first is `Stream` which supports both a plain json API response
// and a streaming jsonpatch response.
//
// `sessions` allow for colocating data and clients that are subscribe to the stream and are able to receive patches.
type Realtime struct {
	sessions map[string]*Session
}

// Creates a new instance of realtime
func New() Realtime {
	return Realtime{sessions: make(map[string]*Session, 0)}
}

// Creates a new session
func (rt *Realtime) CreateSession(sessionID string) *Session {
	session := NewSession()
	rt.sessions[sessionID] = &session
	return &session
}

// Removes a session
func (rt *Realtime) RemoveSession(sessionID string) {
	delete(rt.sessions, sessionID)
}

// Handles creating a stream and channel if the X-Muma-Stream header is set. If header
// is not set, the raw json message is returned to the user like a standard REST api.
func (rt *Realtime) Stream(w http.ResponseWriter, r *http.Request, d json.RawMessage, sessionID string) {
	ctx := r.Context()

	s := r.Header.Get("X-Muma-Stream")
	if s == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(d)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		// TODO: come up with a generic error struct to return instead of a plain string
		http.Error(w, "Streaming not supported!", http.StatusInternalServerError)
		return
	}

	ch := make(chan []byte, 10)
	defer close(ch)

	// check if there is already a session for the current sessionID
	session, ok := rt.sessions[sessionID]

	if !ok {
		session = rt.CreateSession(sessionID)
	}

	clientID := session.AddClient(&ch)
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Content-Type", "application/json+ndjsonpatch")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s\n", d)
	flusher.Flush()

	session.Data = d

	for {
		select {
		case <-ctx.Done():
			count := session.RemoveClient(clientID)
			if count == 0 {
				rt.RemoveSession(sessionID)
			}
			return
		case value := <-ch:
			if len(value) > 0 {
				fmt.Fprintf(w, "%s\n", value)
			}
			flusher.Flush()
		}
	}
}

// Creates a new json patch
func (rt *Realtime) GeneratePatch(target json.RawMessage, sessionID string) (json.RawMessage, error) {
	session, ok := rt.sessions[sessionID]

	if !ok {
		return nil, fmt.Errorf("Failed to get session for sessionID: %s", sessionID)
	}

	patch, _ := jsonpatch.CreatePatch(session.Data, target)
	patchJson, err := json.Marshal(patch)

	if err != nil {
		log.Print("Failed to marshal json for patch")
		return nil, err
	}

	session.Data = target

	return patchJson, nil
}

// Creates a new json patch
func (rt *Realtime) PublishMsg(msg json.RawMessage, sessionID string) {
	session, ok := rt.sessions[sessionID]

	if !ok {
		helpers.Log(helpers.Error, "Failed to get session", nil)
		return
	}

	for _, client := range session.Clients {
		*client.Channel <- msg
	}
}

// Holds all of the active client connections
type Session struct {
	Clients []*Client
	Data    json.RawMessage
}

// Creates a new Session
func NewSession() Session {
	return Session{Clients: make([]*Client, 0), Data: nil}
}

// Adds a new client to the Session
func (s *Session) AddClient(ch *chan []byte) uuid.UUID {
	clientID := uuid.New()
	client := Client{clientID: clientID, Channel: ch}
	s.Clients = append(s.Clients, &client)
	return clientID
}

// Removes a client from the Session
func (s *Session) RemoveClient(clientID uuid.UUID) int {
	newClients := make([]*Client, 0)

	for _, c := range s.Clients {
		if c.clientID != clientID {
			newClients = append(newClients, c)
		}
	}

	s.Clients = newClients

	return len(s.Clients)
}

// Represents a single client connection
type Client struct {
	clientID uuid.UUID
	Channel  *chan []byte
}

// The response structure of a realtime api
type Data struct {
	Data interface{} `json:"data"`
}
