package bots

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/queue"
	"tg-resize-sticker-images/resize"
	"tg-resize-sticker-images/spam"
	"tg-resize-sticker-images/stats"
	"tg-resize-sticker-images/utils"
	"time"

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
		log.Println("⚠️ Error sending message in sendDocument (notifying user):", err)

		_, err := session.Bot.Send(msg.Recipient, "🚦 Error sending resized image! Please try again.")
		if err != nil {
			log.Println("\tUnable to notify user about send failure:", err)
		}

		return
	}

	// If message is successfully sent, +1 conversion
	stats.StatsPlusOneConversion(session.Config)
}

func getBytes(session *config.Session, message *tb.Message, mediaType string) (*bytes.Buffer, error) {
	var err error
	var tbFile *tb.File
	var file io.Reader

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
	file, err = session.Bot.File(tbFile)

	if err != nil {
		log.Println("⚠️ Error running GetFile: ", err)
		return &bytes.Buffer{}, err
	}

	// Copy file contents to imgBuf
	var imgBuf bytes.Buffer
	_, err = io.Copy(&imgBuf, file)

	if err != nil {
		log.Println("⚠️ Error copying image to buffer:", err)
		return &bytes.Buffer{}, err
	}

	return &imgBuf, nil
}

func handleIncomingMedia(session *config.Session, message *tb.Message, mediaType string) {
	/* Handles incoming media, i.e. those caught by tb.OnPhoto, tb.OnDocument etc. */
	// Anti-spam: return if user is not allowed to convert
	if !spam.ConversionPreHandler(session.Spam, message.Sender.ID) {
		log.Println("🚦 Chat", message.Sender.ID, "is ratelimited")

		// Construct message
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   utils.RatelimitedMessage(session.Spam, message.Sender.ID),
			Sopts:     tb.SendOptions{ParseMode: "Markdown"},
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)
		return
	}

	// Download
	imgBytes, err := getBytes(session, message, mediaType)
	if err != nil {
		// Construct message
		msg := queue.Message{
			Recipient: message.Sender,
			Bytes:     nil,
			Caption:   "⚠️ Error downloading image! Please try again.",
		}

		// Add to send queue
		session.Queue.AddToQueue(&msg)

		// Log error
		log.Printf("Error downloading image: %s\n", err.Error())
		return
	}

	// Resize, set message recipient
	msg, err := resize.ResizeImage(imgBytes)
	msg.Recipient = message.Sender

	// Log errors encountered during resize
	if err != nil {
		log.Printf("Error resizing image (handleIncomingMedia): %s\n", err.Error())
	}

	// Add to send queue: regardless of resize outcome, the message is sent
	session.Queue.AddToQueue(msg)

	// Update stat for count of unique chats in a goroutine
	if message.Sender.ID != session.LastUser {
		stats.UpdateUniqueStat(&message.Sender.ID, session.Config)
		stats.UpdateLastUserId(session, message.Sender.ID)
	}
}
