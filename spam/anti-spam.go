package spam

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// In-memory struct keeping track of banned chats and per-chat activity
type AntiSpam struct {
	ChatBanned               map[int64]bool           // Simple "if ChatBanned[chat] { do }" checks
	ChatBannedUntilTimestamp map[int64]int64          // How long banned chats are banned for
	ChatConversionLog        map[int64]*ConversionLog // Map chat ID to a ConversionLog struct
	Rules                    map[string]int64         // Arbitrary rules for code flexibility
	Mutex                    sync.Mutex               // Mutex to avoid concurrent map writes
}

// Per-chat struct keeping track of activity for spam management
type ConversionLog struct {
	ConversionCount        int     // Image conversion count
	ConversionTimestamps   []int64 // Trailing timestamps of converted images
	LastCommandSendTime    time.Time
	UserLimiter            rate.Limiter
	RateLimitMessageSent   bool // Has the user been notified that they're rate-limited?
	RateLimitMessageSentAt time.Time
}

// Enforce a token-based rate-limiter on a per-chat basis
func (spam *AntiSpam) RunUserLimiter(id int64, tokens int) {
	if spam.ChatConversionLog[id] == nil {
		spam.ChatConversionLog[id] = &ConversionLog{
			UserLimiter: *rate.NewLimiter(1, 1),
		}
	}

	// Run limiter
	err := spam.ChatConversionLog[id].UserLimiter.WaitN(
		context.Background(), tokens,
	)

	if err != nil {
		log.Error().Err(err).Msg("Running user-limiter failed")
	}

	spam.ChatConversionLog[id].LastCommandSendTime = time.Now()
}

// A simple function that prints some insights of the AntiSpam struct
func SpamInspectionString(spam *AntiSpam) string {
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

/* Used to periodically clean the conversion log, beacause
many users may never reach the x-image hourly conversion limit. */
func CleanConversionLogs(spam *AntiSpam) {
	// Lock struct to avoid concurrent writes
	spam.Mutex.Lock()

	// Iterate all chats, run RefreshConversions
	for chat := range spam.ChatConversionLog {
		RefreshConversions(spam, chat)
	}

	// Unlock struct
	spam.Mutex.Unlock()
}

/* Count the amount of conversions in the last hour.
Used by /help and /spam, plus periodically ran automatically. */
func RefreshConversions(spam *AntiSpam, chat int64) {
	// Extract chat's conversion log for cleaner code
	ccLog := spam.ChatConversionLog[chat]

	// Mutex has already been locked, so modifying is safe
	if ccLog.ConversionCount == 0 {
		// If more than 3600 seconds since last command, remove entry
		if time.Since(ccLog.LastCommandSendTime) > time.Hour {
			delete(spam.ChatConversionLog, chat)
		}

		return
	}

	// Search for last index outside of the trailing 3600 seconds
	trailingHour := time.Now().Unix() - 3600

	// Last time stamp that is out of range (OOR)
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
			// If user should be unbanned, do it now
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = -1
			ccLog.RateLimitMessageSent = false
			ccLog.RateLimitMessageSentAt = time.Time{}

			log.Info().Msgf("⌛️ Chat %d unbanned in RefreshConversions", chat)
		}
	}

	// If the chat has 0 conversions after refresh, and no fresh command calls, delete it
	if len(ccLog.ConversionTimestamps) == 0 {
		if time.Since(ccLog.LastCommandSendTime) > time.Hour {
			delete(spam.ChatConversionLog, chat)
			return
		}
	}

	// Update ConversionCount, push ConversionLog to spam struct
	ccLog.ConversionCount = len(ccLog.ConversionTimestamps)
}

/* When a conversion is requested, ConversionPreHandler verifies the
chat is not banned and has not exceeded the hourly conversion limit. */
func ConversionPreHandler(spam *AntiSpam, chat int64) bool {
	// Lock spam struct to avoid concurrent writes
	spam.Mutex.Lock()
	defer spam.Mutex.Unlock()

	// Check if user is banned
	if spam.ChatBanned[chat] {
		if spam.ChatBannedUntilTimestamp[chat] <= time.Now().Unix() {
			// Chat's ban period has ended, lift ban
			log.Info().Msgf("⌛️ Chat %d unbanned", chat)
			spam.ChatBanned[chat] = false
			spam.ChatBannedUntilTimestamp[chat] = -1
		} else {
			// Chat is still banned
			return false
		}
	}

	// Check that user's spam log exists
	if spam.ChatConversionLog[chat] == nil {
		spam.ChatConversionLog[chat] = &ConversionLog{
			UserLimiter: *rate.NewLimiter(1, 2),
		}
	}

	// Pointer to chat's spam log
	ccLog := spam.ChatConversionLog[chat]

	// Remove every timestamp older than an hour, if chat seems like they might be bannable
	if ccLog.ConversionCount >= int(spam.Rules["ConversionsPerHour"]) {
		RefreshConversions(spam, chat)
	}

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
			ccLog.RateLimitMessageSent = false
			ccLog.RateLimitMessageSentAt = time.Time{}

			log.Info().Msgf("🚦 %d unratelimited! %d conversions remaining in log",
				chat, len(ccLog.ConversionTimestamps))
		}
	}

	if spam.ChatBanned[chat] {
		// Return if chat was banned
		return false
	}

	// No rules broken: update spam log
	ccLog.ConversionCount++
	ccLog.ConversionTimestamps = append(ccLog.ConversionTimestamps, time.Now().Unix())

	return true
}

func (spam *AntiSpam) ChatReceivedRateLimitMessage(chat int64) {
	// Lock spam struct to avoid concurrent writes
	spam.Mutex.Lock()
	defer spam.Mutex.Unlock()

	// Update flag
	ccLog := spam.ChatConversionLog[chat]
	ccLog.RateLimitMessageSent = true
	ccLog.RateLimitMessageSentAt = time.Now()
}
