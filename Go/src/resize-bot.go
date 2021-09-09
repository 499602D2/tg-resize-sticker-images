/*
golang rewrite of the telegram sticker resize bot python program.
*/

package main

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tg-resize-sticker-images/utils"

	"github.com/dustin/go-humanize/english"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/go-co-op/gocron"
	pngquant "github.com/yusukebe/go-pngquant"
	tb "gopkg.in/tucnak/telebot.v2"
)

func resizeImage(imgBytes []byte) ([]byte, string, error, string) {
	// Build image from buffer
	img, err := vips.NewImageFromBuffer(imgBytes)
	if err != nil {
		go log.Println("⚠️ Error decoding image! Err: ", err)

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
		go log.Println("⚠️ Error resizing image:", err)
	}

	// Increment compression ratio if size is too large
	pngParams := vips.PngExportParams{
		StripMetadata: true,
		Compression:   6,
		Interlace:     false,
	}

	// Encode as png into a new buffer
	pngBuff, _, err := img.ExportPng(&pngParams)
	if err != nil {
		go log.Println("⚠️ Error encoding image as png: ", err)

		if err.Error() == "unsupported image format" {
			return nil, "⚠️ Unsupported image format!", err, ""
		} else {
			return nil, fmt.Sprintf("⚠️ Error: %s", err.Error()), err, ""
		}
	}

	// Did we reach the target file size?
	compressionFailed := len(pngBuff)/1024 >= 512
	pngqStr := ""

	// If compression fails, run the image through pngquant
	if compressionFailed {
		pngqStr = " [Compressed]"
		expParams := vips.ExportParams{
			Format:        vips.ImageTypePNG,
			StripMetadata: true,
			Compression:   6,
		}

		imgImg, err := img.ToImage(&expParams)
		if err != nil {
			go log.Println("⚠️ Error exporting image as image.Image:", err)
		}

		cImg, err := pngquant.Compress(imgImg, "6")
		if err != nil {
			go log.Println("⚠️ Error compressing image with pngquant:", err)
		}

		// Write to buffer
		cBuff := new(bytes.Buffer)
		err = png.Encode(cBuff, cImg)
		if err != nil {
			go log.Println("⚠️ Error encoding cImg as png:", err)
		}

		pngBuff = cBuff.Bytes()
		compressionFailed = len(pngBuff)/1024 >= 512

		if compressionFailed {
			go log.Println("\t⚠️ Image compression failed! Buffer length (KB):", len(cBuff.Bytes())/1024)
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

func getBytes(bot *tb.Bot, message *tb.Message, mediaType string, config *utils.Config) ([]byte, error) {
	// If using local API, no need to get file: open from disk and return bytes
	if config.API.LocalAPIEnabled {
		var err error
		var file tb.File

		// Get file, store
		if mediaType == "photo" {
			file, err = bot.FileByID(message.Photo.File.FileID)
		} else {
			file, err = bot.FileByID(message.Document.File.FileID)
		}

		if err != nil {
			go log.Println("⚠️ Error running GetFile (local): ", err)
			go log.Printf("File: %+v\n", file)
			return []byte{}, err
		}

		// Construct path from config's working directory
		fPath := filepath.Join(config.API.LocalWorkingDir, config.Token, file.FilePath)
		if err != nil {
			go log.Println("Error creating absolute path:", err)
			return []byte{}, err
		}

		// Attempt reading file contents
		imgBuf, err := ioutil.ReadFile(fPath)

		if err != nil {
			go log.Println("⚠️ Error opening local file from FilePath!", message.Photo.FilePath)
			go log.Println("Constructed fPath:", fPath)
			go log.Println("Err:", err)

			// Error: remove file, return
			os.Remove(fPath)
			return []byte{}, err
		}

		// Success: remove file, return
		os.Remove(fPath)
		return imgBuf, nil

	} else {
		// Else, we're using the regular Telegram bot API: get file from servers
		var err error
		var file io.ReadCloser

		if mediaType == "photo" {
			file, err = bot.GetFile(&message.Photo.File)
		} else {
			file, err = bot.GetFile(&message.Document.File)
		}

		defer file.Close()
		if err != nil {
			go log.Println("⚠️ Error running GetFile: ", err)
			return []byte{}, err
		}

		// Download or copy to buffer, depending on API used
		var imgBuf bytes.Buffer
		_, err = io.Copy(&imgBuf, file)

		if err != nil {
			go log.Println("⚠️ Error copying image to buffer:", err)
			return []byte{}, err
		}

		return imgBuf.Bytes(), nil
	}
}

func sendDocument(bot *tb.Bot, message *tb.Message, photo []byte, imgCaption string) (bool) {
	// Send as a document: create object
	doc := tb.Document{
		File:     tb.FromReader(bytes.NewReader(photo)),
		Caption:  imgCaption,
		MIME:     "image/png",
		FileName: fmt.Sprintf("resized-image-%d.png", time.Now().Unix()),
	}

	// Disable notifications
	sendOpts := tb.SendOptions{ DisableNotification: true }

	// Send
	_, err := doc.Send(bot, message.Sender, &sendOpts)

	if err != nil {
		go log.Println("⚠️ Error sending message (notifying user):", err)
		errorMessage := fmt.Sprintf("🚦 Error sending processed image: %s", err)

		_, err := bot.Send(message.Sender, errorMessage)
		if err != nil {
			go log.Println("\tUnable to notify user:", err)
		}

		return false
	}

	return true
}

func setupSignalHandler(config *utils.Config) {
	// Listens for incoming interrupt signals, dumps config if detected
	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGINT)

	go func() {
		<-channel
		go log.Println("🚦 Received interrupt signal: dumping config...")
		utils.DumpConfig(config)
		os.Exit(0)
	}()
}

func main() {
	/* Version history
	0.0.0: 2021.03.29: started
	1.0.0: 2021.05.15: first go implementation
	1.1.0: 2021.05.16: keeping track of unique chats, binsearch
	1.2.0: 2021.05.17: callback buttons for /stats
	1.3.0: 2021.05.17: image compression with pngquant
	1.3.1: 2021.05.19: bug fixes, error handling
	1.4.0: 2021.08.22: error handling, local API support, handle interrupts 
	1.4.1: 2021.08.25: logging changes to reduce disk writes
	1.5.0: 2021.08.30: added anti-spam measures, split the program into modules
	1.5.1: 2021.09.01: fix concurrent map writes 
	1.5.2: 2021.09.09: improvements to spam management */
	const vnum string = "1.5.2 (2021.09.09)"

	// Log file
	wd, _ := os.Getwd()
	logPath := filepath.Join(wd, "logs")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		_ = os.Mkdir(logPath, os.ModePerm)
	}

	// Set-up logging
	logFilePath := filepath.Join(logPath, "log.txt")
	logf, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		go log.Println(err)
	}

	// Set output of logs to f
	defer logf.Close()
	log.SetOutput(logf)
	go log.Printf("🤖 [%s] Bot started at %s", vnum, time.Now())

	// Load (or create) config
	config := utils.LoadConfig()

	// Setup anti-spam
	Spam := utils.AntiSpam{}
	Spam.ChatBannedUntilTimestamp = make(map[int]int)
	Spam.ChatConversionLog = make(map[int]utils.ConversionLog)
	Spam.ChatBanned = make(map[int]bool)
	Spam.Rules = make(map[string]int64)

	// Add rules
	Spam.Rules["ConversionsPerHour"] = int64(config.ConversionRate)
	Spam.Rules["TimeBetweenCommands"] = 2

	// Setup signal handler
	setupSignalHandler(&config)

	// Verify we're logged out if we're using the cloud API
	if config.API.LocalAPIEnabled && !config.API.CloudAPILoggedOut {
		go log.Println("🚦 Local bot API enabled: logging out from cloud API servers...")

		// Start bot in regular mode
		bot, err := tb.NewBot(tb.Settings{
			URL:    "https://api.telegram.org",
			Token:  config.Token,
			Poller: &tb.LongPoller{ Timeout: 10 * time.Second },
		})

		if err != nil {
			go log.Println("Error starting bot during logout:", err)
			return
		}

		// Logout from the cloud API server
		success, err := bot.Logout()

		if success {
			go log.Println("✅ Successfully logged out from the cloud API server!")
		} else {
			go log.Println("⚠️ Error logging out from the server:", err)
			return
		}

		// Success: update config, dump
		config.API.CloudAPILoggedOut = true
		utils.DumpConfig(&config)

		go log.Println("✅ Config updated to use local API!")

		// Warn if working directory is unset
		if config.API.LocalWorkingDir == "working/dir/on/server" || config.API.LocalWorkingDir == "" {
			log.Fatal("⚠️ Local API is enabled, but LocalWorkingDir is unset! Images cannot be downloaded.")
		}

		fmt.Println("✅ Logged out from cloud API server: please restart the program.")
		os.Exit(0)
	}

	bot, err := tb.NewBot(tb.Settings{
		URL:    config.API.URL,
		Token:  config.Token,
		Poller: &tb.LongPoller{ Timeout: 10 * time.Second },
	})

	if err != nil {
		log.Fatal("Error starting bot:", err)
		return
	}

	// https://pkg.go.dev/github.com/davidbyttow/govips/v2@v2.6.0/vips#LoggingSettings
	vips.LoggingSettings(nil, vips.LogLevel(3))
	vips.Startup(nil)

	// Command handler for /start
	bot.Handle("/start", func(message *tb.Message) {
		// Anti-spam
		if !utils.CommandPreHandler(&Spam, message.Sender.ID, message.Unixtime) {
			return
		}

		startMessage := "🖼 Hi there! To use the bot, simply send an image to this chat (jpg/png)."
		bot.Send(message.Sender, startMessage)

		if message.Sender.ID != config.Owner {
			fmt.Println("🌟", message.Sender.ID, "bot added to new chat!")
		}
	})

	// Command handler for /help
	bot.Handle("/help", func(message *tb.Message) {
		// Anti-spam
		if !utils.CommandPreHandler(&Spam, message.Sender.ID, message.Unixtime) {
			return
		}

		// Refresh ConversionCount for chat
		utils.RefreshConversions(&Spam, message.Sender.ID)

		helpMessage := "🖼 To use the bot, simply send your image to this chat! (JPG/PNG)"
		helpMessage += fmt.Sprintf(
			"\n\n*Note:* you can convert up to %d images per hour.",
			Spam.Rules["ConversionsPerHour"])

		helpMessage += fmt.Sprintf(
			" You have done %s during the last hour.",
			english.Plural(
				Spam.ChatConversionLog[message.Sender.ID].ConversionCount, "conversion", ""))

		bot.Send(message.Sender, helpMessage, "Markdown")

		if message.Sender.ID != config.Owner {
			fmt.Println("🙋‍♂️", message.Sender.ID, "requested help!")
		}
	})

	// Keep track of the last chat to convert an image;
	// this should reduce UpdateUniqueStat checks a lot
	var lastUser int

	// Command handler for /stats
	bot.Handle("/stats", func(message *tb.Message) {
		// Anti-spam
		if !utils.CommandPreHandler(&Spam, message.Sender.ID, message.Unixtime) {
			return
		}

		msg, sopts := utils.BuildStatsMsg(&config, vnum)
		bot.Send(message.Sender, msg, &sopts)

		if message.Sender.ID != config.Owner {
			fmt.Println("📊", message.Sender.ID, "requested to view stats!")
		}
	})

	// Register photo handler
	bot.Handle(tb.OnPhoto, func(message *tb.Message) {
		// Anti-spam: return if user is not allowed to convert
		if !utils.ConversionPreHandler(&Spam, message.Sender.ID) {
			go log.Println("🚦 Chat", message.Sender.ID, "is ratelimited")
			bot.Send(message.Sender, utils.RatelimitedMessage(&Spam, message.Sender.ID), "Markdown")
			return
		}

		// Download
		imgBytes, err := getBytes(bot, message, "photo", &config)
		if err != nil {
			bot.Send(message.Sender, "⚠️ Error downloading image! Please try again.")
			return
		}

		// Resize
		photo, imgCaption, err, _ := resizeImage(imgBytes)

		// Send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			success := sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1

			if success {
				/*
				if message.Sender.ID != config.Owner {
					fmt.Printf("🖼 %d successfully converted an image!%s\n", message.Sender.ID, pngqC)
				}
				*/
			}
		}

		// Update stat for count of unique chats in a goroutine
		if message.Sender.ID != lastUser {
			go utils.UpdateUniqueStat(&message.Sender.ID, &config)
			lastUser = message.Sender.ID
		}
	})

	// Register document handler
	bot.Handle(tb.OnDocument, func(message *tb.Message) {
		// Anti-spam: return if user is not allowed to convert
		if !utils.ConversionPreHandler(&Spam, message.Sender.ID) {
			go log.Println("🚦 Chat", message.Sender.ID, "is ratelimited")
			bot.Send(message.Sender, utils.RatelimitedMessage(&Spam, message.Sender.ID), "Markdown")
			return
		}

		// Download
		imgBytes, err := getBytes(bot, message, "document", &config)
		if err != nil {
			bot.Send(message.Sender, "⚠️ Error downloading image! Please try again.")
			return
		}

		// Resize
		photo, imgCaption, err, _ := resizeImage(imgBytes)

		// Send
		if err != nil {
			bot.Send(message.Sender, imgCaption)
		} else {
			success := sendDocument(bot, message, photo, imgCaption)
			config.StatConverted += 1

			if success {
				/*
				if message.Sender.ID != config.Owner {
					fmt.Printf("🖼 %d successfully converted an image!%s\n", message.Sender.ID, pngqC)
				}
				*/
			}
		}

		// Update stat for count of unique chats in a goroutine
		if message.Sender.ID != lastUser {
			go utils.UpdateUniqueStat(&message.Sender.ID, &config)
			lastUser = message.Sender.ID
		}
	})

	// Register handler for incoming callback queries (i.e. stats refresh)
	bot.Handle(tb.OnCallback, func(cb *tb.Callback) {
		// Anti-spam
		if !utils.CommandPreHandler(&Spam, cb.Sender.ID, time.Now().Unix()) {
			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "⚠️ Please do not spam the bot for no reason.",
				ShowAlert:  true,
			}

			bot.Respond(cb, &resp)
			return
		}

		if cb.Data == "stats/refresh" {
			msg, sopts := utils.BuildStatsMsg(&config, vnum)
			bot.Edit(cb.Message, msg, &sopts)

			resp := tb.CallbackResponse{
				CallbackID: cb.ID,
				Text:       "🔄 Statistics refreshed!",
				ShowAlert:  false,
			}

			bot.Respond(cb, &resp)
		} else {
			go log.Println("⚠️ Invalid callback data received:", cb.Data)
		}
	})

	// Dump statistics to disk once every 30 minutes, clean spam struct every 60 minutes
	scheduler := gocron.NewScheduler(time.UTC)
	scheduler.Every(30).Minutes().Do(utils.DumpConfig, &config)
	scheduler.Every(60).Minutes().Do(utils.CleanConversionLogs, &Spam)
	scheduler.StartAsync()

	bot.Start()
}
