package bots

import (
	"log"
	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/queue"
	"tg-resize-sticker-images/spam"
	"tg-resize-sticker-images/stats"
	"tg-resize-sticker-images/utils"

	"time"

	tb "gopkg.in/telebot.v3"
)

func MessageSender(session *config.Session) {
	/* Function clears the SendQueue and stays within API limits while doing so */
	sleepDuration := time.Millisecond * time.Duration(1.0/session.Queue.MessagesPerSecond*1000.0)

	for {
		// If queue is not empty, clear it
		if len(session.Queue.MessageQueue) != 0 {
			// Lock sendQueue for parsing
			session.Queue.Mutex.Lock()

			// Iterate over queue
			for i, msg := range session.Queue.MessageQueue {
				// If nil bytes, we are only sending text
				if msg.Bytes == nil {
					_, err := session.Bot.Send(msg.Recipient, msg.Caption, &msg.Sopts)
					if err != nil {
						log.Println("Error sending non-bytes message in messageSender:", err)
					}
				} else {
					// If non-nil bytes, we are sending a photo
					sendDocument(session, &msg)
				}

				// Sleep long enough to stay within API limits
				if i < len(session.Queue.MessageQueue)-1 {
					time.Sleep(sleepDuration)
				}
			}

			// Clear queue
			session.Queue.MessageQueue = nil

			// Batch send done, unlock sendQueue
			session.Queue.Mutex.Unlock()
		}

		// Sleep while waiting for updates
		time.Sleep(time.Millisecond * 500)
	}
}

func SetupBot(session *config.Session) {
	// Pull pointers from session for cleaner code
	bot := session.Bot
	aspam := session.Spam

	// Command handler for /start
	bot.Handle("/start", func(c tb.Context) error {
		// Anti-spam
		message := c.Message()
		if !spam.CommandPreHandler(aspam, message.Sender.ID, message.Unixtime) {
			return nil
		}

		// Construct message
		startMessage := utils.HelpMessage(message, aspam)
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   startMessage,
			Sopts:     tb.SendOptions{ParseMode: "Markdown"},
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		// Check if the chat is actually new, or just calling /start again
		if !stats.ChatExists(&message.Sender.ID, session.Config) {
			log.Println("🌟", message.Sender.ID, "bot added to new chat!")
		}

		return nil
	})

	// Command handler for /help
	bot.Handle("/help", func(c tb.Context) error {
		// Pointer to message
		message := c.Message()

		// Anti-spam
		if !spam.CommandPreHandler(aspam, message.Sender.ID, message.Unixtime) {
			return nil
		}

		// Refresh ConversionCount for chat
		spam.RefreshConversions(aspam, message.Sender.ID)

		// Help message
		helpMessage := utils.HelpMessage(message, aspam)

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
			log.Println("🙋", message.Sender.ID, "requested help!")
		}

		return nil
	})

	// Command handler for /stats
	bot.Handle("/stats", func(c tb.Context) error {
		// Pointer to message
		message := c.Message()

		// Anti-spam
		if !spam.CommandPreHandler(aspam, message.Sender.ID, message.Unixtime) {
			return nil
		}

		// Get stats message
		caption, sopts := stats.BuildStatsMsg(session.Config, aspam, session.Vnum)

		// Construct message
		msg := queue.Message{Recipient: message.Sender, Bytes: nil, Caption: caption, Sopts: sopts}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		if message.Sender.ID != session.Config.Owner {
			log.Println("📊", message.Sender.ID, "requested to view stats!")
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

		// Anti-spam
		if !spam.CommandPreHandler(aspam, cb.Sender.ID, time.Now().Unix()) {
			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "⚠️ Please do not spam the bot for no reason.",
				ShowAlert:  true,
			}

			err := bot.Respond(cb, &resp)
			if err != nil {
				log.Println("Error responding to callback!", err)
			}

			return nil
		}

		if cb.Data == "stats/refresh" {
			// Create updated message
			msg, sopts := stats.BuildStatsMsg(session.Config, aspam, session.Vnum)

			// Edit message with new content
			_, err := bot.Edit(cb.Message, msg, &sopts)
			if err != nil {
				log.Println("Error editing stats message!", err)
				return nil
			}

			// Callback response
			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "🔄 Statistics refreshed!",
				ShowAlert:  false,
			}

			err = bot.Respond(cb, &resp)
			if err != nil {
				log.Println("Error responding to callback!", err)
			}

		} else {
			log.Println("⚠️ Invalid callback data received:", cb.Data)
		}

		return nil
	})
}
