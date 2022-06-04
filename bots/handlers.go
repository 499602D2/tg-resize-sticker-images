package bots

import (
	"bytes"
	"fmt"
	"io"
	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/queue"
	"tg-resize-sticker-images/resize"
	"tg-resize-sticker-images/spam"
	"tg-resize-sticker-images/stats"
	"tg-resize-sticker-images/templates"
	"time"

	"github.com/rs/zerolog/log"

	tb "gopkg.in/telebot.v3"
)

func sendDocument(session *config.Session, msg *queue.Message) {
	// Send as a document: create object
	doc := tb.Document{
		File:     tb.FromReader(bytes.NewReader(*msg.Bytes)),
		Caption:  msg.Caption,
		MIME:     "image/png",
		FileName: fmt.Sprintf("resized-image-%d.png", time.Now().Unix()),
	}

	// Disable notifications
	sendOpts := tb.SendOptions{DisableNotification: true}

	// Send
	_, err := doc.Send(session.Bot, msg.Recipient, &sendOpts)

	if err != nil {
		log.Error().Err(err).Msg("⚠️ Error sending message in sendDocument (notifying user)")

		_, err := session.Bot.Send(msg.Recipient, "🚦 Error sending resized image! Please try again.")

		if err != nil {
			log.Error().Err(err).Msg("Unable to notify user about send failure")
		}

		return
	}

	// If message is successfully sent, +1 conversion
	stats.StatsPlusOneConversion(session.Config)
}

func getBytes(session *config.Session, message *tb.Message, mediaType string) (*bytes.Buffer, error) {
	// Variables
	var tbFile *tb.File

	// Get file with a method corresponding to the media type
	switch mediaType {
	case "photo":
		tbFile = message.Photo.MediaFile()
	case "document":
		tbFile = message.Document.MediaFile()
	case "sticker":
		tbFile = message.Sticker.MediaFile()
	}

	// Get file
	file, err := session.Bot.File(tbFile)

	if err != nil {
		log.Error().Err(err).Msg("⚠️ Error running GetFile")
		return &bytes.Buffer{}, err
	}

	// Copy file contents to imgBuf
	var imgBuf bytes.Buffer
	_, err = io.Copy(&imgBuf, file)

	if err != nil {
		log.Error().Err(err).Msg("⚠️ Error copying image to buffer")
		return &bytes.Buffer{}, err
	}

	return &imgBuf, nil
}

// Handles incoming media, i.e. those caught by tb.OnPhoto, tb.OnDocument etc.
func handleIncomingMedia(session *config.Session, message *tb.Message, mediaType string) {
	// Anti-spam: return if user is not allowed to convert
	if !spam.ConversionPreHandler(session.Spam, message.Sender.ID) {
		log.Debug().Msgf("🚦 Chat %d is ratelimited", message.Sender.ID)

		// Extract pointer to user's spam log
		userSpam := session.Spam.ChatConversionLog[message.Sender.ID]

		if userSpam.RateLimitMessageSent {
			/* Check if the message has already been sent recently
			This simply avoids spamming the same rate-limit message 50 times. */
			if time.Since(userSpam.RateLimitMessageSentAt) < time.Minute {
				log.Debug().Msgf("Rate-limit message for %d has been already sent recently, not sending again",
					message.Sender.ID)
				return
			}
		}

		// Construct message
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   templates.RatelimitedMessage(session.Spam, message.Sender.ID),
			Sopts:     tb.SendOptions{ParseMode: "Markdown"},
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		// Update the RateLimitMessageSent flag + time sent at
		session.Spam.ChatReceivedRateLimitMessage(message.Sender.ID)

		return
	}

	// Download
	imgBytes, err := getBytes(session, message, mediaType)

	if err != nil {
		var caption string
		if err == tb.ErrTooLarge {
			caption = "File is too large! Try compressing it first."
		} else {
			caption = "Error downloading image! Please try again."
		}

		// Construct error message
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   fmt.Sprintf("⚠️ %s", caption),
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		// Log error
		log.Error().Err(err).Msgf("Error downloading image (caption='%s')", caption)
		return
	}

	// Resize, set message recipient
	msg, _ := resize.ResizeImage(imgBytes)
	msg.Recipient = message.Sender

	// Add to send queue: regardless of resize outcome, the message is sent
	session.Queue.AddToQueue(msg)

	// Update stat for count of unique chats in a goroutine
	if message.Sender.ID != session.LastUser {
		stats.UpdateUniqueStat(message.Sender.ID, session.Config)
		stats.UpdateLastUserId(session, message.Sender.ID)
	}
}
