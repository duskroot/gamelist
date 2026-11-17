package dice

import "testing"

func TestCompareHighRoll(t *testing.T) {
	winner, draw := CompareHighRoll([][5]int{{6, 6, 6, 6, 6}, {1, 1, 1, 1, 1}})
	if draw || winner != 0 {
		t.Fatalf("winner %d draw %v", winner, draw)
	}
	winner, draw = CompareHighRoll([][5]int{{1, 2, 3, 4, 5}, {5, 4, 3, 2, 1}})
	if !draw || winner != -1 {
		t.Fatalf("expected draw, winner %d draw %v", winner, draw)
	}
	winner, draw = CompareHighRoll([][5]int{{6, 1, 1, 1, 1}, {5, 2, 1, 1, 1}})
	if draw || winner != 0 {
		t.Fatalf("lex winner %d draw %v", winner, draw)
	}
}

func TestCallAndOpen(t *testing.T) {
	if !CallHigher(false, 0, 0, 1, 1) {
		t.Fatal("first call")
	}
	if CallHigher(true, 2, 3, 2, 3) {
		t.Fatal("equal call must fail")
	}
	if !CallHigher(true, 2, 3, 2, 4) {
		t.Fatal("higher face")
	}
	if !CallHigher(true, 2, 6, 3, 1) {
		t.Fatal("higher count")
	}
	hands := [][5]int{{1, 1, 1, 2, 3}, {1, 4, 5, 6, 6}}
	if CountFace(hands, 1) != 4 {
		t.Fatalf("ones are not wild, count %d", CountFace(hands, 1))
	}
}
