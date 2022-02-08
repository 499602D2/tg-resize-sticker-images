package spam

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/dustin/go-humanize/english"
)

type AntiSpam struct {
	/* In-memory struct keeping track of banned chats and per-chat activity */
	ChatBanned               map[int64]bool          // Simple "if ChatBanned[chat] { do }" checks
	ChatBannedUntilTimestamp map[int64]int64         // How long banned chats are banned for
	ChatConversionLog        map[int64]ConversionLog // Map chat ID to a ConversionLog struct
	Rules                    map[string]int64        // Arbitrary rules for code flexibility
	Mutex                    sync.Mutex              // Mutex to avoid concurrent map writes
}

type ConversionLog struct {
	/* Per-chat struct keeping track of activity for spam management */
	ConversionCount             int     // Image conversion count
	ConversionTimestamps        []int64 // Trailing timestamps of converted images
	NextAllowedCommandTimestamp int64   // Next time the chat is allowed to convert an image
	CommandSpamOffenses         int     // Count of spam offences (not used yet)
}

func SpamInspectionString(spam *AntiSpam) string {
	/* A simple function that prints some insights of the AntiSpam struct */
	inspectionString := ""

	// Amount of chats in AntiSpam
	chatCount := len(spam.ChatConversionLog)

	// Track some insights
	totalConversions := 0     // Total conversion across all chats
	maxConversionCount := 0   // Most per-chat conversions in the last 60 minutes
	onceBannedChats := 0      // How many chats have been banned at some point
	currentlyBannedChats := 0 // How many chats are currently banned

	// Iterate over all chats that are tracked
	for chat := range spam.ChatConversionLog {
		totalConversions += spam.ChatConversionLog[chat].ConversionCount
		if spam.ChatConversionLog[chat].ConversionCount > maxConversionCount {
			maxConversionCount = spam.ChatConversionLog[chat].ConversionCount
		}

		// Check if chat is, or has been, banned
		banStatus := spam.ChatBannedUntilTimestamp[chat]
		if banStatus != 0 {
			onceBannedChats++
			if banStatus == 1 {
				currentlyBannedChats++
			}
		}
	}

	// Construct the spam message
	inspectionString += "🖼 *Hourly statistics*\n"
	inspectionString += fmt.Sprintf("Active chats: %d\n", chatCount)
	inspectionString += fmt.Sprintf("Images converted: %d\n", totalConversions)
	inspectionString += fmt.Sprintf("Max conversions by a chat: %d", maxConversionCount)

	return inspectionString
}

func CleanConversionLogs(spam *AntiSpam) {
	/* Used to periodically clean the conversion log, beacause
	many users may never reach the x-image hourly conversion limit. */

	// Lock struct to avoid concurrent writes
	spam.Mutex.Lock()

	// Iterate all chats, run RefreshConversions
	for chat := range spam.ChatConversionLog {
		RefreshConversions(spam, chat)
	}

	// Unlock struct
	spam.Mutex.Unlock()
}

func RefreshConversions(spam *AntiSpam, chat int64) {
	/* Count the amount of conversions in the last hour.
	Used by /help and /spam, plus periodically ran automatically. */
	if spam.ChatConversionLog[chat].ConversionCount == 0 {
		// If more than 3600 seconds since last command, remove entry
		if spam.ChatConversionLog[chat].NextAllowedCommandTimestamp <= time.Now().Unix()-3600 {
			delete(spam.ChatConversionLog, chat)
		}
		return
	}

	// Extract chat's conversion log
	ccLog := spam.ChatConversionLog[chat]

	// Search for last index outside of the trailing 3600 seconds
	trailingHour := time.Now().Unix() - 3600
	lastOOR := sort.Search(
		len(ccLog.ConversionTimestamps),
		func(i int) bool { return ccLog.ConversionTimestamps[i] > trailingHour },
	)

	if lastOOR == len(ccLog.ConversionTimestamps) {
		// If we go over the last index, clear the array
		ccLog.ConversionTimestamps = []int64{}
	} else if lastOOR == 0 {
		// Nothing to do if all timestamps are within the last trailing hour
	} else {
		// Otherwise, we're somewhere inside the array: truncate
		ccLog.ConversionTimestamps = ccLog.ConversionTimestamps[lastOOR:len(ccLog.ConversionTimestamps)]
	}

	// Check if user is banned
	if spam.ChatBanned[chat] {
		if spam.ChatBannedUntilTimestamp[chat] <= time.Now().Unix() {
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = -1

			log.Println("⌛️ Chat", chat, "unbanned in RefreshConversions!")
		}
	}

	// If the chat has 0 conversions after refresh, and no fresh command calls, delete it
	if len(ccLog.ConversionTimestamps) == 0 {
		if ccLog.NextAllowedCommandTimestamp <= time.Now().Unix()-3600 {
			delete(spam.ChatConversionLog, chat)
			return
		}
	}

	// Update ConversionCount, push ConversionLog to spam struct
	ccLog.ConversionCount = len(ccLog.ConversionTimestamps)
	spam.ChatConversionLog[chat] = ccLog
}

func ConversionPreHandler(spam *AntiSpam, chat int64) bool {
	/* When a conversion is requested, ConversionPreHandler verifies the
	chat is not banned and has not exceeded the hourly conversion limit. */

	// Lock spam struct to avoid concurrent writes
	spam.Mutex.Lock()

	// Check if user is banned
	if spam.ChatBanned[chat] {
		if spam.ChatBannedUntilTimestamp[chat] <= time.Now().Unix() {
			// Chat's ban period has ended, lift ban
			log.Println("⌛️ Chat", chat, "unbanned!")
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = -1
		} else {
			// Chat is still banned
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
			spam.ChatBannedUntilTimestamp[chat] = time.Now().Unix() + 3600
		}
	} else {
		// Otherwise, update ban status
		if spam.ChatBanned[chat] {
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = 0

			log.Printf("🚦 %d unratelimited! %d conversions remaining in log.",
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
	ccLog.ConversionTimestamps = append(ccLog.ConversionTimestamps, time.Now().Unix())
	spam.ChatConversionLog[chat] = ccLog

	spam.Mutex.Unlock()
	return true
}

func CommandPreHandler(spam *AntiSpam, chat int64, sentAt int64) bool {
	/* When user sends a command, verify the chat is eligible for a command parse. */
	chatLog := spam.ChatConversionLog[chat]
	spam.Mutex.Lock()

	if chatLog.NextAllowedCommandTimestamp > sentAt {
		chatLog.CommandSpamOffenses++

		log.Printf("🚦 %d has %s",
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
