package utils

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	tb "gopkg.in/tucnak/telebot.v2"
)

func HelpMessage(message *tb.Message, spam *AntiSpam) string {
	helpMessage := "🖼 Hi there! To use the bot, simply send your image to this chat as JPG/PNG/WebP."
	helpMessage += " The bot can also copy stickers — try sending one!"

	helpMessage += fmt.Sprintf(
		"\n\n*Note:* you can convert up to %d images per hour.",
		spam.Rules["ConversionsPerHour"])

	helpMessage += fmt.Sprintf(
		" You have done %s during the last hour.",
		english.Plural(spam.ChatConversionLog[message.Sender.ID].ConversionCount, "conversion", ""))

	return helpMessage
}

func RatelimitedMessage(spam *AntiSpam, chat int) string {
	/* Construct the message for rate-limited chats. */
	return fmt.Sprintf(
		"🚦 *Slow down!* You're allowed to convert %d images per hour. %s %s.",
		spam.Rules["ConversionsPerHour"], "You can convert images again in",
		humanize.Time(time.Unix(int64(spam.ChatBannedUntilTimestamp[chat]), 0)))
}
