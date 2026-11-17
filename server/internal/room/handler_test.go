package room_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"unicode"

	"den/internal/httpapi"
	"den/internal/platform/testbox"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

type capturePub struct {
	events []realtime.Event
	rooms  []string
}

func (c *capturePub) EmitRoom(roomID string, ev realtime.Event) {
	c.rooms = append(c.rooms, roomID)
	c.events = append(c.events, ev)
}

func (c *capturePub) EmitUser(string, string, realtime.Event) {}

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

func TestRoomSeats(t *testing.T) {
	pool, rdb := testbox.Start(t)
	pub := &capturePub{}
	h := httpapi.New(httpapi.Deps{Pool: pool, Redis: rdb, Publisher: pub})

	user1, token1 := login(t, h.Engine, "33333333-3333-4333-8333-333333333333", "玩家甲乙")
	user2, token2 := login(t, h.Engine, "44444444-4444-4444-8444-444444444444", "玩家丙丁")
	_, token3 := login(t, h.Engine, "55555555-5555-4555-8555-555555555555", "玩家戊己")

	created := authedPost(t, h.Engine, token1, "/den/v1/room/CreateRoom", `{"Mode":"HighRoll"}`)
	createdResult := resultOf(t, created)
	if createdResult["SeatNo"].(float64) != 0 {
		t.Fatalf("seat %#v", createdResult["SeatNo"])
	}
	roomCode, _ := createdResult["RoomCode"].(string)
	if len(roomCode) != 6 {
		t.Fatalf("room code %q", roomCode)
	}
	for _, r := range roomCode {
		if !unicode.IsDigit(r) {
			t.Fatalf("room code %q", roomCode)
		}
	}
	roomID := createdResult["RoomId"].(string)

	joined := authedPost(t, h.Engine, token2, "/den/v1/room/JoinRoom", `{"RoomCode":"`+roomCode+`"}`)
	if resultOf(t, joined)["SeatNo"].(float64) != 1 {
		t.Fatalf("join %#v", joined["Result"])
	}
	again := authedPost(t, h.Engine, token2, "/den/v1/room/JoinRoom", `{"RoomCode":"`+roomCode+`"}`)
	if resultOf(t, again)["SeatNo"].(float64) != 1 {
		t.Fatalf("rejoin %#v", again["Result"])
	}
	described := authedPost(t, h.Engine, token1, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`)
	seats := resultOf(t, described)["Seats"].([]any)
	if len(seats) != 2 {
		t.Fatalf("seats %#v", seats)
	}
	seenUser := 0
	for _, raw := range seats {
		seat := raw.(map[string]any)
		if seat["UserId"] == user2 {
			seenUser++
		}
	}
	if seenUser != 1 {
		t.Fatalf("user seats %d %#v", seenUser, seats)
	}

	full := authedPost(t, h.Engine, token1, "/den/v1/room/CreateRoom", `{"Mode":"HighRoll","MaxSeats":2}`)
	fullResult := resultOf(t, full)
	fullCode := fullResult["RoomCode"].(string)
	fullID := fullResult["RoomId"].(string)
	if fullResult["SeatNo"].(float64) != 0 {
		t.Fatalf("full host %#v", fullResult)
	}
	second := authedPost(t, h.Engine, token2, "/den/v1/room/JoinRoom", `{"RoomCode":"`+fullCode+`"}`)
	if resultOf(t, second)["SeatNo"].(float64) != 1 {
		t.Fatalf("second %#v", second["Result"])
	}
	third := authedPost(t, h.Engine, token3, "/den/v1/room/JoinRoom", `{"RoomCode":"`+fullCode+`"}`)
	if errorCode(t, third) != "200002" {
		t.Fatalf("full %#v", third["Error"])
	}
	notIn := authedPost(t, h.Engine, token3, "/den/v1/room/Ready", `{"RoomId":"`+fullID+`","Ready":true}`)
	if errorCode(t, notIn) != "200003" {
		t.Fatalf("ready %#v", notIn["Error"])
	}

	ready1 := authedPost(t, h.Engine, token1, "/den/v1/room/Ready", `{"RoomId":"`+fullID+`","Ready":true}`)
	resultOf(t, ready1)
	ready2 := authedPost(t, h.Engine, token2, "/den/v1/room/Ready", `{"RoomId":"`+fullID+`","Ready":true}`)
	resultOf(t, ready2)
	denied := authedPost(t, h.Engine, token2, "/den/v1/room/StartGame", `{"RoomId":"`+fullID+`"}`)
	if errorCode(t, denied) != "200004" {
		t.Fatalf("start %#v", denied["Error"])
	}
	started := authedPost(t, h.Engine, token1, "/den/v1/room/StartGame", `{"RoomId":"`+fullID+`"}`)
	roundID, _ := resultOf(t, started)["RoundId"].(string)
	if roundID == "" {
		t.Fatal("empty RoundId")
	}
	var status, outcome string
	var turnSeat, winnerSeat int
	if err := pool.QueryRow(context.Background(), `SELECT status, turn_seat, winner_seat, outcome FROM dice_rounds WHERE round_id=$1 AND room_id=$2`, roundID, fullID).Scan(&status, &turnSeat, &winnerSeat, &outcome); err != nil {
		t.Fatal(err)
	}
	if status != "Rolling" || turnSeat != 0 || winnerSeat != -1 || outcome != "" {
		t.Fatalf("round %s turn %d winner %d outcome %q", status, turnSeat, winnerSeat, outcome)
	}
	var roomStatus string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM rooms WHERE room_id=$1`, fullID).Scan(&roomStatus); err != nil {
		t.Fatal(err)
	}
	if roomStatus != "Playing" {
		t.Fatalf("room status %s", roomStatus)
	}
	againStart := authedPost(t, h.Engine, token1, "/den/v1/room/StartGame", `{"RoomId":"`+fullID+`"}`)
	if errorCode(t, againStart) != "300002" {
		t.Fatalf("restart %#v", againStart["Error"])
	}

	if user1 == "" {
		t.Fatal("missing user")
	}
	for _, ev := range pub.events {
		if ev.Service != "room" || ev.Action != "RoomChanged" || ev.Result == nil {
			t.Fatalf("event %#v", ev)
		}
	}
	if len(pub.events) == 0 {
		t.Fatal("expected RoomChanged")
	}
}
