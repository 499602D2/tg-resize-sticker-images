package templates

import (
	"fmt"
	"time"

	"tg-resize-sticker-images/spam"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	tb "gopkg.in/telebot.v3"
)

// Response to the /help command
func HelpMessage(message *tb.Message, spam *spam.AntiSpam) string {
	return fmt.Sprintf(
		"🖼 Hi there! To use the bot, simply send your image to this chat. "+
			"Supported file-formats are `jpg`, `png`, and `webp`.\n\n"+
			"🖌️ The bot can also copy stickers from other packs. Just send any non-animated sticker, and it will be extracted!\n\n"+
			"*Note:* you can convert up to %d images per hour. You have done %s during the last hour. ",

		spam.Rules["ConversionsPerHour"],
		english.Plural(spam.ChatConversionLog[message.Sender.ID].ConversionCount, "conversion", ""),
	)
}

// Construct the message for rate-limited chats.
func RatelimitedMessage(spam *spam.AntiSpam, chat int64) string {
	return fmt.Sprintf(
		"🚦 *Slow down!* You're allowed to convert %d images per hour. %s %s.",
		spam.Rules["ConversionsPerHour"], "You can convert images again in",
		humanize.Time(time.Unix(int64(spam.ChatBannedUntilTimestamp[chat]), 0)))
}
