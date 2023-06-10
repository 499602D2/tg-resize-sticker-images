package daily

import (
	"testing"
	"time"
)

func TestAddConversionByUser(t *testing.T) {
	cs := NewConversionStatistics()

	cs.AddConversionByUser(1)
	cs.AddConversionByUser(1)
	cs.AddConversionByUser(2)

	if cs.CountConversions() != 3 {
		t.Errorf("Expected count to be 3, got %d", cs.CountConversions())
	}

	if cs.CountUniqueUsers() != 2 {
		t.Errorf("Expected unique user count to be 2, got %d", cs.CountUniqueUsers())
	}
}

func TestAddConversionAt(t *testing.T) {
	cs := NewConversionStatistics()

	// Add conversion 25 hours ago for user 1
	pastTime := time.Now().Add(-25 * time.Hour).Unix()
	cs.AddConversionAt(1, pastTime)

	if cs.CountConversions() != 0 {
		t.Errorf("Expected count to be 0, got %d", cs.CountConversions())
	}

	// Add conversion for the current hour for user 1
	cs.AddConversionAt(1, time.Now().Unix())

	if cs.CountConversions() != 1 {
		t.Errorf("Expected count to be 1, got %d", cs.CountConversions())
	}
}

func TestCountConversions(t *testing.T) {
	cs := NewConversionStatistics()

	// Add conversion for the current hour for user 1
	cs.AddConversionByUser(1)

	if cs.CountConversions() != 1 {
		t.Errorf("Expected count to be 1, got %d", cs.CountConversions())
	}

	// Add conversion 25 hours ago for user 1
	pastTime := time.Now().Add(-25 * time.Hour).Unix()
	cs.AddConversionAt(1, pastTime)

	if cs.CountConversions() != 1 {
		t.Errorf("Expected count to be 1, got %d", cs.CountConversions())
	}
}

func TestCountUniqueUsers(t *testing.T) {
	cs := NewConversionStatistics()

	// Add conversion for the current hour for users 1 and 2
	cs.AddConversionByUser(1)
	cs.AddConversionByUser(2)

	if cs.CountUniqueUsers() != 2 {
		t.Errorf("Expected unique user count to be 2, got %d", cs.CountUniqueUsers())
	}

	// Add conversion 25 hours ago for user 3
	pastTime := time.Now().Add(-25 * time.Hour).Unix()
	cs.AddConversionAt(3, pastTime)

	if cs.CountUniqueUsers() != 2 {
		t.Errorf("Expected unique user count to be 2, got %d", cs.CountUniqueUsers())
	}

	// Add conversion 25 hours ago for user 1
	cs.AddConversionAt(1, pastTime)

	// If adding a very old interaction, we replace the fresh timestamp with an
	// old one, meaning that the user gets immediately cleaned up
	if cs.CountUniqueUsers() != 1 {
		t.Errorf("Expected unique user count to be 2, got %d", cs.CountUniqueUsers())
		cs.Debug()
	}
}

func TestStatisticsString(t *testing.T) {
	cs := NewConversionStatistics()

	// Add conversion for the current hour for users 1 and 2
	cs.AddConversionByUser(1)
	cs.AddConversionByUser(2)

	// Add thousands of conversions for the current hour
	for i := 0; i < 10000; i++ {
		cs.AddConversionByUser(3)
	}

	// Add random number of conversions for each hour
	for i := 0; i < 100; i++ {
		cs.AddConversionAt(4, time.Now().Add(-time.Duration(i)*time.Hour).Unix())
	}

	t.Log("Conversions added")

	// Output:
	t.Logf("%s", cs.StatisticsString())
}
