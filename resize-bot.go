/*
golang rewrite of the telegram sticker resize bot python program.
*/

package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tg-resize-sticker-images/bots"
	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/queue"
	"tg-resize-sticker-images/spam"

	"github.com/go-co-op/gocron"
	"github.com/h2non/bimg"

	tb "gopkg.in/telebot.v3"
)

func setupSignalHandler(session *config.Session) {
	// Listens for incoming interrupt signals, dumps config if detected
	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-channel
		// Log shutdown
		log.Println("🚦 Received interrupt signal: dumping config...")

		// Dump config, close bot connection
		config.DumpConfig(session.Config)
		session.Bot.Close()

		// Shutdown bimg, exit
		bimg.Shutdown()
		os.Exit(0)
	}()
}

func main() {
	const vnum string = "1.8.0 (2022.02.02)"

	// Log to file
	wd, _ := os.Getwd()
	logPath := filepath.Join(wd, "logs")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		_ = os.Mkdir(logPath, os.ModePerm)
	}

	// Set-up logging
	logFilePath := filepath.Join(logPath, "bot-log.log")
	logf, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Println(err)
	}

	// Set output of logs to file
	defer logf.Close()
	log.SetOutput(logf)
	log.Printf("🤖 [%s] Bot started at %s", vnum, time.Now())

	// Load (or create) config
	conf := config.LoadConfig()

	// Setup anti-spam
	Spam := spam.AntiSpam{}
	Spam.ChatBannedUntilTimestamp = make(map[int64]int64)
	Spam.ChatConversionLog = make(map[int64]spam.ConversionLog)
	Spam.ChatBanned = make(map[int64]bool)
	Spam.Rules = make(map[string]int64)

	// Add rules
	Spam.Rules["ConversionsPerHour"] = conf.ConversionRate
	Spam.Rules["TimeBetweenCommands"] = 1

	// Create bot
	bot, err := tb.NewBot(tb.Settings{
		Token:  conf.Token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal("Error starting bot:", err)
		return
	}

	// https://pkg.go.dev/github.com/h2non/bimg@v1.1.6#VipsCacheSetMax
	// 16 seems to limit memory usage to under 300 MB
	bimg.VipsCacheSetMax(16)

	// Setup messageSender
	sendQueue := queue.SendQueue{MessagesPerSecond: 15.0}

	// Define session: used to throw around structs that are needed frequently
	session := config.Session{
		Bot:    bot,
		Config: conf,
		Spam:   &Spam,
		Queue:  &sendQueue,
		Vnum:   vnum,
	}

	// Setup signal handler
	setupSignalHandler(&session)

	// Run MessageSender in a goroutine
	go bots.MessageSender(&session)

	// Setup bot
	bots.SetupBot(&session)

	// Dump statistics to disk once every 30 minutes, clean spam struct every 60 minutes
	scheduler := gocron.NewScheduler(time.UTC)
	scheduler.Every(30).Minutes().Do(config.DumpConfig, conf)
	scheduler.Every(60).Minutes().Do(spam.CleanConversionLogs, &Spam)
	scheduler.StartAsync()

	session.Bot.Start()
}
