package hall_test

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

func postToken(t *testing.T, engine *route.Engine, device string) string {
	t.Helper()
	body := []byte(`{"DeviceId":"` + device + `","Nickname":"玩家甲乙"}`)
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
	return payload["Result"].(map[string]any)["Token"].(string)
}

func TestEnterHallAndRename(t *testing.T) {
	pool, rdb := testbox.Start(t)
	h := httpapi.New(httpapi.Deps{Pool: pool, Redis: rdb, Publisher: realtime.Nop{}})
	token := postToken(t, h.Engine, "22222222-2222-4222-8222-222222222222")
	w := ut.PerformRequest(h.Engine, "POST", "/den/v1/hall/EnterHall", nil)
	var denied map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatal(err)
	}
	if denied["Error"].(map[string]any)["Code"] != "100003" {
		t.Fatalf("expected 100003 %#v", denied)
	}
	w = ut.PerformRequest(h.Engine, "POST", "/den/v1/hall/EnterHall", nil,
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	var hall map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &hall); err != nil {
		t.Fatal(err)
	}
	if hall["Result"].(map[string]any)["Nickname"] != "玩家甲乙" {
		t.Fatalf("hall %#v", hall)
	}
	body := []byte(`{"Nickname":"新名字"}`)
	w = ut.PerformRequest(h.Engine, "POST", "/den/v1/account/UpdateNickname",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	var renamed map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &renamed); err != nil {
		t.Fatal(err)
	}
	if renamed["Result"].(map[string]any)["Nickname"] != "新名字" {
		t.Fatalf("rename %#v", renamed)
	}
}
