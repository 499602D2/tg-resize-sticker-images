package queue

import (
	"sync"

	tb "gopkg.in/telebot.v3"
)

type Message struct {
	// A message that is created for SendQueue
	Recipient *tb.User        // Recipient of the message
	Bytes     *[]byte         // Photo, as a byte array
	Caption   string          // Caption for the photo
	Sopts     *tb.SendOptions // Send options
}

type SendQueue struct {
	/* Enforces a rate-limiter to stay within Telegram's send-rate boundaries */
	MessagesPerSecond float32    // Messages-per-second limit
	MessageQueue      []Message  // Queue of messages to send
	Mutex             sync.Mutex // Mutex to avoid concurrent writes
}

func (s *SendQueue) AddToQueue(message *Message) {
	s.Mutex.Lock()
	s.MessageQueue = append(s.MessageQueue, *message)
	s.Mutex.Unlock()
}
