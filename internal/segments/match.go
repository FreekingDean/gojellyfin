package segments

import (
	"math/bits"
	"slices"
	"time"
)

const (
	frameMatchBits   = 8
	runMeanBits      = 10.0
	smoothingFrames  = 16
	gapFrames        = 32
	candidateOffsets = 3
	minimumIntro     = 15 * time.Second
	maximumIntro     = 2 * time.Minute
)

type span struct {
	start, end time.Duration
}

func (s span) length() time.Duration {
	return s.end - s.start
}

func shared(a, b []uint32) (span, span, bool) {
	var bestA, bestB span
	for _, offset := range offsets(a, b) {
		first, last, ok := longestRun(a, b, offset)
		if !ok {
			continue
		}

		inB := span{start: at(first), end: at(last)}
		if inB.length() > bestB.length() {
			bestA = span{start: at(first + offset), end: at(last + offset)}
			bestB = inB
		}
	}

	return bestA, bestB, bestB.length() >= minimumIntro
}

func at(frame int) time.Duration {
	return time.Duration((float64(frame) + float64(frameSize)/hopSize/2) / framesPerSecond * float64(time.Second))
}

type scored struct {
	offset, score int
}

func offsets(a, b []uint32) []int {
	minimum := int(minimumIntro.Seconds() * framesPerSecond)
	var all []scored
	for offset := minimum - len(b); offset <= len(a)-minimum; offset++ {
		score := 0
		for k := max(0, -offset); k < len(b) && k+offset < len(a); k++ {
			if bits.OnesCount32(a[k+offset]^b[k]) <= frameMatchBits {
				score++
			}
		}
		all = append(all, scored{offset: offset, score: score})
	}

	slices.SortFunc(all, func(x, y scored) int { return y.score - x.score })

	var picked []int
	for _, candidate := range all {
		if len(picked) == candidateOffsets {
			break
		}
		if !slices.ContainsFunc(picked, func(offset int) bool { return abs(offset-candidate.offset) < smoothingFrames }) {
			picked = append(picked, candidate.offset)
		}
	}

	return picked
}

func longestRun(a, b []uint32, offset int) (int, int, bool) {
	from := max(0, -offset)
	to := min(len(b), len(a)-offset)
	if to-from < smoothingFrames {
		return 0, 0, false
	}

	sums := make([]int, to-from+1)
	for k := from; k < to; k++ {
		sums[k-from+1] = sums[k-from] + bits.OnesCount32(a[k+offset]^b[k])
	}

	var runs [][2]int
	for k := from; k < to; k++ {
		low := max(from, k-smoothingFrames/2) - from
		high := min(to, k+smoothingFrames/2+1) - from
		if float64(sums[high]-sums[low])/float64(high-low) > runMeanBits {
			continue
		}

		if last := len(runs) - 1; last >= 0 && k-runs[last][1] <= gapFrames {
			runs[last][1] = k
		} else {
			runs = append(runs, [2]int{k, k})
		}
	}

	maximum := int(maximumIntro.Seconds() * framesPerSecond)
	best, found := [2]int{}, false
	for _, run := range runs {
		if length := run[1] - run[0]; length <= maximum && (!found || length > best[1]-best[0]) {
			best, found = run, true
		}
	}

	return best[0], best[1], found
}

func abs(value int) int {
	if value < 0 {
		return -value
	}

	return value
}
