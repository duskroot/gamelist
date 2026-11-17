package match_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"unicode"

	"den/internal/httpapi"
	"den/internal/platform/testbox"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

func login(t *testing.T, engine *route.Engine, device, nickname string) (userID, token string) {
	t.Helper()
	body := []byte(`{"DeviceId":"` + device + `","Nickname":"` + nickname + `"}`)
	w := ut.PerformRequest(engine, "POST", "/den/v1/account/GuestLogin",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["Result"] == nil {
		t.Fatalf("login failed %s", w.Body.String())
	}
	result := payload["Result"].(map[string]any)
	return result["UserId"].(string), result["Token"].(string)
}

func authedPost(t *testing.T, engine *route.Engine, token, path, jsonBody string) map[string]any {
	t.Helper()
	body := []byte(jsonBody)
	w := ut.PerformRequest(engine, "POST", path,
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func resultOf(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	if _, ok := payload["Error"]; ok {
		t.Fatalf("error %#v", payload["Error"])
	}
	result, ok := payload["Result"].(map[string]any)
	if !ok {
		t.Fatalf("result %#v", payload["Result"])
	}
	return result
}

func errorCode(t *testing.T, payload map[string]any) string {
	t.Helper()
	errBody, ok := payload["Error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error %#v", payload)
	}
	return errBody["Code"].(string)
}

func assertWaiting(t *testing.T, payload map[string]any) string {
	t.Helper()
	result := resultOf(t, payload)
	if result["Status"] != "Waiting" {
		t.Fatalf("status %#v", result["Status"])
	}
	matchID, _ := result["MatchId"].(string)
	if matchID == "" {
		t.Fatalf("match id %#v", result["MatchId"])
	}
	return matchID
}

func sixDigit(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func TestMatchTwoPlayers(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{Pool: pool, Redis: rdb})
	_, tokenA := login(t, h.Engine, "11111111-1111-4111-8111-111111111111", "匹配甲")
	_, tokenB := login(t, h.Engine, "22222222-2222-4222-8222-222222222222", "匹配乙")
	_, tokenC := login(t, h.Engine, "33333333-3333-4333-8333-333333333333", "匹配丙")

	startA := authedPost(t, h.Engine, tokenA, "/den/v1/match/StartMatch", `{"Mode":"HighRoll","MaxSeats":2}`)
	startB := authedPost(t, h.Engine, tokenB, "/den/v1/match/StartMatch", `{"Mode":"HighRoll","MaxSeats":2}`)
	matchA := assertWaiting(t, startA)
	matchB := assertWaiting(t, startB)

	descA := resultOf(t, authedPost(t, h.Engine, tokenA, "/den/v1/match/DescribeMatch", `{"MatchId":"`+matchA+`"}`))
	descB := resultOf(t, authedPost(t, h.Engine, tokenB, "/den/v1/match/DescribeMatch", `{"MatchId":"`+matchB+`"}`))
	if descA["Status"] != "Matched" || descB["Status"] != "Matched" {
		t.Fatalf("describe %#v %#v", descA["Status"], descB["Status"])
	}
	roomID, _ := descA["RoomId"].(string)
	roomCode, _ := descA["RoomCode"].(string)
	if roomID == "" || descB["RoomId"] != roomID {
		t.Fatalf("room id %#v %#v", descA["RoomId"], descB["RoomId"])
	}
	if !sixDigit(roomCode) || descB["RoomCode"] != roomCode {
		t.Fatalf("room code %#v %#v", descA["RoomCode"], descB["RoomCode"])
	}

	room := resultOf(t, authedPost(t, h.Engine, tokenA, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`))
	seats, _ := room["Seats"].([]any)
	if len(seats) != 2 {
		t.Fatalf("seats %#v", room["Seats"])
	}

	// The third player queues alone, so the second start is still Waiting.
	startC := authedPost(t, h.Engine, tokenC, "/den/v1/match/StartMatch", `{"Mode":"HighRoll","MaxSeats":2}`)
	matchC := assertWaiting(t, startC)
	againC := authedPost(t, h.Engine, tokenC, "/den/v1/match/StartMatch", `{"Mode":"HighRoll","MaxSeats":2}`)
	if errorCode(t, againC) != "400003" {
		t.Fatalf("again %#v", againC["Error"])
	}

	canceled := resultOf(t, authedPost(t, h.Engine, tokenC, "/den/v1/match/CancelMatch", `{"MatchId":"`+matchC+`"}`))
	if canceled["MatchId"] != matchC || canceled["Status"] != "Canceled" {
		t.Fatalf("cancel %#v", canceled)
	}
	describedC := resultOf(t, authedPost(t, h.Engine, tokenC, "/den/v1/match/DescribeMatch", `{"MatchId":"`+matchC+`"}`))
	if describedC["Status"] != "Canceled" {
		t.Fatalf("canceled describe %#v", describedC["Status"])
	}
	cancelAgain := authedPost(t, h.Engine, tokenC, "/den/v1/match/CancelMatch", `{"MatchId":"`+matchC+`"}`)
	if errorCode(t, cancelAgain) != "400002" {
		t.Fatalf("cancel again %#v", cancelAgain["Error"])
	}

	// Matched, still seated, and no longer Waiting, so a new match is allowed.
	assertWaiting(t, authedPost(t, h.Engine, tokenA, "/den/v1/match/StartMatch", `{"Mode":"HighRoll","MaxSeats":2}`))

	unknown := authedPost(t, h.Engine, tokenA, "/den/v1/match/DescribeMatch", `{"MatchId":"missing-match"}`)
	if errorCode(t, unknown) != "400001" {
		t.Fatalf("unknown %#v", unknown["Error"])
	}
}
