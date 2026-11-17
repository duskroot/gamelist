CREATE TABLE IF NOT EXISTS users (
  user_id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL UNIQUE,
  nickname TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rooms (
  room_id TEXT PRIMARY KEY,
  room_code TEXT NOT NULL UNIQUE,
  mode TEXT NOT NULL,
  max_seats INT NOT NULL,
  host_user_id TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS room_seats (
  room_id TEXT NOT NULL,
  seat_no INT NOT NULL,
  user_id TEXT NOT NULL,
  ready BOOLEAN NOT NULL DEFAULT false,
  PRIMARY KEY (room_id, seat_no),
  UNIQUE (room_id, user_id)
);
CREATE TABLE IF NOT EXISTS matches (
  match_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  max_seats INT NOT NULL,
  status TEXT NOT NULL,
  room_id TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS dice_rounds (
  round_id TEXT PRIMARY KEY,
  room_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  turn_seat INT NOT NULL DEFAULT 0,
  winner_seat INT NOT NULL DEFAULT -1,
  outcome TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS dice_hands (
  round_id TEXT NOT NULL,
  seat_no INT NOT NULL,
  user_id TEXT NOT NULL,
  die_1 INT NOT NULL,
  die_2 INT NOT NULL,
  die_3 INT NOT NULL,
  die_4 INT NOT NULL,
  die_5 INT NOT NULL,
  PRIMARY KEY (round_id, seat_no)
);
CREATE TABLE IF NOT EXISTS dice_calls (
  round_id TEXT NOT NULL,
  seq INT NOT NULL,
  seat_no INT NOT NULL,
  count INT NOT NULL,
  face INT NOT NULL,
  PRIMARY KEY (round_id, seq)
);
