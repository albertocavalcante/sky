package skyconfig

import (
	"math"
	"math/big"
	"strconv"
	"testing"
)

// exactMeets decides covered*100/total >= tenths/10 with integers only:
// covered*1000 >= tenths*total.
func exactMeets(covered, total, tenths int) bool {
	return covered*1000 >= tenths*total
}

// Every one-decimal threshold 0.1..100.0 against totals 1..300: the equality
// case (when it exists) and the one-line-short case must agree with exact
// arithmetic on the decimal string.
func TestCoverageMeetsOneDecimalTable(t *testing.T) {
	for tenths := 1; tenths <= 1000; tenths++ {
		text := strconv.Itoa(tenths/10) + "." + strconv.Itoa(tenths%10)
		threshold, err := strconv.ParseFloat(text, 64)
		if err != nil {
			t.Fatal(err)
		}
		rat, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatalf("bad decimal %q", text)
		}
		for total := 1; total <= 300; total++ {
			// Smallest covered count that meets the threshold, and its neighbours.
			need := (tenths*total + 999) / 1000
			for _, covered := range []int{need - 1, need, need + 1} {
				if covered < 0 || covered > total {
					continue
				}
				want := exactMeets(covered, total, tenths)
				oracle := new(big.Rat).SetFrac64(int64(covered)*100, int64(total)).Cmp(rat) >= 0
				if oracle != want {
					t.Fatalf("test oracle disagrees for %d/%d at %s", covered, total, text)
				}
				if got := CoverageMeets(covered, total, threshold); got != want {
					t.Fatalf("CoverageMeets(%d, %d, %s) = %v, want %v", covered, total, text, got, want)
				}
			}
		}
	}
}

func TestCoverageMeetsKnownCases(t *testing.T) {
	tests := []struct {
		covered, total int
		threshold      float64
		want           bool
	}{
		{161, 250, 64.4, true}, // 64.4*250 rounds to 16100.000000000002
		{160, 250, 64.4, false},
		{57, 100, 57, true}, // 57/100*100 rounds to 56.99999999999999
		{56, 100, 57, false},
		{0, 0, 80, true}, // no data is the caller's concern
		{5, 10, 0, true},
		{0, 10, 0, true},
		{0, 10, 0.1, false},
		{10, 10, 100, true},
		{9, 10, 100, false},
		{1, 3, math.NaN(), false},
		{1, 3, math.Inf(1), false},
	}
	for _, tt := range tests {
		if got := CoverageMeets(tt.covered, tt.total, tt.threshold); got != tt.want {
			t.Errorf("CoverageMeets(%d, %d, %v) = %v, want %v", tt.covered, tt.total, tt.threshold, got, tt.want)
		}
	}
}
