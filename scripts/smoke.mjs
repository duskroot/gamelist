#!/usr/bin/env node
/**
 * End-to-end smoke against table-server.
 * Usage: API_BASE=http://127.0.0.1:8080 npm run smoke
 */
const API_BASE = (process.env.API_BASE || "http://127.0.0.1:8080").replace(/\/$/, "");

async function call(service, action, body = {}, token = "") {
  const headers = { "Content-Type": "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}/table/v1/${service}/${action}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
  const json = await res.json();
  if (json.Error) {
    const err = new Error(`${json.Error.Code} ${json.Error.Message}`);
    err.detail = json.Error.Detail;
    err.requestId = json.ResponseMetadata?.RequestId;
    throw err;
  }
  return json.Result;
}

async function login(code, nickname) {
  return call("session", "LoginByWechat", { Code: code, Nickname: nickname }, "");
}

async function main() {
  const health = await call("session", "DescribeHealth", {}, "");
  if (health.Status !== "ok") throw new Error("health failed");

  const a = await login(`smoke_${Date.now()}_a`, "甲");
  const b = await login(`smoke_${Date.now()}_b`, "乙");

  const room = await call(
    "room",
    "CreateRoom",
    { GameCode: "dice", Variant: "classic", Config: { Count: 2 }, MaxSeats: 4 },
    a.Token,
  );
  await call("room", "JoinRoom", { Code: room.Code }, b.Token);
  await call("room", "SetReady", { RoomId: room.RoomId, Ready: true }, a.Token);
  await call("room", "SetReady", { RoomId: room.RoomId, Ready: true }, b.Token);

  let snap = await call("play", "StartPlay", { RoomId: room.RoomId }, a.Token);
  let clientSeqA = 0;
  let clientSeqB = 0;
  let guard = 0;
  while (!snap.Settled && guard++ < 20) {
    const actor = snap.Legal?.length ? snap.Seat : 0;
    if (!snap.Legal?.length) {
      // refresh from other seat perspective not needed; describe again
      snap = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, a.Token);
      if (!snap.Legal?.length) {
        // wait for timeout path shouldn't be needed for dice with 2 ready players
        const other = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, b.Token);
        if (other.Legal?.length) {
          clientSeqB += 1;
          await call(
            "play",
            "SubmitAction",
            {
              RoomId: room.RoomId,
              PlayId: other.PlayId,
              Seat: other.Seat,
              ClientSeq: clientSeqB,
              Action: { Type: "Roll" },
            },
            b.Token,
          );
          snap = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, a.Token);
          continue;
        }
        throw new Error(`no legal actions; phase=${snap.Phase}`);
      }
    }
    if (actor === 1 || snap.Seat === 1) {
      const mine = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, a.Token);
      if (mine.Legal?.some((x) => x.Type === "Roll")) {
        clientSeqA += 1;
        await call(
          "play",
          "SubmitAction",
          {
            RoomId: room.RoomId,
            PlayId: mine.PlayId,
            Seat: mine.Seat,
            ClientSeq: clientSeqA,
            Action: { Type: "Roll" },
          },
          a.Token,
        );
      }
    }
    if (true) {
      const mine = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, b.Token);
      if (mine.Legal?.some((x) => x.Type === "Roll")) {
        clientSeqB += 1;
        await call(
          "play",
          "SubmitAction",
          {
            RoomId: room.RoomId,
            PlayId: mine.PlayId,
            Seat: mine.Seat,
            ClientSeq: clientSeqB,
            Action: { Type: "Roll" },
          },
          b.Token,
        );
      }
    }
    snap = await call("play", "DescribeSnapshot", { PlayId: snap.PlayId, RoomId: room.RoomId }, a.Token);
  }

  if (!snap.Settled) throw new Error("dice play did not settle");
  const records = await call("record", "ListRecord", { Page: 1, PageSize: 5 }, a.Token);
  console.log(
    JSON.stringify(
      {
        ok: true,
        roomCode: room.Code,
        playId: snap.PlayId,
        records: records.Items?.length ?? 0,
      },
      null,
      2,
    ),
  );
}

main().catch((err) => {
  console.error("SMOKE FAILED", err.message, err.requestId || "");
  process.exit(1);
});
