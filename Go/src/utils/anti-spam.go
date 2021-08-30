package utils

import (
	"fmt"
	"log"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
)

type AntiSpam struct {
	// Global struct keeping track of banned chats and per-chat activity
	ChatBanned                map[int]bool          // Simple "if ChatBanned[chat] { do }" checks
	ChatBannedUntilTimestamp  map[int]int           // How long banned chats are banned for
	ChatConversionLog         map[int]ConversionLog // map chat ID to a ConversionLog struct
	Rules                     map[string]int64      // Arbitrary rules for code flexibility
}

type ConversionLog struct {
	// Per-chat in-struct keeping track of per-chat activity
	ConversionCount             int   // Image conversion count
	ConversionTimestamps 	    []int // Trailing timestamps of converted images
	NextAllowedCommandTimestamp int64 // Next time the chat is allowed to convert an image
	CommandSpamOffenses         int   // Count of spam offences (not used yet)
}

func CleanConversionLogs(spam *AntiSpam) {
	/* Used to periodically clean the conversion log, beacause
	many users may never reach the 100-image hourly conversion limit. */

	// Keep track of statistics
	cleanedArrays, cleanedEntries := 0, 0

	trailingHour := int(time.Now().Unix() - 3600)
	for chat, conversionLog := range spam.ChatConversionLog {
		if conversionLog.ConversionCount != 0 {
			if conversionLog.ConversionTimestamps[conversionLog.ConversionCount - 1] <= trailingHour {
				// Keep track of stats
				cleanedArrays++
				cleanedEntries += conversionLog.ConversionCount

				// The latest timestamp is older than 3600 seconds: clean the entire array
				conversionLog.ConversionCount = 0
				conversionLog.ConversionTimestamps = []int{}

				// Re-assign conversionLog to spam struct
				spam.ChatConversionLog[chat] = conversionLog
			}
		}
	}

	log.Printf("✨ ConversionLogs cleaned! Cleaned %s array(s) and removed %s entries!\n",
		humanize.Comma(int64(cleanedArrays)),
		humanize.Comma(int64(cleanedEntries)),
	)
}

func ConversionPreHandler(spam *AntiSpam, chat int) bool {
	// Check if user is banned
	if spam.ChatBanned[chat] {
		if spam.ChatBannedUntilTimestamp[chat] <= int(time.Now().Unix()) {
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = 0
		} else {
			fmt.Println("🔨 Chat", chat, "is currently banned until",
				spam.ChatConversionLog[chat].ConversionTimestamps[0])
			return false
		}
	}

	// Remove every timetsamp older than an hour
	timeNow := int(time.Now().Unix())
	ccLog := spam.ChatConversionLog[chat]

	// Iterate timestamps. TODO: consider binary search
	i, arrLen, skipTruncate := 0, len(ccLog.ConversionTimestamps), false
	if arrLen != 0 {
		for _, timestamp := range ccLog.ConversionTimestamps {
			if timeNow < timestamp + 3600 {
				if i == 0 && arrLen == 1 {
					// Avoid a range error when dealing with single-element arrays
					skipTruncate = true
				}
				break;
			}

			if i != arrLen - 1 {
				i++
			}
		}

		// Update ConversionTimestamp ranges now that old timestamps are removed
		if i == arrLen - 1 && arrLen != 1 {
			// If we reached last index, clean array
			ccLog.ConversionTimestamps = []int{}
		} else if !skipTruncate {
			// Truncate array, unless we're at first index of single-element array
			ccLog.ConversionTimestamps = ccLog.ConversionTimestamps[i:arrLen]
		}

		// Update conversionCount and timestamps
		ccLog.ConversionCount = len(ccLog.ConversionTimestamps)
	}
	
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
		return false
	}

	// No rules broken: update spam log
	ccLog.ConversionCount++
	ccLog.ConversionTimestamps = append(ccLog.ConversionTimestamps, timeNow)
	spam.ChatConversionLog[chat] = ccLog

	return true
}

func CommandPreHandler(spam *AntiSpam, chat int, sentAt int64) bool {
	// Verify chat is eligible for command parse
	chatLog := spam.ChatConversionLog[chat]
	if chatLog.NextAllowedCommandTimestamp > sentAt {
		chatLog.CommandSpamOffenses++

		go log.Printf("🚦 %d has %s",
			chat, english.Plural(chatLog.CommandSpamOffenses, "spam offence", ""))
		chatLog.NextAllowedCommandTimestamp = time.Now().Unix() + spam.Rules["TimeBetweenCommands"]

		spam.ChatConversionLog[chat] = chatLog
		return false
	}

	// No spam, update chat's ConversionLog
	chatLog.NextAllowedCommandTimestamp = time.Now().Unix() + spam.Rules["TimeBetweenCommands"]
	spam.ChatConversionLog[chat] = chatLog
	return true
}

func RatelimitedMessage(spam *AntiSpam, chat int) string {
	return fmt.Sprintf(
		"🚦 *Slow down!* You're allowed to convert %d images per hour. %s %s.",
		spam.Rules["ConversionsPerHour"], "You can convert images again in",
		humanize.Time(time.Unix(int64(spam.ChatBannedUntilTimestamp[chat]), 0)))
}
