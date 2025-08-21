package ws

import (
	"bufio"
	"encoding/json"
	"log"
	"net/http"
	"ollama-tiny-chat/server/internal/config"
	"ollama-tiny-chat/server/internal/database"
	"ollama-tiny-chat/server/internal/ollama"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn           *websocket.Conn
	currentConvoID string
}

type WSRequest struct {
	Type    string `json:"type"` // "message", "start_conversation", "resume_conversation"
	Message string `json:"message"`
	Model   string `json:"model"`
	ConvoID string `json:"convo_id,omitempty"`
}

type WSResponse struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("New WebSocket connection request from: %s", r.RemoteAddr)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		http.Error(w, "Could not upgrade connection", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client := &Client{
		conn:           conn,
		currentConvoID: "",
	}
	log.Printf("WebSocket client connected from: %s", r.RemoteAddr)

	for {
		var req WSRequest
		if err := conn.ReadJSON(&req); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		log.Printf("Received message type: %s", req.Type)

		switch req.Type {
		case "start_conversation":
			log.Printf("Starting new conversation with model: %s", req.Model)
			handleNewConversation(client, req)
		case "resume_conversation":
			log.Printf("Resuming conversation: %s", req.ConvoID)
			handleResumeConversation(client, req)
		case "message":
			log.Printf("Handling message for conversation: %s", client.currentConvoID)
			handleMessage(client, req)
		}
	}
}

func handleNewConversation(client *Client, req WSRequest) {
	log.Printf("Creating new conversation with first message: %s", req.Message)
	title := req.Message
	if len(title) > 30 {
		title = title[:30] + "..."
	}
	convoID, err := database.CreateConversation(title, req.Model)
	if err != nil {
		log.Printf("Failed to create conversation: %v", err)
		sendError(client, "Failed to create conversation")
		return
	}
	client.currentConvoID = convoID
	log.Printf("Created conversation with ID: %s", convoID)

	log.Printf("Saving initial user message")
	if err := database.AddMessage(convoID, "user", req.Message); err != nil {
		log.Printf("Failed to save initial message: %v", err)
		sendError(client, "Failed to save message")
		return
	}

	client.conn.WriteJSON(WSResponse{
		Type:    "conversation_started",
		Content: convoID,
	})

	generateResponse(client, req, true)
}

func handleResumeConversation(client *Client, req WSRequest) {
	// Verify conversation exists
	convo, err := database.GetConversationByID(req.ConvoID)
	if err != nil {
		log.Printf("Error fetching conversation: %v", err)
		sendError(client, "Failed to resume conversation")
		return
	}
	if convo == nil {
		log.Printf("Conversation not found: %s", req.ConvoID)
		sendError(client, "Conversation not found")
		return
	}

	// Set the conversation ID
	client.currentConvoID = req.ConvoID
	log.Printf("Resumed conversation: %s", req.ConvoID)

	// Send success response
	client.conn.WriteJSON(WSResponse{
		Type:    "conversation_resumed",
		Content: req.ConvoID,
	})
}

func handleMessage(client *Client, req WSRequest) {
	if client.currentConvoID == "" {
		log.Printf("Received message without active conversation")
		sendError(client, "No active conversation")
		return
	}
	log.Printf("User sent Message: %s, For model: %s", req.Message,req.Model)
	log.Printf("Saving user message to conversation: %s", client.currentConvoID)
	if err := database.AddMessage(client.currentConvoID, "user", req.Message); err != nil {
		log.Printf("Failed to save user message: %v", err)
		sendError(client, "Failed to save message")
		return
	}

	generateResponse(client, req, false)
}

func generateResponse(client *Client, req WSRequest, isFirstMessage bool) {
	log.Printf("Starting response generation - First Message: %v, ConvoID: %s", isFirstMessage, client.currentConvoID)

	var ollamaMessages []ollama.Message

	if !isFirstMessage {
		log.Printf("Fetching conversation history for ID: %s", client.currentConvoID)
		messages, err := database.GetMessagesByConversationID(client.currentConvoID)
		if err != nil {
			log.Printf("Error fetching history: %v", err)
			sendError(client, "Failed to get conversation history")
			return
		}
		log.Printf("Found %d previous messages", len(messages))

		ollamaMessages = make([]ollama.Message, len(messages))
		for i, msg := range messages {
			ollamaMessages[i] = ollama.Message{
				Role:    msg.Role,
				Content: msg.RawContent,
			}
		}
	}

	// Add current message
	ollamaMessages = append(ollamaMessages, ollama.Message{
		Role:    "user",
		Content: req.Message,
	})
	log.Printf("Sending request to Ollama with %d messages", len(ollamaMessages))

	ollamaClient := ollama.NewClient(config.Get().OllamaURL)
	resp, err := ollamaClient.GenerateStream(req.Model, ollamaMessages)
	if err != nil {
		log.Printf("Ollama request failed: %v", err)
		sendError(client, "Failed to generate response")
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var fullResponse strings.Builder
	var thinking strings.Builder
	var rawContent strings.Builder
	isThinking := false
	var thinkStartTime time.Time
	var thinkingDuration float64

	log.Println("Starting to process Ollama stream")
	for scanner.Scan() {
		var genResp ollama.GenerateResponse
		if err := json.Unmarshal(scanner.Bytes(), &genResp); err != nil {
			log.Printf("Error unmarshaling response chunk: %v", err)
			continue
		}

		rawContent.WriteString(genResp.Response)

		// Process the response chunk first
		if strings.Contains(genResp.Response, "<think>") {
			log.Println("Entering thinking mode")
			isThinking = true
			thinkStartTime = time.Now()

			// Notify client that thinking is starting
			client.conn.WriteJSON(WSResponse{
				Type:    "thinking_start",
				Content: "",
			})

			continue
		}
		if strings.Contains(genResp.Response, "</think>") {
			log.Println("Exiting thinking mode")
			isThinking = false
			thinkingDuration = time.Since(thinkStartTime).Seconds()

			client.conn.WriteJSON(WSResponse{
				Type:    "thinking_end",
				Content: thinking.String(),
			})
			continue
		}

		if isThinking {
			thinking.WriteString(genResp.Response)
			// Stream thinking content too
			client.conn.WriteJSON(WSResponse{
				Type:    "thinking_chunk",
				Content: genResp.Response,
			})
		} else {
			fullResponse.WriteString(genResp.Response)
			client.conn.WriteJSON(WSResponse{
				Type:    "response_chunk",
				Content: genResp.Response,
			})
		}

		// Only break after processing the response
		if genResp.Done {
			log.Printf("Full response so far: %s", fullResponse.String())
			log.Println("Received done signal from Ollama")
			break
		}
	}

	// Save final response
	log.Println("Stream complete, saving response")
	finalResponse := fullResponse.String()
	if finalResponse != "" {
		err := database.AddMessageWithThinking(
			client.currentConvoID,
			"assistant",
			finalResponse,
			rawContent.String(),
			pointerString(thinking.String()),
			&thinkingDuration,
		)
		if err != nil {
			log.Printf("Error saving response: %v", err)
			sendError(client, "Failed to save response")
			return
		}
		log.Printf("Response saved successfully for conversation: %s", client.currentConvoID)
	} else {
		log.Printf("Warning: Empty response received for conversation: %s", client.currentConvoID)
	}

	log.Printf("Response generation complete for conversation: %s", client.currentConvoID)
	client.conn.WriteJSON(WSResponse{
		Type:    "done",
		Content: "",
	})
}

func pointerString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sendError(client *Client, message string) {
	log.Printf("Sending error to client: %s", message)
	client.conn.WriteJSON(WSResponse{
		Type:    "error",
		Content: message,
	})
}
