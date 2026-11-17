package realtime_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"den/internal/httpapi"
	"den/internal/platform/testbox"
	"den/internal/realtime"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"github.com/gorilla/websocket"
)

func TestReadyPushesRoomChanged(t *testing.T) {
	pool, rdb := testbox.Start(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := server.New(server.WithListener(ln), server.WithTransport(standard.NewTransporter))
	h.NoHijackConnPool = true
	hub := &realtime.Hub{}
	httpapi.Register(h, httpapi.Deps{Pool: pool, Redis: rdb, Hub: hub})
	go h.Spin()
	t.Cleanup(func() { _ = h.Close() })

	base := "http://" + ln.Addr().String()
	waitReady(t, base)

	hostID, hostToken := login(t, base, "12121212-1212-4121-8121-121212121212", "房主甲")
	_, guestToken := login(t, base, "13131313-1313-4131-8131-131313131313", "玩家乙")
	created := resultOf(t, authedPost(t, base, hostToken, "/den/v1/room/CreateRoom", `{"Mode":"HighRoll"}`))
	roomID := created["RoomId"].(string)
	roomCode := created["RoomCode"].(string)
	resultOf(t, authedPost(t, base, guestToken, "/den/v1/room/JoinRoom", `{"RoomCode":"`+roomCode+`"}`))

	conn := dial(t, ln.Addr().String())
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.WriteJSON(map[string]string{"Token": guestToken, "RoomId": roomID}); err != nil {
		t.Fatal(err)
	}
	waitSubscribed(t, hub, roomID)

	resultOf(t, authedPost(t, base, hostToken, "/den/v1/room/Ready", `{"RoomId":"`+roomID+`","Ready":true}`))

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var frame map[string]any
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatal(err)
	}
	meta, _ := frame["ResponseMetadata"].(map[string]any)
	if meta["Action"] != "RoomChanged" || meta["Service"] != "room" {
		t.Fatalf("metadata %#v", frame["ResponseMetadata"])
	}
	result, _ := frame["Result"].(map[string]any)
	seats, _ := result["Seats"].([]any)
	hostReady := false
	for _, raw := range seats {
		seat, _ := raw.(map[string]any)
		if seat["UserId"] == hostID && seat["Ready"] == true {
			hostReady = true
		}
	}
	if !hostReady {
		t.Fatalf("seats %#v", result["Seats"])
	}

	bad := dial(t, ln.Addr().String())
	t.Cleanup(func() { _ = bad.Close() })
	if err := bad.WriteJSON(map[string]string{"Token": "not-a-token", "RoomId": roomID}); err != nil {
		t.Fatal(err)
	}
	_ = bad.SetReadDeadline(time.Now().Add(3 * time.Second))
	var denied map[string]any
	if err := bad.ReadJSON(&denied); err != nil {
		t.Fatal(err)
	}
	errBody, _ := denied["Error"].(map[string]any)
	if errBody["Code"] != "100003" {
		t.Fatalf("error %#v", denied["Error"])
	}
	deniedMeta, _ := denied["ResponseMetadata"].(map[string]any)
	if deniedMeta["Service"] != "room" || deniedMeta["Action"] != "Subscribe" {
		t.Fatalf("metadata %#v", denied["ResponseMetadata"])
	}
	if _, _, err := bad.ReadMessage(); err == nil {
		t.Fatal("expected closed socket")
	}
}

func waitReady(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		resp, err := http.Post(base+"/den/v1/account/GuestLogin", "application/json", strings.NewReader(`{}`))
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return
		}
		last = err
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(last)
}

func waitSubscribed(t *testing.T, hub *realtime.Hub, roomID string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hub.Connected(roomID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("connection was not registered")
}

func dial(t *testing.T, addr string) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial("ws://"+addr+"/den/v1/room/Subscribe", nil)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			t.Fatalf("dial %v status %s body %s", err, resp.Status, body)
		}
		t.Fatal(err)
	}
	return conn
}

func login(t *testing.T, base, device, nickname string) (string, string) {
	t.Helper()
	payload := resultOf(t, postJSON(t, base, "/den/v1/account/GuestLogin", "", `{"DeviceId":"`+device+`","Nickname":"`+nickname+`"}`))
	return payload["UserId"].(string), payload["Token"].(string)
}

func authedPost(t *testing.T, base, token, path, body string) map[string]any {
	t.Helper()
	return postJSON(t, base, path, token, body)
}

func postJSON(t *testing.T, base, path, token, body string) map[string]any {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", resp.StatusCode, raw)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
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
