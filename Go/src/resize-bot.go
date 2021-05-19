/*
golang rewrite of the telegram sticker resize bot python program.
*/

package main

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"log"
	"sort"
	"time"
	"bufio"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/dustin/go-humanize"
	"github.com/go-co-op/gocron"
	pngquant "github.com/yusukebe/go-pngquant"
	tb "gopkg.in/tucnak/telebot.v2"
)

func resizeImage(imgBytes []byte) ([]byte, string, error, string) {
	// Build image from buffer
	img, err := vips.NewImageFromBuffer(imgBytes)
	if err != nil {
		log.Println("⚠️ Error decoding image! Err: ", err)

		errorMsg := fmt.Sprintf("⚠️ Error decoding image: %s.", err.Error())
		if err.Error() == "unsupported image format" {
			errorMsg += " Please send jpg/png images."
		}

		return nil, errorMsg, err, ""
	}

	// Dimensions for resize (int)
	w, h := img.Width(), img.Height()

	// Determine the factor by how much to scale the image with (vips wants f64)
	var resScale float64

	if w >= h {
		resScale = 512.0 / float64(w)
	} else {
		resScale = 512.0 / float64(h)
	}

	// Resize, upscale status
	err = img.Resize(resScale, vips.KernelAuto)
	imgUpscaled := resScale > 1.0

	if err != nil {
		log.Println("⚠️ Error resizing image:", err)
	}

	// Increment compression ratio if size is too large
	pngParams := vips.PngExportParams{
		StripMetadata: true,
		Compression:   6,
		Interlace:     false,
	}

	// encode as png into a new buffer
	pngBuff, _, err := img.ExportPng(&pngParams)
	if err != nil {
		log.Fatal("⚠️ Error encoding image as png: ", err)

		if err.Error() == "unsupported image format" {
			return nil, "⚠️ Unsupported image format!", err, ""
		} else {
			return nil, fmt.Sprintf("⚠️ Error: %s", err.Error()), err, ""
		}
	}

	// Did we reach the target file size?
	compressionFailed := len(pngBuff)/1024 >= 512
	pngqStr := ""

	// if compression fails, run the image through pngquant
	if compressionFailed {
		pngqStr = " [Compressed]"
		expParams := vips.ExportParams{
			Format:        vips.ImageTypePNG,
			StripMetadata: true,
			Compression:   6,
		}

		imgImg, err := img.ToImage(&expParams)
		if err != nil {
			log.Println("⚠️ Error exporting image as image.Image:", err)
		}

		cImg, err := pngquant.Compress(imgImg, "6")
		if err != nil {
			log.Println("⚠️ Error compressing image with pngquant:", err)
		}

		// write to buffer
		cBuff := new(bytes.Buffer)
		err = png.Encode(cBuff, cImg)
		if err != nil {
			log.Println("⚠️ Error encoding cImg as png:", err)
		}

		pngBuff = cBuff.Bytes()
		compressionFailed = len(pngBuff)/1024 >= 512

		if compressionFailed {
			log.Println("\t⚠️ Image compression failed! Buffer length (KB):", len(cBuff.Bytes())/1024)
		}
	}

	// Construct the caption
	imgCaption := fmt.Sprintf(
		"🖼 Here's your sticker-ready image (%dx%d)! Forward this to @Stickers.",
		img.Width(), img.Height(),
	)

	// Add notice to user if image was upscaled or compressed
	if imgUpscaled {
		imgCaption += "\n\n⚠️ Image upscaled! Quality may have been lost: consider using a larger image."
	} else if compressionFailed {
		imgCaption += "\n\n⚠️ Image compression failed (≥512 KB): you must manually compress the image!"
	}

	img.Close()
	return pngBuff, imgCaption, nil, pngqStr
}

func getBytes(bot *tb.Bot, message *tb.Message, mediaType string) ([]byte, error) {
	// Get file from tg servers
	var err error
	var file io.ReadCloser

	if mediaType == "photo" {
		file, err = bot.GetFile(&message.Photo.File)
	} else {
		file, err = bot.GetFile(&message.Document.File)
	}

	if err != nil {
		log.Fatal("⚠️ Error running GetFile: ", err)
		return []byte{}, err
	}

	// Download (copy) to buffer
	var imgBuf bytes.Buffer
	_, err = io.Copy(&imgBuf, file)

	if err != nil {
		log.Println("⚠️ Error downloading image:", err)
		return []byte{}, err
	}

	return imgBuf.Bytes(), nil
}

func sendDocument(bot *tb.Bot, message *tb.Message, photo []byte, imgCaption string) {
	// Send as a document: create object
	doc := tb.Document{
		File:     tb.FromReader(bytes.NewReader(photo)),
		Caption:  imgCaption,
		MIME:     "image/png",
		FileName: fmt.Sprintf("resized-image-%d.png", time.Now().Unix()),
	}

	// Disable notifications
	sendOpts := tb.SendOptions{DisableNotification: true}

	// Send
	_, err := doc.Send(bot, message.Sender, &sendOpts)

	if err != nil {
		log.Println("⚠️ Error sending message:", err)
	}
}

func updateUniqueStat(uid *int, config *Config) {
	// uarr is always sorted when performing check
	i := sort.Search(
		len(config.UniqueUsers),
		func(i int) bool { return config.UniqueUsers[i] >= *uid },
	)

	if i < len(config.UniqueUsers) && config.UniqueUsers[i] == *uid {
		return // uid exists in the array
	} else {
		if len(config.UniqueUsers) == i {
			// nil or empty slice, or after last element
			config.UniqueUsers = append(config.UniqueUsers, *uid)
		} else if i == 0 {
			// if zeroth index, append
			config.UniqueUsers = append([]int{*uid}, config.UniqueUsers...)
		} else {
			// otherwise, we're inserting in the middle of the array
			config.UniqueUsers = append(config.UniqueUsers[:i+1], config.UniqueUsers[i:]...)
			config.UniqueUsers[i] = *uid
		}
	}

	// Stat++
	config.StatUniqueChats++
}

type Config struct {
	Token           string
	Owner           int
	StatConverted   int
	StatUniqueChats int
	StatStarted     int64
	UniqueUsers     []int
}

func dumpConfig(config *Config) {
	jsonbytes, err := json.MarshalIndent(*config, "", "\t")
	if err != nil {
		log.Fatalf("⚠️ Error marshaling json! Err: %s", err)
	}

	file, err := os.Create("botConfig.json")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// Write, close
	file.Write(jsonbytes)
	file.Close()
}

func loadConfig() Config {
	// Config doesn't exist: create
	if _, err := os.Stat("botConfig.json"); os.IsNotExist(err) {
		fmt.Print("\nEnter bot token: ")

		reader := bufio.NewReader(os.Stdin)
		inp, _ := reader.ReadString('\n')
		botToken := strings.TrimSuffix(inp, "\n")

		// Create, marshal
		config := Config{
			Token:           botToken,
			Owner:           0,
			StatConverted:   0,
			StatUniqueChats: 0,
			StatStarted:     time.Now().Unix(),
			UniqueUsers:     []int{},
		}

		dumpConfig(&config)
		return config
	}

	// Config exists: load
	fbytes, err := ioutil.ReadFile("botConfig.json")
	if err != nil {
		log.Println("⚠️ Error reading config file:", err)
		os.Exit(1)
	}

	// New config struct
	var config Config

	// Unmarshal into our config struct
	err = json.Unmarshal(fbytes, &config)
	if err != nil {
		log.Fatal("⚠️ Error unmarshaling config json: ", err)
		os.Exit(1)
	}

	// Set startup time
	config.StatStarted = time.Now().Unix()
	config.StatUniqueChats = len(config.UniqueUsers)

	// Sort UniqueChats, as they may be unsorted
	sort.Ints(config.UniqueUsers)

	return config
}

func buildStatsMsg(config *Config, vnum string) (string, tb.SendOptions) {
	// main stats
	msg := fmt.Sprintf(
		"📊 *Bot statistics*\nImages converted: %s\nUnique users seen: %s",
		humanize.Comma(int64(config.StatConverted)),
		humanize.Comma(int64(config.StatUniqueChats)),
	)

	// server info
	msg += fmt.Sprintf("\n\n*🎛 Server information*\nBot started %s",
		humanize.RelTime(time.Unix(config.StatStarted, 0), time.Now(), "ago", "ago"),
	)

	// Vnum, link
	msg += fmt.Sprintf(
		"\nRunning version [%s](https://github.com/499602D2/tg-resize-sticker-images)",
		vnum,
	)

	// Construct keyboard for refresh functionality
	kb := [][]tb.InlineButton{{tb.InlineButton{Text: "🔄 Refresh statistics", Data: "stats/refresh"}}}
	rplm := tb.ReplyMarkup{InlineKeyboard: kb}

	// Add Markdown parsing for a pretty link embed + keyboard
	sopts := tb.SendOptions{ParseMode: "Markdown", ReplyMarkup: &rplm}

	return msg, sopts
}

func main() {
	const vnum string = "2020.5.19"

	// Set-up logging
	logf, err := os.OpenFile("log-file.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
	}

	// Defer to close when you're done with it, not because you think it's idiomatic!
	defer logf.Close()

	// Set output of logs to f
	log.SetOutput(logf)

	// Version 0.1: 2020.3.29
	log.Println("go-resize-bot", vnum)
	log.Println("Bot started at", time.Now())

	config := loadConfig()
	bot, err := tb.NewBot(tb.Settings{
		Token:  config.Token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal("Error starting bot: ", err)
		return
	}

	// https://pkg.go.dev/github.com/davidbyttow/govips/v2@v2.6.0/vips#LoggingSettings
	vips.LoggingSettings(nil, vips.LogLevel(3))
	vips.Startup(nil)

	// Command handler for /start
	bot.Handle("/start", func(message *tb.Message) {
		startMessage := "🖼 Hi there! To use the bot, simply send an image to this chat (jpg/png)."
		bot.Send(message.Sender, startMessage)

		if message.Sender.ID != config.Owner {
			log.Println("🌟", message.Sender.ID, "bot added to new chat!")
		}
	})

	// Command handler for /help
	bot.Handle("/help", func(message *tb.Message) {
		helpMessage := "🖼 To use the bot, simply send your image to this chat (jpg/png)!"
		bot.Send(message.Sender, helpMessage)

		if message.Sender.ID != config.Owner {
			log.Println("🙋‍♂️", message.Sender.ID, "requested help!")
		}
	})

	// Keep track of the last chat to convert an image;
	// this should reduce updateUniqueStat checks a lot
	var lastUser int

	// Command handler for /stats
	bot.Handle("/stats", func(message *tb.Message) {
		msg, sopts := buildStatsMsg(&config, vnum)
		bot.Send(message.Sender, msg, &sopts)

		if message.Sender.ID != config.Owner {
			log.Println("📊", message.Sender.ID, "requested to view stats!")
		}
	})

	// Register photo handler
	bot.Handle(tb.OnPhoto, func(message *tb.Message) {
		// Download
		imgBytes, err := getBytes(bot, message, "photo")
		if err != nil {
			bot.Send(message.Sender, "⚠️ Error downloading image! Please try again.")
			return
		}

		// resize
		photo, imgCaption, err, pngqC := resizeImage(imgBytes)

		// Send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1

			if message.Sender.ID != config.Owner {
				log.Printf("🖼 %d successfully converted an image!%s\n", message.Sender.ID, pngqC)
			}
		}

		// Update stat for count of unique chats in a goroutine
		if message.Sender.ID != lastUser {
			go updateUniqueStat(&message.Sender.ID, &config)
			lastUser = message.Sender.ID
		}
	})

	// Register document handler
	bot.Handle(tb.OnDocument, func(message *tb.Message) {
		// Download
		imgBytes, err := getBytes(bot, message, "document")
		if err != nil {
			bot.Send(message.Sender, "⚠️ Error downloading image! Please try again.")
			return
		}

		// Resize
		photo, imgCaption, err, pngqC := resizeImage(imgBytes)

		// Send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1

			if message.Sender.ID != config.Owner {
				log.Printf("🖼 %d successfully converted an image!%s\n", message.Sender.ID, pngqC)
			}
		}

		// Update stat for count of unique chats in a goroutine
		if message.Sender.ID != lastUser {
			go updateUniqueStat(&message.Sender.ID, &config)
			lastUser = message.Sender.ID
		}
	})

	// Register handler for incoming callback queries (i.e. stats refresh)
	bot.Handle(tb.OnCallback, func(cb *tb.Callback) {
		if cb.Data == "stats/refresh" {
			msg, sopts := buildStatsMsg(&config, vnum)
			bot.Edit(cb.Message, msg, &sopts)

			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "🔄 Statistics refreshed!",
				ShowAlert:  false,
			}

			bot.Respond(cb, &resp)
		} else {
			log.Println("⚠️ Invalid callback data received:", cb.Data)
		}
	})

	// Dump statistics to disk once every 30 minutes
	scheduler := gocron.NewScheduler(time.UTC)
	scheduler.Every(30).Minutes().Do(dumpConfig, &config)
	scheduler.StartAsync()

	bot.Start()
}
