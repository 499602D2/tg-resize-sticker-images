/*
golang rewrite of the telegram sticker resize bot python program.
*/

package main

import (
	"flag"
	"fmt"
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
	"golang.org/x/time/rate"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	tb "gopkg.in/telebot.v3"
)

// Variables injected at build-time
var GitSHA = "0000000"

func setupSignalHandler(session *config.Session) {
	// Listens for incoming interrupt signals, dumps config if detected
	channel := make(chan os.Signal)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-channel
		// Log shutdown
		log.Info().Msg("🚦 Received interrupt signal: dumping config...")

		// Dump config, close bot connection
		config.DumpConfig(session.Config)
		session.Bot.Close()

		// Shutdown bimg, exit
		bimg.Shutdown()
		os.Exit(0)
	}()
}

func main() {
	// Get commit the bot is running
	vnum := fmt.Sprintf("1.10.0 (%s)", GitSHA[0:7])

	// Log to file
	wd, _ := os.Getwd()
	logPath := filepath.Join(wd, "logs")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		_ = os.Mkdir(logPath, os.ModePerm)
	}

	var debug bool
	flag.BoolVar(&debug, "debug", false, "Specify to show logs in the console")
	flag.Parse()

	if !debug {
		// If not debugging, log to file
		logFilePath := filepath.Join(logPath, "bot-log.log")
		logf, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		defer logf.Close()

		if err != nil {
			log.Fatal().Err(err).Msg("Setting up log-file failed")
		}

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: logf, NoColor: true, TimeFormat: time.RFC822Z})
	} else {
		// If debugging, output to console
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC822Z})
	}

	log.Info().Msgf("🤖 [%s] Bot started with vips version %s", vnum, bimg.VipsVersion)

	// Load (or create) config
	conf := config.LoadConfig()

	// Setup anti-spam
	Spam := spam.AntiSpam{}
	Spam.ChatBannedUntilTimestamp = make(map[int64]int64)
	Spam.ChatConversionLog = make(map[int64]*spam.ConversionLog)
	Spam.ChatBanned = make(map[int64]bool)
	Spam.Rules = make(map[string]int64)

	// Add rules
	Spam.Rules["ConversionsPerHour"] = conf.ConversionRate

	// Create bot
	bot, err := tb.NewBot(tb.Settings{
		Token:  conf.Token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal().Err(err).Msg("Error starting bot")
		return
	}

	/* https://pkg.go.dev/github.com/h2non/bimg@v1.1.6#VipsCacheSetMax.
	16 seems to limit memory usage to under 300 MB

	default value is 500
	https://github.com/h2non/bimg/blob/a8f6d5fa08deb38350e173bf5c4445ee9bc2baaf/vips.go#L31 */
	bimg.VipsCacheSetMax(64)

	// Setup messageSender
	sendQueue := queue.SendQueue{}
	sendQueue.Limiter = rate.NewLimiter(20, 2)

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

	// Create scheduler
	scheduler := gocron.NewScheduler(time.UTC)

	// Save config (and stats) every half-hour
	_, err = scheduler.Every(30).Minutes().Do(config.DumpConfig, conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Starting config saver job failed")
	}

	// Clean conversion logs every hour
	_, err = scheduler.Every(60).Minutes().Do(spam.CleanConversionLogs, &Spam)
	if err != nil {
		log.Fatal().Err(err).Msg("Starting conversion log cleaner job failed")
	}

	scheduler.StartAsync()

	session.Bot.Start()
}
