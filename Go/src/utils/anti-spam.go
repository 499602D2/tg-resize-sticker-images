package utils

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
)

type AntiSpam struct {
	/* In-memory struct keeping track of banned chats and per-chat activity */
	ChatBanned                map[int]bool          // Simple "if ChatBanned[chat] { do }" checks
	ChatBannedUntilTimestamp  map[int]int           // How long banned chats are banned for
	ChatConversionLog         map[int]ConversionLog // map chat ID to a ConversionLog struct
	Rules                     map[string]int64      // Arbitrary rules for code flexibility
	Mutex                     sync.Mutex            // Mutex to avoid concurrent map writes
}

type ConversionLog struct {
	/* Per-chat struct keeping track of activity for spam management */
	ConversionCount             int   // Image conversion count
	ConversionTimestamps 	    []int // Trailing timestamps of converted images
	NextAllowedCommandTimestamp int64 // Next time the chat is allowed to convert an image
	CommandSpamOffenses         int   // Count of spam offences (not used yet)
}

func CleanConversionLogs(spam *AntiSpam) {
	/* Used to periodically clean the conversion log, beacause
	many users may never reach the x-image hourly conversion limit. */

	// Lock struct to avoid concurrent writes
	spam.Mutex.Lock()

	// Iterate all chats
	for chat := range spam.ChatConversionLog {
		RefreshConversions(spam, chat)
	}

	// Unlock struct
	spam.Mutex.Unlock()
}

func RefreshConversions(spam *AntiSpam, chat int) {
	/* Count the amount of conversions in the last hour. Used by /help. */
	if spam.ChatConversionLog[chat].ConversionCount == 0 {
		return
	}

	ccLog := spam.ChatConversionLog[chat]

	// Count every timestamp in the last 3600 seconds
	trailingHour := int(time.Now().Unix() - 3600)

	// Search for last index outside of the trailing 3600 seconds
	lastOOR := sort.Search(
		len(ccLog.ConversionTimestamps),
		func(i int) bool { return ccLog.ConversionTimestamps[i] > trailingHour },
	)

	if lastOOR == len(ccLog.ConversionTimestamps) {
		// If we go over the last index, clear the array
		ccLog.ConversionTimestamps = []int{}
	} else if lastOOR == 0 {
		// Nothing to do if all timestamps are within the last trailing hour
	} else {
		// Otherwise, we're somewhere inside the array: truncate
		ccLog.ConversionTimestamps = ccLog.ConversionTimestamps[lastOOR:len(ccLog.ConversionTimestamps)]
	}

	// Update ConversionCount, push ConversionLog to spam struct
	ccLog.ConversionCount = len(ccLog.ConversionTimestamps)
	spam.ChatConversionLog[chat] = ccLog
}

func ConversionPreHandler(spam *AntiSpam, chat int) bool {
	/* When a conversion is requested, ConversionPreHandler verifies the
	chat is not banned and has not exceeded the hourly conversion limit. */

	// Lock spam struct
	spam.Mutex.Lock()

	// Check if user is banned
	if spam.ChatBanned[chat] {
		if spam.ChatBannedUntilTimestamp[chat] <= int(time.Now().Unix()) {
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = 0
		} else {
			fmt.Println("🔨 Chat", chat, "is currently banned until",
				spam.ChatBannedUntilTimestamp[chat],
			)

			spam.Mutex.Unlock()
			return false
		}
	}

	// Remove every timestamp older than an hour
	RefreshConversions(spam, chat)
	ccLog := spam.ChatConversionLog[chat]

	// If chat has too many conversion within trailing hour, update ChatBannedUntilTimestamp
	if ccLog.ConversionCount >= int(spam.Rules["ConversionsPerHour"]) {
		spam.ChatBanned[chat] = true

		if len(ccLog.ConversionTimestamps) != 0 {
			spam.ChatBannedUntilTimestamp[chat] = ccLog.ConversionTimestamps[0] + 3600
		} else {
			spam.ChatBannedUntilTimestamp[chat] = int(time.Now().Unix()) + 3600
		}
	} else {
		// Otherwise, update ban status
		if spam.ChatBanned[chat] {
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = 0

			go log.Printf("🚦 %d unratelimited! %d conversions remaining in log.",
				chat, len(ccLog.ConversionTimestamps))
		}
	}
	
	// Push updated ConversionLog to spam struct
	spam.ChatConversionLog[chat] = ccLog

	// Return if chat was banned
	if spam.ChatBanned[chat] {
		spam.Mutex.Unlock()
		return false
	}

	// No rules broken: update spam log
	ccLog.ConversionCount++
	ccLog.ConversionTimestamps = append(ccLog.ConversionTimestamps, int(time.Now().Unix()))
	spam.ChatConversionLog[chat] = ccLog

	spam.Mutex.Unlock()
	return true
}

func CommandPreHandler(spam *AntiSpam, chat int, sentAt int64) bool {
	/* When user sends a command, verify the chat is eligible for a command parse. */

	chatLog := spam.ChatConversionLog[chat]
	spam.Mutex.Lock()

	if chatLog.NextAllowedCommandTimestamp > sentAt {
		chatLog.CommandSpamOffenses++

		go log.Printf("🚦 %d has %s",
			chat, english.Plural(chatLog.CommandSpamOffenses, "spam offence", ""))
		chatLog.NextAllowedCommandTimestamp = time.Now().Unix() + spam.Rules["TimeBetweenCommands"]

		spam.ChatConversionLog[chat] = chatLog
		spam.Mutex.Unlock()
		return false
	}

	// No spam, update chat's ConversionLog
	chatLog.NextAllowedCommandTimestamp = time.Now().Unix() + spam.Rules["TimeBetweenCommands"]
	spam.ChatConversionLog[chat] = chatLog
	spam.Mutex.Unlock()
	return true
}

func RatelimitedMessage(spam *AntiSpam, chat int) string {
	/* Construct the message for rate-limited chats. */
	return fmt.Sprintf(
		"🚦 *Slow down!* You're allowed to convert %d images per hour. %s %s.",
		spam.Rules["ConversionsPerHour"], "You can convert images again in",
		humanize.Time(time.Unix(int64(spam.ChatBannedUntilTimestamp[chat]), 0)))
}
