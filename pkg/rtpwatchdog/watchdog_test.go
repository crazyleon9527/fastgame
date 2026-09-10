package rtpwatchdog

import (
	"testing"

	"fastgame/pkg/money"
)

func TestWatchdogTriggersHighRtp(t *testing.T) {
	w := New(Config{GlobalMax: 100, PlayerMax: 50, ThresholdPPM: 1800000, MinSampleBet: money.Scale})

	var alerts []Alert
	for i := 0; i < 20; i++ {
		bet := int64(10 * money.Scale)
		win := int64(20 * money.Scale)
		alerts = append(alerts, w.Record(RecordInput{
			MerchantCode: "m001",
			GameCode:     "fishing",
			UserID:       10001,
			BetMinor:     bet,
			WinMinor:     win,
		})...)
	}
	if len(alerts) == 0 {
		t.Fatal("expected rtp alert")
	}
	if alerts[0].RtpPPM < 1800000 {
		t.Fatalf("unexpected rtp ppm: %d", alerts[0].RtpPPM)
	}
}
