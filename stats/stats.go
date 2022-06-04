package stats

import (
	"fmt"
	"sort"
	"time"

	"tg-resize-sticker-images/config"
	"tg-resize-sticker-images/spam"

	"github.com/dustin/go-humanize"
	"github.com/hako/durafmt"
	tb "gopkg.in/telebot.v3"
)

const gitUrl = "https://github.com/499602D2/tg-resize-sticker-images"

// Update what the last observed user ID was
func UpdateLastUserId(session *config.Session, id int64) {
	session.LastUser = id
}

// Add one conversion to the stats
func StatsPlusOneConversion(conf *config.Config) {
	conf.Mutex.Lock()
	conf.StatConverted++
	conf.Mutex.Unlock()
}

// Checks if the chat ID has been seen before
func ChatExists(uid int64, conf *config.Config) bool {
	conf.Mutex.Lock()
	defer conf.Mutex.Unlock()

	i := sort.Search(
		len(conf.UniqueUsers),
		func(i int) bool { return conf.UniqueUsers[i] >= uid },
	)

	// UID already exists in the array, return true
	if i < len(conf.UniqueUsers) && conf.UniqueUsers[i] == uid {
		return true
	}

	return false
}

func UpdateUniqueStat(uid int64, conf *config.Config) {
	// Lock
	conf.Mutex.Lock()
	defer conf.Mutex.Unlock()

	// User array is always sorted when performing check
	i := sort.Search(
		len(conf.UniqueUsers),
		func(i int) bool { return conf.UniqueUsers[i] >= uid },
	)

	if i < len(conf.UniqueUsers) && conf.UniqueUsers[i] == uid {
		// UID already exists in the array
		return
	} else {
		if len(conf.UniqueUsers) == i {
			// Nil or empty slice, or after last element
			conf.UniqueUsers = append(conf.UniqueUsers, uid)
		} else if i == 0 {
			// If zeroth index, append
			conf.UniqueUsers = append([]int64{uid}, conf.UniqueUsers...)
		} else {
			// Otherwise, we're inserting in the middle of the array
			conf.UniqueUsers = append(conf.UniqueUsers[:i+1], conf.UniqueUsers[i:]...)
			conf.UniqueUsers[i] = uid
		}
	}

	// Increment unique chat count
	conf.StatUniqueChats++
}

func BuildStatsMsg(conf *config.Config, aspam *spam.AntiSpam, vnum string) (string, tb.SendOptions) {
	// Main stats
	msg := fmt.Sprintf(
		"📊 *Overall statistics*\n"+
			"Images converted: %s\n"+
			"Unique users seen: %s\n\n"+

			"%s\n\n"+

			"*🎛 Server information*\n"+
			"Bot started %s ago\n"+
			"Running version [%s](%s)",

		// Overall stats
		humanize.Comma(int64(conf.StatConverted)),
		humanize.Comma(int64(conf.StatUniqueChats)),

		// Trailing-hour statistics
		spam.SpamInspectionString(aspam),

		// Server info
		durafmt.Parse(time.Since(time.Unix(conf.StatStarted, 0))).LimitFirstN(2),
		vnum, gitUrl,
	)

	// Construct keyboard for refresh functionality
	kb := [][]tb.InlineButton{{tb.InlineButton{Text: "🔄 Refresh statistics", Data: "stats/refresh"}}}

	// Add Markdown parsing for a pretty link embed + keyboard
	sopts := tb.SendOptions{ParseMode: "Markdown", ReplyMarkup: &tb.ReplyMarkup{InlineKeyboard: kb}}

	return msg, sopts
}
