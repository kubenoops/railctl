package apply

import (
	"os"
	"testing"
)

// TestMain zeroes the live-propagation waits for the whole package: they exist
// to give Railway's async commits time to land (config before a rollout, a
// volume's instance before its schedules are set), which is meaningless against
// mocks and would otherwise add SettleDelay to every create test and a minute
// of polling to any test whose mock lists no matching volume instance.
func TestMain(m *testing.M) {
	SettleDelay = 0
	configCommitTimeout = 0
	configCommitPoll = 0
	QuiesceTimeout = 0
	QuiescePoll = 0
	volumeInstanceAttempts = 1
	volumeInstancePoll = 0
	os.Exit(m.Run())
}
