package bots

import (
	"context"
	"strings"
	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/queue"
	"tg-resize-sticker-images/spam"
	"tg-resize-sticker-images/stats"
	"tg-resize-sticker-images/templates"

	"time"

	"github.com/rs/zerolog/log"

	tb "gopkg.in/telebot.v3"
)

// Function clears the SendQueue and stays within API limits while doing so
func MessageSender(session *config.Session) {
	for {
		// If queue is not empty, clear it
		if len(session.Queue.MessageQueue) != 0 {
			// Lock sendQueue for parsing
			session.Queue.Mutex.Lock()

			// Iterate over queue
			for _, msg := range session.Queue.MessageQueue {
				// If nil bytes, we are only sending text
				if msg.Bytes == nil {
					// Text-only, take one token from the pool (limits to 20 msg/sec)
					err := session.Queue.Limiter.WaitN(context.Background(), 1)

					if err != nil {
						log.Error().Err(err).Msg("Running limiter.WaitN failed in text-only sender")
					}

					// Send text only
					_, err = session.Bot.Send(msg.Recipient, msg.Caption, &msg.Sopts)

					if err != nil {
						log.Error().Err(err).Msg("Error sending non-bytes message in messageSender")
					}
				} else {
					// Photo, take two tokens from the pool (limits to 10 msg/sec)
					err := session.Queue.Limiter.WaitN(context.Background(), 2)

					if err != nil {
						log.Error().Err(err).Msg("Running limiter.WaitN failed in bytes sender")
					}

					// If non-nil bytes, we are sending a photo
					sendDocument(session, &msg)
				}
			}

			// Clear queue
			session.Queue.MessageQueue = nil

			// Batch send done, unlock sendQueue
			session.Queue.Mutex.Unlock()
		}

		// Sleep while waiting for updates
		time.Sleep(time.Millisecond * 50)
	}
}

func SetupBot(session *config.Session) {
	// Pull pointers from session for cleaner code
	bot, aspam := session.Bot, session.Spam

	// Command handler for /start
	bot.Handle("/start", func(c tb.Context) error {
		// Anti-spam
		message := c.Message()

		// Run rate-limiter
		session.Spam.RunUserLimiter(message.Sender.ID, 1)

		// Construct message
		startMessage := templates.HelpMessage(message, aspam)
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   startMessage,
			Sopts:     tb.SendOptions{ParseMode: "Markdown"},
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		// Check if the chat is actually new, or just calling /start again
		if !stats.ChatExists(message.Sender.ID, session.Config) {
			log.Info().Msgf("🌟 %d bot added to new chat", message.Sender.ID)
		}

		return nil
	})

	// Command handler for /help
	bot.Handle("/help", func(c tb.Context) error {
		// Pointer to message
		message := c.Message()

		// Run rate-limiter
		session.Spam.RunUserLimiter(message.Sender.ID, 1)

		// Refresh ConversionCount for chat
		spam.RefreshConversions(aspam, message.Sender.ID)

		// Help message
		helpMessage := templates.HelpMessage(message, aspam)

		// Construct message
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   helpMessage,
			Sopts:     tb.SendOptions{ParseMode: "Markdown"},
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		if message.Sender.ID != session.Config.Owner {
			log.Info().Msgf("🙋 %d requested help", message.Sender.ID)
		}

		return nil
	})

	// Command handler for /stats
	bot.Handle("/stats", func(c tb.Context) error {
		// Pointer to message
		message := c.Message()

		// Run rate-limiter
		session.Spam.RunUserLimiter(message.Sender.ID, 1)

		// Get stats message
		caption, sopts := stats.BuildStatsMsg(session.Config, aspam, session.Vnum)

		// Construct message
		msg := queue.Message{Recipient: message.Sender, Bytes: nil, Caption: caption, Sopts: sopts}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		if message.Sender.ID != session.Config.Owner {
			log.Info().Msgf("📊 %d requested to view stats", message.Sender.ID)
		}

		return nil
	})

	// Register photo handler
	bot.Handle(tb.OnPhoto, func(c tb.Context) error {
		handleIncomingMedia(session, c.Message(), "photo")
		return nil
	})

	// Register document handler
	bot.Handle(tb.OnDocument, func(c tb.Context) error {
		handleIncomingMedia(session, c.Message(), "document")
		return nil
	})

	// Register sticker handler
	bot.Handle(tb.OnSticker, func(c tb.Context) error {
		handleIncomingMedia(session, c.Message(), "sticker")
		return nil
	})

	// Register handler for incoming callback queries (i.e. stats refresh)
	bot.Handle(tb.OnCallback, func(c tb.Context) error {
		// Pointer to received callback
		cb := c.Callback()

		if cb.Data == "stats/refresh" {
			// Run rate-limiter
			session.Spam.RunUserLimiter(cb.Sender.ID, 1)

			// Create updated message
			msg, sopts := stats.BuildStatsMsg(session.Config, aspam, session.Vnum)

			// Edit message with new content if the messages aren't identical
			_, err := bot.Edit(cb.Message, msg, &sopts)

			if err != nil {
				if !strings.Contains(err.Error(), "message is not modified") {
					// If error is caused by something other than message not being modified, return
					log.Error().Err(err).Msg("Error editing stats message")
					return nil
				}
			}

			// Callback response
			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "🔄 Statistics refreshed",
				ShowAlert:  false,
			}

			err = bot.Respond(cb, &resp)

			if err != nil {
				log.Error().Err(err).Msg("Error responding to callback")
			}

		} else {
			log.Error().Msgf("⚠️ Invalid callback data received: %s", cb.Data)
		}

		return nil
	})
}
