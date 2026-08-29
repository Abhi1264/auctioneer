package engine

import "testing"

func TestDecodeCachedResult(t *testing.T) {
	got := decodeCachedResult("1|accepted|150|u1|3|evt-9|1710000000000")
	if !got.Accepted || got.Reason != "accepted" || got.CurrentPrice != 150 || got.WinnerUserID != "u1" || got.Version != 3 || got.EventID != "evt-9" || got.ServerUnixMilli != 1710000000000 {
		t.Fatalf("unexpected result: %+v", got)
	}

	low := decodeCachedResult("0|bid_too_low|100||2||1710000000000")
	if low.Accepted || low.Reason != "bid_too_low" || low.CurrentPrice != 100 || low.WinnerUserID != "" || low.EventID != "" {
		t.Fatalf("unexpected rejected result: %+v", low)
	}
}
