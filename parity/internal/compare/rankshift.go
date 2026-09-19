package compare

import (
	"regexp"
	"strconv"
)

// rankToken matches a normalised timestamp: <ts#12|f6|Z>.
var rankToken = regexp.MustCompile(`<ts#(\d+)\|([^>]*)>`)

// RankShift reports that every difference in a scenario is the SAME constant
// offset in <ts#N> ranks, with nothing else differing.
//
// Why this needs a name of its own. The rank is a dense rank over the DISTINCT
// instants a scenario observed, so two unrelated rows landing in the same
// microsecond on one stack and not on the other leave that stack one distinct
// value short, and every rank after it shifts by the same amount. Measured
// twice: 16/09 on W10, 143 comparisons all off by 3; 19/09, 15 comparisons all
// off by 1. Neither reproduced when the scenario was re-run (0/12, 0/5).
//
// The obvious repair -- rank by order of appearance instead of by value --
// was tried and REVERTED: it makes {0.1, 0.2} and {0.1, 0.1} normalise alike,
// which hides a port that reads the clock once and stamps two columns with it.
// `TestClockSourceChangeCollapsesRanks` exists for that bug, so the ranking
// stays as it is and the noise is named here instead.
//
// This is a classifier, not a pardon. A caller that sees RankShift must re-run
// the scenario; only a shift that DISAPPEARS on the re-run is background noise.
// One that survives is a real difference, because a port that shifts every
// timestamp rank by a constant has genuinely changed how many distinct moments
// it writes.
func RankShift(diffs []Difference) (offset int, ok bool) {
	if len(diffs) == 0 {
		return 0, false
	}
	seen := false
	// Every (reference, candidate) rank pair seen anywhere in the scenario, so
	// the equality STRUCTURE can be checked across differences, not just inside
	// one of them.
	var allRef, allCand []int
	for _, d := range diffs {
		refNums, refSkel := splitRanks(d.Reference)
		candNums, candSkel := splitRanks(d.Candidate)
		// Everything outside the rank numbers must be identical, including the
		// shape half of each token: a zone or precision change is never noise.
		if refSkel != candSkel || len(refNums) != len(candNums) || len(refNums) == 0 {
			return 0, false
		}
		for i := range refNums {
			delta := candNums[i] - refNums[i]
			if delta == 0 {
				continue
			}
			if !seen {
				offset, seen = delta, true
				continue
			}
			if delta != offset {
				return 0, false
			}
		}
		allRef = append(allRef, refNums...)
		allCand = append(allCand, candNums...)
	}
	// A renumbering must preserve which moments were the SAME moment. If two
	// fields had different ranks and now share one, the candidate collapsed two
	// instants into one -- a port reading the clock once for two columns -- and
	// that is the opposite of noise. The reverse, a split, matters just as much.
	for i := range allRef {
		for j := i + 1; j < len(allRef); j++ {
			if (allRef[i] == allRef[j]) != (allCand[i] == allCand[j]) {
				return 0, false
			}
		}
	}
	return offset, seen
}

// splitRanks returns the rank numbers in order, and the text with every rank
// number blanked so two strings can be compared ignoring the numbers alone.
func splitRanks(text string) ([]int, string) {
	nums := []int{}
	skeleton := rankToken.ReplaceAllStringFunc(text, func(token string) string {
		m := rankToken.FindStringSubmatch(token)
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return token
		}
		nums = append(nums, n)
		return "<ts#|" + m[2] + ">"
	})
	return nums, skeleton
}
