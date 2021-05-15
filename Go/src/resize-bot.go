/*
golang rewrite of the telegram sticker resize bot python program.
*/

package main

import (
	"fmt"
	"log"
	"time"
	"bytes"
	"io"

	"os"
	"bufio"
	"strings"
	"io/ioutil"
	"encoding/json"

	"github.com/go-co-op/gocron"
	"github.com/dustin/go-humanize"
	"github.com/davidbyttow/govips/v2/vips"
	tb "gopkg.in/tucnak/telebot.v2"
)

func resizeImage(imgBytes []byte) ([]byte, string, error) {
	// build image from buffer
	img, err := vips.NewImageFromBuffer(imgBytes)
	if err != nil {
		fmt.Println("Error decoding image! Err: ", err)

		errorMsg := fmt.Sprintf("⚠️ Error decoding image: %s.", err.Error())
		if err.Error() == "unsupported image format" {
			errorMsg += " Please send jpg/png images."
		}

		return nil, errorMsg, err
	}

	// w, h for resize (int)
	w, h := img.Width(), img.Height()

	// determine the factor by how much to scale the image with (vips wants int64)
	var resScale float64

	if w >= h {
		resScale = 512.0 / float64(w)
	} else {
		resScale = 512.0 / float64(h)
	}

	// resize, upscale status
	err = img.Resize(resScale, -1)
	imgUpscaled := resScale > 1

	// construct params for png export
	var pngBuff []byte
	compressionFailed := false
	compression := 0

	// compress image
	for {
		pngParams := vips.PngExportParams{
			StripMetadata:	true,
			Compression:   	compression,
			Interlace:     	false,
		}

		// encode as png into a new buffer
		pngBuff, _, err = img.ExportPng(&pngParams)
		if err != nil {
			log.Fatal("Error encoding image as png: ", err)
			if err.Error() == "unsupported image format" {
				return nil, "⚠️ Unsupported image format!", err
			}
		}

		// check filesize is within limits (max. 512 KB)
		if len(pngBuff) / 1024 >= 512 {
			if compression < 10 {
				compression++; continue;
			} else {
				compressionFailed = true
				break
			}
		} else {
			break
		}
	}

	// construct the caption
	imgCaption := fmt.Sprintf(
		"🖼 Here's your sticker-ready image (%dx%d)! Forward this to @Stickers.",
		img.Width(), img.Height(),
	)

	// add notice to user if image was upscaled or compressed
	if imgUpscaled == true {
		imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
	} else if compressionFailed == true {
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	// adios
	img.Close()
	return pngBuff, imgCaption, nil
}

func getBytes(bot *tb.Bot, message *tb.Message, mediaType string) []byte {
	// get file
	var err error; var file io.ReadCloser;
	if mediaType == "photo" {
		file, err = bot.GetFile(&message.Photo.File)
	} else {
		file, err = bot.GetFile(&message.Document.File)
	}

	if err != nil {
		log.Fatal("Error running GetFile: ", err)
	}

	// download to buffer
	var imgBuf bytes.Buffer
	_, err = io.Copy(&imgBuf, file)

	return imgBuf.Bytes()
}

func sendDocument(bot *tb.Bot, message *tb.Message, photo []byte, imgCaption string) {
	// send as a document: create object
	doc := tb.Document{
		File: 		tb.FromReader(bytes.NewReader(photo)),
		Caption: 	imgCaption,
		MIME: 		"image/png",
		FileName: 	fmt.Sprintf("resized-image-%d.png", time.Now().Unix()),
	}

	// disable notifications
	sendOpts := tb.SendOptions{ DisableNotification: true, }

	// send
	_, err := doc.Send(bot, message.Sender, &sendOpts)

	if err != nil {
		fmt.Println("Error sending message:", err)
	}
}

type Config struct {
	Token 			string
	StatConverted 	uint32
	StatStarted		int64
}

func dumpConfig(config *Config) {
	jsonbytes, err := json.MarshalIndent(*config, "", "\t")
	if err != nil {
		log.Fatalf("Error marshaling json! Err: %s", err)
	}

	file, err := os.Create("botConfig.json")
	if err != nil {
		log.Fatal(err); os.Exit(1)
	}

	// write, close
	file.Write(jsonbytes); file.Close()
}

func loadToken() Config {
	// config doesn't exist: create
	if _, err := os.Stat("botConfig.json"); os.IsNotExist(err) {
		fmt.Print("\nEnter bot token: ")
		reader := bufio.NewReader(os.Stdin)
		inp, _ := reader.ReadString('\n')
		botToken := strings.TrimSuffix(inp, "\n")

		// marshal
		config := Config { Token: botToken, StatConverted: 0, StatStarted: time.Now().Unix() }
		dumpConfig(&config)
		return config
	}

	// config exists: load
	fbytes, err := ioutil.ReadFile("botConfig.json")
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
		os.Exit(1)
	}

	// new config struct
	var config Config

	// unmarshal into our config struct
	json.Unmarshal(fbytes, &config)

	// set startup time
	config.StatStarted = time.Now().Unix()

	return config
}

func main() {
	// version 0.1: 2020.3.29
	fmt.Println("go-resize-bot 2020.5.15")

	config := loadToken()
	bot, err := tb.NewBot(tb.Settings{
		Token: config.Token,
		Poller: &tb.LongPoller{ Timeout: 10 * time.Second },
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	// https://pkg.go.dev/github.com/davidbyttow/govips/v2@v2.6.0/vips#LoggingSettings
	vips.LoggingSettings(nil, vips.LogLevel(3))

	// command handler for /stats
	bot.Handle("/stats", func(message *tb.Message) {
		msg := fmt.Sprintf(
			"📊 *Bot statistics*\nImages converted: %d\nBot started %s",
			config.StatConverted,
			humanize.RelTime(time.Unix(config.StatStarted, 0), time.Now(), "ago", "ago"),
		)

		// add Markdown parsing for a pretty link embed
		sopts := tb.SendOptions{ ParseMode: "Markdown", }

		// add link to github
		msg += "\n\n🐙 Source code is on [Github!](https://github.com/499602D2/tg-resize-sticker-images)"
		bot.Send(message.Sender, msg, &sopts)
	})

	// command handler for /stats
	bot.Handle("/help", func(message *tb.Message) {
		helpMessage := "🖼 To use the bot, simply send your image to this chat (jpg/png)."
		bot.Send(message.Sender, helpMessage)
	})

	// register photo handler
	bot.Handle(tb.OnPhoto, func(message *tb.Message) {
		// resize photo
		photo, imgCaption, err := resizeImage(getBytes(bot, message, "photo"))

		// send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1
		}
	})

	// register document handler
	bot.Handle(tb.OnDocument, func(message *tb.Message) {
		// resize photo
		photo, imgCaption, err := resizeImage(getBytes(bot, message, "document"))

		// send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1
		}
	})

	// dump statistics to disk once every 30 minutes
	scheduler := gocron.NewScheduler(time.UTC)
	scheduler.Every(30).Minutes().Do(dumpConfig, &config)
	scheduler.StartAsync()

	bot.Start()
}
