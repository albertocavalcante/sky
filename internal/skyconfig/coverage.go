package skyconfig

import (
	"math/big"
	"strconv"
)

// CoverageMeets reports whether covered of total lines meets threshold
// percent. The comparison is exact: threshold is read as its shortest decimal
// form (so 64.4 means 64.4, not the nearest float64) and compared as a
// rational with covered*100/total. Float arithmetic rounds on inputs such as
// 57/100 (division first) or 161/250 at 64.4 (multiplication first) and would
// fail a run whose coverage equals the threshold.
//
// A threshold of 0 or less is always met, and so is a total of 0 or less
// (callers report "no data" separately). A NaN or +Inf threshold is never met.
func CoverageMeets(covered, total int, threshold float64) bool {
	if threshold <= 0 || total <= 0 {
		return true
	}
	t, ok := new(big.Rat).SetString(strconv.FormatFloat(threshold, 'f', -1, 64))
	if !ok {
		return false
	}
	// covered*100/total >= t  <=>  covered*100 >= t*total
	lhs := new(big.Rat).SetInt64(int64(covered) * 100)
	rhs := t.Mul(t, new(big.Rat).SetInt64(int64(total)))
	return lhs.Cmp(rhs) >= 0
}
