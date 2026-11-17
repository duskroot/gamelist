package room_test

import (
	"context"
	"testing"

	"den/internal/httpapi"
	"den/internal/platform/testbox"
)

func TestLeaveHighRollUnrolledPlayerResolves(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{
		Pool:   pool,
		Redis:  rdb,
		Roller: func() [5]int { return [5]int{3, 3, 3, 3, 3} },
	})
	_, token0 := login(t, h.Engine, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1", "离席甲")
	_, token1 := login(t, h.Engine, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2", "离席乙")
	created := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/CreateRoom", `{"Mode":"HighRoll","MaxSeats":2}`))
	roomID := created["RoomId"].(string)
	code := created["RoomCode"].(string)
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/JoinRoom", `{"RoomCode":"`+code+`"}`))
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	started := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/StartGame", `{"RoomId":"`+roomID+`"}`))
	roundID := started["RoundId"].(string)
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/LeaveRoom", `{"RoomId":"`+roomID+`"}`))

	view := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/DescribeRound", `{"RoundId":"`+roundID+`"}`))
	if view["Status"] != "Resolved" || view["WinnerSeat"].(float64) != 0 {
		t.Fatalf("round %#v", view)
	}
	room := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`))
	if room["Status"] != "Open" {
		t.Fatalf("room %#v", room["Status"])
	}
}

func TestLeaveLiarsDiceCurrentTurn(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{
		Pool:   pool,
		Redis:  rdb,
		Roller: func() [5]int { return [5]int{1, 2, 3, 4, 5} },
	})
	_, token0 := login(t, h.Engine, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb1", "叫席甲")
	_, token1 := login(t, h.Engine, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbb2", "叫席乙")
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
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/LeaveRoom", `{"RoomId":"`+roomID+`"}`))

	var turn int
	if err := pool.QueryRow(context.Background(), `SELECT turn_seat FROM dice_rounds WHERE round_id=$1`, roundID).Scan(&turn); err != nil {
		t.Fatal(err)
	}
	view := resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/dice/DescribeRound", `{"RoundId":"`+roundID+`"}`))
	room := resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/DescribeRoom", `{"RoomId":"`+roomID+`"}`))
	opened := authedPost(t, h.Engine, token1, "/den/v1/dice/Open", `{"RoundId":"`+roundID+`"}`)
	_, openErr := opened["Error"]
	canAct := !openErr || turn == 1
	if room["Status"] == "Playing" && !canAct {
		t.Fatalf("stuck playing turn %d open %#v", turn, opened)
	}
	if view["Status"] != "Resolved" || view["Outcome"] != "Win" || view["WinnerSeat"].(float64) != 1 || room["Status"] != "Open" {
		t.Fatalf("round %#v room %#v turn %d", view, room["Status"], turn)
	}
}

func TestLeaveLiarsDiceOpenCountsDepartedDice(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{
		Pool:   pool,
		Redis:  rdb,
		Roller: func() [5]int { return [5]int{6, 6, 6, 6, 6} },
	})
	_, token0 := login(t, h.Engine, "cccccccc-cccc-4ccc-8ccc-ccccccccccc1", "开骰甲")
	_, token1 := login(t, h.Engine, "cccccccc-cccc-4ccc-8ccc-ccccccccccc2", "开骰乙")
	_, token2 := login(t, h.Engine, "cccccccc-cccc-4ccc-8ccc-ccccccccccc3", "开骰丙")
	created := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/CreateRoom", `{"Mode":"LiarsDice","MaxSeats":3}`))
	roomID := created["RoomId"].(string)
	code := created["RoomCode"].(string)
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/JoinRoom", `{"RoomCode":"`+code+`"}`))
	resultOf(t, authedPost(t, h.Engine, token2, "/den/v1/room/JoinRoom", `{"RoomCode":"`+code+`"}`))
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	resultOf(t, authedPost(t, h.Engine, token2, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))
	started := resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/room/StartGame", `{"RoomId":"`+roomID+`"}`))
	roundID := started["RoundId"].(string)
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resultOf(t, authedPost(t, h.Engine, token2, "/den/v1/dice/Roll", `{"RoundId":"`+roundID+`"}`))
	resultOf(t, authedPost(t, h.Engine, token0, "/den/v1/dice/Call", `{"RoundId":"`+roundID+`","Count":11,"Face":6}`))
	resultOf(t, authedPost(t, h.Engine, token1, "/den/v1/room/LeaveRoom", `{"RoomId":"`+roomID+`"}`))

	var hands int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM dice_hands WHERE round_id=$1`, roundID).Scan(&hands); err != nil {
		t.Fatal(err)
	}
	if hands != 3 {
		t.Fatalf("hands %d", hands)
	}
	opened := resultOf(t, authedPost(t, h.Engine, token2, "/den/v1/dice/Open", `{"RoundId":"`+roundID+`"}`))
	if opened["WinnerSeat"].(float64) != 0 || opened["Outcome"] != "Win" {
		t.Fatalf("open %#v", opened)
	}
}
