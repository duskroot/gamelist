package dice_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"den/internal/httpapi"
	"den/internal/platform/testbox"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
)

type pubEvent struct {
	RoomID string
	UserID string
	Event  realtime.Event
}

type capturePub struct {
	roomEvents []pubEvent
	userEvents []pubEvent
}

func (c *capturePub) EmitRoom(roomID string, ev realtime.Event) {
	c.roomEvents = append(c.roomEvents, pubEvent{RoomID: roomID, Event: ev})
}

func (c *capturePub) EmitUser(roomID, userID string, ev realtime.Event) {
	c.userEvents = append(c.userEvents, pubEvent{RoomID: roomID, UserID: userID, Event: ev})
}

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

func asInts(t *testing.T, v any) []int {
	t.Helper()
	raw, ok := v.([]any)
	if !ok {
		t.Fatalf("dice %#v", v)
	}
	out := make([]int, len(raw))
	for i, n := range raw {
		out[i] = int(n.(float64))
	}
	return out
}

func sameDice(got []int, want [5]int) bool {
	if len(got) != 5 {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func describeRound(t *testing.T, engine *route.Engine, token, roundID string) map[string]any {
	t.Helper()
	return resultOf(t, authedPost(t, engine, token, "/den/v1/dice/DescribeRound", `{"RoundId":"`+roundID+`"}`))
}

func assertHandsEmpty(t *testing.T, view map[string]any) {
	t.Helper()
	hands, ok := view["Hands"].([]any)
	if !ok || len(hands) != 0 {
		t.Fatalf("Hands %#v", view["Hands"])
	}
}

func assertAllUnready(t *testing.T, room map[string]any) {
	t.Helper()
	if room["Status"] != "Open" {
		t.Fatalf("room %#v", room["Status"])
	}
	seats, ok := room["Seats"].([]any)
	if !ok || len(seats) != 2 {
		t.Fatalf("seats %#v", room["Seats"])
	}
	for _, raw := range seats {
		seat := raw.(map[string]any)
		if seat["Ready"] != false {
			t.Fatalf("ready %#v", seat)
		}
	}
}

func TestHighRollRoll(t *testing.T) {
	pool, rdb := testbox.Start(t)
	pub := &capturePub{}
	hand := [5]int{1, 1, 1, 1, 1}
	h := httpapi.New(httpapi.Deps{
		Pool:      pool,
		Redis:     rdb,
		Publisher: pub,
		Roller:    func() [5]int { return hand },
	})
	_, token0 := login(t, h.Engine, "66666666-6666-4666-8666-666666666666", "骰子甲")
	user1, token1 := login(t, h.Engine, "77777777-7777-4777-8777-777777777777", "骰子乙")
	created := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/CreateRoom", `{"Mode":"HighRoll","MaxSeats":2}`))
	roomID := created["RoomId"].(string)
	code := created["RoomCode"].(string)
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/JoinRoom", `{"RoomCode":"`+code+`"}`))
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	started := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/StartGame", `{"RoomId":"`+roomID+`"}`))
	roundID := started["RoundId"].(string)

	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	mine := describeRound(t, h.Engine, token0, roundID)
	if !sameDice(asInts(t, mine["YourDice"]), hand) {
		t.Fatalf("YourDice %#v", mine["YourDice"])
	}
	assertHandsEmpty(t, mine)
	if mine["Revealed"] != false || mine["Status"] != "Rolling" || mine["WinnerSeat"].(float64) != -1 {
		t.Fatalf("hidden round %#v", mine)
	}
	other := describeRound(t, h.Engine, token1, roundID)
	if len(asInts(t, other["YourDice"])) != 0 {
		t.Fatalf("other dice %#v", other["YourDice"])
	}
	assertHandsEmpty(t, other)

	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resolved := describeRound(t, h.Engine, token0, roundID)
	if resolved["Outcome"] != "Draw" || resolved["WinnerSeat"].(float64) != -1 || resolved["Status"] != "Resolved" || resolved["Revealed"] != true {
		t.Fatalf("resolved %#v", resolved)
	}
	hands := resolved["Hands"].([]any)
	if len(hands) != 2 {
		t.Fatalf("hands %#v", hands)
	}
	for _, raw := range hands {
		seat := raw.(map[string]any)
		if !sameDice(asInts(t, seat["Dice"]), hand) {
			t.Fatalf("visible %#v", seat)
		}
	}
	room := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`))
	assertAllUnready(t, room)

	again := authedPost(t, h.Engine, token0, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`)
	if errorCode(t, again) != "300002" {
		t.Fatalf("second roll %#v", again["Error"])
	}
	still := describeRound(t, h.Engine, token0, roundID)
	if !sameDice(asInts(t, still["YourDice"]), hand) {
		t.Fatalf("dice changed %#v", still["YourDice"])
	}

	var hidden, resolvedEv int
	for _, ev := range pub.roomEvents {
		if ev.Event.Service != "dice" {
			continue
		}
		body, _ := ev.Event.Result.(map[string]any)
		switch ev.Event.Action {
		case "DiceHidden":
			hidden++
			if body["RoundId"] != roundID || body["Dice"] != nil || body["SeatNo"] == nil {
				t.Fatalf("DiceHidden %#v", body)
			}
		case "RoundResolved":
			resolvedEv++
			if body["Outcome"] != "Draw" || body["WinnerSeat"] != -1 {
				t.Fatalf("RoundResolved %#v", body)
			}
		}
	}
	if hidden != 2 || resolvedEv != 1 {
		t.Fatalf("room events hidden %d resolved %d", hidden, resolvedEv)
	}
	rolled := 0
	for _, ev := range pub.userEvents {
		if ev.Event.Action != "DiceRolled" || ev.Event.Service != "dice" {
			continue
		}
		rolled++
		body := ev.Event.Result.(map[string]any)
		dice, ok := body["Dice"].([]int)
		if !ok || !sameDice(dice, hand) {
			t.Fatalf("DiceRolled %#v", body)
		}
		if ev.UserID == user1 && body["SeatNo"] != 1 {
			t.Fatalf("seat %#v", body["SeatNo"])
		}
	}
	if rolled != 2 {
		t.Fatalf("DiceRolled %d", rolled)
	}
}

func TestLiarsDiceOpen(t *testing.T) {
	pool, rdb := testbox.Start(t)
	pub := &capturePub{}
	hand := [5]int{6, 2, 2, 2, 2}
	h := httpapi.New(httpapi.Deps{
		Pool:      pool,
		Redis:     rdb,
		Publisher: pub,
		Roller:    func() [5]int { return hand },
	})
	_, token0 := login(t, h.Engine, "88888888-8888-4888-8888-888888888888", "骰子丙")
	_, token1 := login(t, h.Engine, "99999999-9999-4999-8999-999999999999", "骰子丁")
	created := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/CreateRoom", `{"Mode":"LiarsDice","MaxSeats":2}`))
	roomID := created["RoomId"].(string)
	code := created["RoomCode"].(string)
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/JoinRoom", `{"RoomCode":"`+code+`"}`))
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	started := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/StartGame", `{"RoomId":"`+roomID+`"}`))
	roundID := started["RoundId"].(string)

	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	calling := describeRound(t, h.Engine, token0, roundID)
	if calling["Status"] != "Calling" || calling["Revealed"] != false {
		t.Fatalf("calling %#v", calling)
	}
	assertHandsEmpty(t, calling)
	if !sameDice(asInts(t, calling["YourDice"]), hand) {
		t.Fatalf("YourDice %#v", calling["YourDice"])
	}

	earlyOpen := authedPost(t, h.Engine, token0, "/den/v1/dice/Open", `{"RoundId":"`+roundID+`"}`)
	if errorCode(t, earlyOpen) != "300002" {
		t.Fatalf("open before call %#v", earlyOpen["Error"])
	}
	early := authedPost(t, h.Engine, token1, "/den/v1/dice/Call", `{"RoundId":"`+roundID+`","Count":1,"Face":6}`)
	if errorCode(t, early) != "300001" {
		t.Fatalf("seat 1 before seat 0 %#v", early["Error"])
	}
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Call", `{"RoundId":"`+roundID+`","Count":1,"Face":6}`))
	same := authedPost(t, h.Engine, token1, "/den/v1/dice/Call", `{"RoundId":"`+roundID+`","Count":1,"Face":6}`)
	if errorCode(t, same) != "300002" {
		t.Fatalf("equal call %#v", same["Error"])
	}
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/dice/Open", `{"RoundId":"`+roundID+`"}`))

	opened := describeRound(t, h.Engine, token1, roundID)
	if opened["WinnerSeat"].(float64) != 0 || opened["Outcome"] != "Win" || opened["Status"] != "Resolved" || opened["Revealed"] != true {
		t.Fatalf("opened %#v", opened)
	}
	hands := opened["Hands"].([]any)
	if len(hands) != 2 {
		t.Fatalf("hands %#v", hands)
	}
	room := resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`))
	assertAllUnready(t, room)

	var called, openedEv, resolvedEv int
	for _, ev := range pub.roomEvents {
		if ev.Event.Service != "dice" {
			continue
		}
		body, _ := ev.Event.Result.(map[string]any)
		switch ev.Event.Action {
		case "DiceCalled":
			called++
			if body["Count"] != 1 || body["Face"] != 6 || body["SeatNo"] != 0 {
				t.Fatalf("DiceCalled %#v", body)
			}
		case "DiceOpened":
			openedEv++
			if body["Hands"] == nil {
				t.Fatalf("DiceOpened %#v", body)
			}
		case "RoundResolved":
			resolvedEv++
			if body["Outcome"] != "Win" || body["WinnerSeat"] != 0 {
				t.Fatalf("RoundResolved %#v", body)
			}
		}
	}
	if called != 1 || openedEv != 1 || resolvedEv != 1 {
		t.Fatalf("events called %d opened %d resolved %d", called, openedEv, resolvedEv)
	}
}
