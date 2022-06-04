package queue

import (
	"sync"

	"golang.org/x/time/rate"
	tb "gopkg.in/telebot.v3"
)

// A message that is created for SendQueue
type Message struct {
	Recipient *tb.User       // Recipient of the message
	Bytes     *[]byte        // Photo, as a byte array
	Caption   string         // Caption for the photo
	Sopts     tb.SendOptions // Send options
}

// Enforces a rate-limiter to stay within Telegram's send-rate boundaries
type SendQueue struct {
	MessageQueue []Message     // Queue of messages to send
	Limiter      *rate.Limiter // Rate-limiter
	Mutex        sync.Mutex    // Mutex to avoid concurrent writes
}

// Adds a message to the send-queue
func (queue *SendQueue) AddToQueue(message *Message) {
	queue.Mutex.Lock()
	queue.MessageQueue = append(queue.MessageQueue, *message)
	queue.Mutex.Unlock()
}
