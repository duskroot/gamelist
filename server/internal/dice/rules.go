package dice

import "sort"

// CompareHighRoll ranks hands by sum, then by the five dice sorted descending.
// The winner is the index of the single best hand. A shared best hand is a draw.
func CompareHighRoll(hands [][5]int) (winner int, draw bool) {
	if len(hands) == 0 {
		return -1, true
	}
	best := 0
	ties := 1
	for i := 1; i < len(hands); i++ {
		switch compareHands(hands[i], hands[best]) {
		case 1:
			best = i
			ties = 1
		case 0:
			ties++
		}
	}
	if ties > 1 {
		return -1, true
	}
	return best, false
}

func compareHands(a, b [5]int) int {
	if sa, sb := sum(a), sum(b); sa != sb {
		if sa > sb {
			return 1
		}
		return -1
	}
	as, bs := sortedDesc(a), sortedDesc(b)
	for i := 0; i < 5; i++ {
		if as[i] != bs[i] {
			if as[i] > bs[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

func sum(hand [5]int) int {
	total := 0
	for _, die := range hand {
		total += die
	}
	return total
}

func sortedDesc(hand [5]int) [5]int {
	out := hand
	sort.Slice(out[:], func(i, j int) bool { return out[i] > out[j] })
	return out
}

// CallHigher reports whether count/face is a legal opening bid or a strictly higher bid.
// A higher bid raises the count, or keeps the count and raises the face.
func CallHigher(hasPrev bool, prevCount, prevFace, count, face int) bool {
	if count < 1 || face < 1 || face > 6 {
		return false
	}
	if !hasPrev {
		return true
	}
	if count > prevCount {
		return true
	}
	return count == prevCount && face > prevFace
}

// CountFace counts exact matches. One is not wild.
func CountFace(hands [][5]int, face int) int {
	n := 0
	for _, hand := range hands {
		for _, die := range hand {
			if die == face {
				n++
			}
		}
	}
	return n
}

// nextOccupied returns the next seat strictly above current, or the lowest seat.
func nextOccupied(seats []int, current int) int {
	for _, seat := range seats {
		if seat > current {
			return seat
		}
	}
	if len(seats) == 0 {
		return current
	}
	return seats[0]
}
