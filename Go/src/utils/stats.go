package utils

import (
	"fmt"
	"sort"
	"time"

	"github.com/dustin/go-humanize"
	tb "gopkg.in/tucnak/telebot.v2"
)

func StatsPlusOneConversion(config *Config) {
	config.Mutex.Lock()
	config.StatConverted++
	config.Mutex.Unlock()
}

func ChatExists(uid *int, config *Config) bool {
	// Checks if the chat ID already exists
	i := sort.Search(
		len(config.UniqueUsers),
		func(i int) bool { return config.UniqueUsers[i] >= *uid },
	)

	// UID already exists in the array, return true
	if i < len(config.UniqueUsers) && config.UniqueUsers[i] == *uid {
		return true
	}

	return false
}

func UpdateUniqueStat(uid *int, config *Config) {
	// Lock
	config.Mutex.Lock()

	// User array is always sorted when performing check
	i := sort.Search(
		len(config.UniqueUsers),
		func(i int) bool { return config.UniqueUsers[i] >= *uid },
	)

	if i < len(config.UniqueUsers) && config.UniqueUsers[i] == *uid {
		// UID already exists in the array
		config.Mutex.Unlock()
		return
	} else {
		if len(config.UniqueUsers) == i {
			// Nil or empty slice, or after last element
			config.UniqueUsers = append(config.UniqueUsers, *uid)
		} else if i == 0 {
			// If zeroth index, append
			config.UniqueUsers = append([]int{*uid}, config.UniqueUsers...)
		} else {
			// Otherwise, we're inserting in the middle of the array
			config.UniqueUsers = append(config.UniqueUsers[:i+1], config.UniqueUsers[i:]...)
			config.UniqueUsers[i] = *uid
		}
	}

	// Increment unique chat count
	config.StatUniqueChats++

	// Unlock
	config.Mutex.Unlock()
}

func BuildStatsMsg(config *Config, vnum string) (string, tb.SendOptions) {
	// Main stats
	msg := fmt.Sprintf(
		"📊 *Bot statistics*\nImages converted: %s\nUnique users seen: %s",
		humanize.Comma(int64(config.StatConverted)),
		humanize.Comma(int64(config.StatUniqueChats)),
	)

	// Server info
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
