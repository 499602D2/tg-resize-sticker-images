package stats

import (
	"fmt"
	"sort"
	"time"

	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/spam"

	"github.com/dustin/go-humanize"
	tb "gopkg.in/tucnak/telebot.v3"
)

func StatsPlusOneConversion(conf *config.Config) {
	conf.Mutex.Lock()
	conf.StatConverted++
	conf.Mutex.Unlock()
}

func ChatExists(uid *int64, conf *config.Config) bool {
	// Checks if the chat ID already exists
	i := sort.Search(
		len(conf.UniqueUsers),
		func(i int) bool { return conf.UniqueUsers[i] >= *uid },
	)

	// UID already exists in the array, return true
	if i < len(conf.UniqueUsers) && conf.UniqueUsers[i] == *uid {
		return true
	}

	return false
}

func UpdateUniqueStat(uid *int64, conf *config.Config) {
	// Lock
	conf.Mutex.Lock()

	// User array is always sorted when performing check
	i := sort.Search(
		len(conf.UniqueUsers),
		func(i int) bool { return conf.UniqueUsers[i] >= *uid },
	)

	if i < len(conf.UniqueUsers) && conf.UniqueUsers[i] == *uid {
		// UID already exists in the array
		conf.Mutex.Unlock()
		return
	} else {
		if len(conf.UniqueUsers) == i {
			// Nil or empty slice, or after last element
			conf.UniqueUsers = append(conf.UniqueUsers, *uid)
		} else if i == 0 {
			// If zeroth index, append
			conf.UniqueUsers = append([]int64{*uid}, conf.UniqueUsers...)
		} else {
			// Otherwise, we're inserting in the middle of the array
			conf.UniqueUsers = append(conf.UniqueUsers[:i+1], conf.UniqueUsers[i:]...)
			conf.UniqueUsers[i] = *uid
		}
	}

	// Increment unique chat count
	conf.StatUniqueChats++

	// Unlock
	conf.Mutex.Unlock()
}

func BuildStatsMsg(conf *config.Config, aspam *spam.AntiSpam, vnum string) (string, tb.SendOptions) {
	// Main stats
	msg := fmt.Sprintf(
		"📊 *Overall statistics*\nImages converted: %s\nUnique users seen: %s",
		humanize.Comma(int64(conf.StatConverted)),
		humanize.Comma(int64(conf.StatUniqueChats)),
	)

	// Add spam statistics
	msg += "\n\n" + spam.SpamInspectionString(aspam)

	// Server info
	msg += fmt.Sprintf("\n\n*🎛 Server information*\nBot started %s",
		humanize.RelTime(time.Unix(conf.StatStarted, 0), time.Now(), "ago", "ago"),
	)

	// Version number, link to Github
	gitUrl := "https://github.com/499602D2/tg-resize-sticker-images"
	msg += fmt.Sprintf("\nRunning version [%s](%s)", vnum, gitUrl)

	// Construct keyboard for refresh functionality
	kb := [][]tb.InlineButton{{tb.InlineButton{Text: "🔄 Refresh statistics", Data: "stats/refresh"}}}
	rplm := tb.ReplyMarkup{InlineKeyboard: kb}

	// Add Markdown parsing for a pretty link embed + keyboard
	sopts := tb.SendOptions{ParseMode: "Markdown", ReplyMarkup: &rplm}

	return msg, sopts
}
