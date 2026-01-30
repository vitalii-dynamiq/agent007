package store

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	ToolCallID string   `json:"toolCallId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
}

// Conversation represents a chat conversation
type Conversation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	UserID       string    `json:"userId"`
	SandboxID    string    `json:"sandboxId,omitempty"`
	EnabledTools []string  `json:"enabledTools,omitempty"` // Integration IDs enabled for this chat
	Messages     []Message `json:"messages"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MemoryStore is an in-memory store for conversations
type MemoryStore struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		conversations: make(map[string]*Conversation),
	}
}

// CreateConversation creates a new conversation
func (s *MemoryStore) CreateConversation(userID, title string) *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	conv := &Conversation{
		ID:        uuid.New().String(),
		Title:     title,
		UserID:    userID,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.conversations[conv.ID] = conv
	return conv
}

// GetConversation gets a conversation by ID
func (s *MemoryStore) GetConversation(id string) *Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.conversations[id]
}

// ListConversations lists conversations for a user
func (s *MemoryStore) ListConversations(userID string) []*Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Conversation
	for _, conv := range s.conversations {
		if conv.UserID == userID {
			result = append(result, conv)
		}
	}

	return result
}

// AddMessage adds a message to a conversation
func (s *MemoryStore) AddMessage(conversationID string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return nil // Conversation not found
	}

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now()

	// Update title from first user message if not set
	if conv.Title == "" && msg.Role == "user" {
		title := msg.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv.Title = title
	}

	return nil
}

// SetSandboxID sets the sandbox ID for a conversation
func (s *MemoryStore) SetSandboxID(conversationID, sandboxID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conv, ok := s.conversations[conversationID]; ok {
		conv.SandboxID = sandboxID
		conv.UpdatedAt = time.Now()
	}
}

// DeleteConversation deletes a conversation
func (s *MemoryStore) DeleteConversation(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.conversations, id)
}

// SetEnabledTools sets the enabled tools for a conversation
func (s *MemoryStore) SetEnabledTools(conversationID string, tools []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[conversationID]
	if !ok {
		return nil // Conversation not found
	}

	conv.EnabledTools = tools
	conv.UpdatedAt = time.Now()
	return nil
}

// GetEnabledTools gets the enabled tools for a conversation
func (s *MemoryStore) GetEnabledTools(conversationID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if conv, ok := s.conversations[conversationID]; ok {
		return conv.EnabledTools
	}
	return nil
}

// UpdateConversation updates a conversation's mutable fields
func (s *MemoryStore) UpdateConversation(id string, title string, enabledTools []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conv, ok := s.conversations[id]
	if !ok {
		return nil
	}

	if title != "" {
		conv.Title = title
	}
	if enabledTools != nil {
		conv.EnabledTools = enabledTools
	}
	conv.UpdatedAt = time.Now()
	return nil
}
