package daily

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

type HourlyStatistic struct {
	hour      int
	count     int
	timestamp int64
}

type ConversionStatistics struct {
	stats [24]HourlyStatistic
	mutex sync.Mutex
	users map[int64]int64 // Map of userID to the last interaction time in Unix time
}

func isOlderThan24Hours(now, timestamp int64) bool {
	return now-timestamp > int64(24*3600)
}

func NewConversionStatistics() *ConversionStatistics {
	cs := &ConversionStatistics{}
	now := time.Now().UTC().Unix()
	for i := 0; i < 24; i++ {
		cs.stats[i].hour = i
		cs.stats[i].timestamp = now
	}
	cs.users = make(map[int64]int64)
	return cs
}

func (cs *ConversionStatistics) AddConversionByUser(userId int64) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	now := time.Now().UTC().Unix()
	currentHour := time.Now().UTC().Hour()

	// Defer the cleanup
	defer cs.cleanupUsers(now)

	// Check if the statistic for the current hour is outdated
	if isOlderThan24Hours(now, cs.stats[currentHour].timestamp) {
		cs.stats[currentHour].count = 0
		cs.stats[currentHour].timestamp = now
	}

	cs.stats[currentHour].count++

	// Store the user interaction
	cs.users[userId] = now
}

func (cs *ConversionStatistics) AddConversionAt(userId int64, unixTime int64) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	roundedTime := time.Unix(unixTime, 0).UTC()
	currentHour := roundedTime.Hour()

	now := time.Now().UTC().Unix()

	// Defer the cleanup
	defer cs.cleanupUsers(now)

	// If the stored timestamp is outdated (i.e., more than 24 hours ago)
	if isOlderThan24Hours(now, cs.stats[currentHour].timestamp) {
		cs.stats[currentHour].count = 0
		cs.stats[currentHour].timestamp = unixTime
	}

	// Only increment the count if the conversion was within the past 24 hours
	if !isOlderThan24Hours(now, unixTime) {
		cs.stats[currentHour].count++
	}

	// Store the user interaction
	cs.users[userId] = unixTime
}

func (cs *ConversionStatistics) CountConversions() int {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	total := 0
	now := time.Now().UTC().Unix()

	for i := 0; i < 24; i++ {
		stat := &cs.stats[i]

		// Ignore statistics older than 24 hours
		if now-stat.timestamp > int64(24*3600) {
			continue
		}

		total += stat.count
	}

	return total
}

func (cs *ConversionStatistics) CountUniqueUsers() int {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	// Cleanup the users map before counting
	cs.cleanupUsers(time.Now().UTC().Unix())

	counter := 0
	for range cs.users {
		counter++
	}

	return counter
}

func (cs *ConversionStatistics) MeanAndMedianConversions() (int64, int64) {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	sum := 0
	counts := make([]int, 0, 24)
	now := time.Now().UTC().Unix()

	for i := 0; i < 24; i++ {
		stat := &cs.stats[i]

		// Ignore statistics older than 24 hours
		if isOlderThan24Hours(now, stat.timestamp) {
			continue
		}

		sum += stat.count
		counts = append(counts, stat.count)
	}

	// Calculate mean
	n := len(counts)
	if n == 0 {
		return 0, 0
	}

	mean := float64(sum) / float64(n)

	// Calculate median
	sort.Ints(counts)
	median := 0.0

	if n%2 == 0 {
		median = float64(counts[n/2-1]+counts[n/2]) / 2
	} else {
		median = float64(counts[n/2])
	}

	return int64(mean), int64(median)
}

func (cs *ConversionStatistics) cleanupUsers(now int64) {
	for userId, timestamp := range cs.users {
		// If user interaction is more than 24 hours old, remove it from the map
		if isOlderThan24Hours(now, timestamp) {
			delete(cs.users, userId)
		}
	}
}

func (cs *ConversionStatistics) Debug() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	fmt.Printf("Hourly Statistics:\n")
	for i := 0; i < 24; i++ {
		fmt.Printf("Hour: %d, Count: %d, Timestamp: %d\n", cs.stats[i].hour, cs.stats[i].count, cs.stats[i].timestamp)
	}

	fmt.Printf("\nUsers map:\n")
	for userId, timestamp := range cs.users {
		fmt.Printf("User ID: %d, Last Interaction Time: %d\n", userId, timestamp)
	}
}

func (cs *ConversionStatistics) StatisticsString() string {
	mean, median := cs.MeanAndMedianConversions()
	return "🖼 *Daily statistics*\n" +
		fmt.Sprintf("Active chats: %s\n", humanize.Comma(int64(cs.CountUniqueUsers()))) +
		fmt.Sprintf("Images converted: %s\n", humanize.Comma(int64(cs.CountConversions()))) +
		fmt.Sprintf("Hourly mean/median: %s/%s", humanize.Comma(int64(mean)), humanize.Comma(int64(median)))
}
