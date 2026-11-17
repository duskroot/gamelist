package room

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"den/internal/dice"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("room not found")
	ErrFull      = errors.New("room full")
	ErrNotIn     = errors.New("not in room")
	ErrNotHost   = errors.New("not host")
	ErrNotReady  = errors.New("not ready")
	ErrIllegal   = errors.New("illegal action")
	ErrBadInput  = errors.New("bad input")
	errLockRetry = errors.New("retry leave lock")
)

type SeatView struct {
	SeatNo   int    `json:"SeatNo"`
	UserId   string `json:"UserId"`
	Nickname string `json:"Nickname"`
	Ready    bool   `json:"Ready"`
}

type RoomView struct {
	RoomId     string     `json:"RoomId"`
	RoomCode   string     `json:"RoomCode"`
	Mode       string     `json:"Mode"`
	MaxSeats   int        `json:"MaxSeats"`
	HostUserId string     `json:"HostUserId"`
	Status     string     `json:"Status"`
	RoundId    string     `json:"RoundId"`
	Seats      []SeatView `json:"Seats"`
}

type Store struct {
	Pool *pgxpool.Pool
}

func (s *Store) Create(ctx context.Context, userID, mode string, maxSeats int) (string, string, error) {
	if mode != "HighRoll" && mode != "LiarsDice" {
		return "", "", ErrBadInput
	}
	if maxSeats == 0 {
		maxSeats = 4
	}
	if maxSeats < 2 || maxSeats > 6 {
		return "", "", ErrBadInput
	}
	roomID := uuid.NewString()
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		code, err := newRoomCode()
		if err != nil {
			return "", "", err
		}
		err = s.insertRoom(ctx, roomID, code, mode, maxSeats, userID)
		if err == nil {
			return roomID, code, nil
		}
		if !uniqueViolation(err) {
			return "", "", err
		}
		last = err
	}
	if last == nil {
		last = errors.New("room code conflict")
	}
	return "", "", last
}

func (s *Store) insertRoom(ctx context.Context, roomID, code, mode string, maxSeats int, userID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO rooms(room_id, room_code, mode, max_seats, host_user_id, status) VALUES ($1,$2,$3,$4,$5,'Open')`, roomID, code, mode, maxSeats, userID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO room_seats(room_id, seat_no, user_id, ready) VALUES ($1,0,$2,false)`, roomID, userID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) Join(ctx context.Context, userID, roomCode string) (string, int, error) {
	if userID == "" || roomCode == "" {
		return "", 0, ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback(ctx)
	var roomID, status string
	var maxSeats int
	err = tx.QueryRow(ctx, `SELECT room_id, status, max_seats FROM rooms WHERE room_code=$1 FOR UPDATE`, roomCode).Scan(&roomID, &status, &maxSeats)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, ErrNotFound
	}
	if err != nil {
		return "", 0, err
	}
	if status == "Closed" {
		return "", 0, ErrNotFound
	}
	if status != "Open" {
		return "", 0, ErrIllegal
	}
	var seatNo int
	err = tx.QueryRow(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1 AND user_id=$2`, roomID, userID).Scan(&seatNo)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return "", 0, err
		}
		return roomID, seatNo, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", 0, err
	}
	rows, err := tx.Query(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1`, roomID)
	if err != nil {
		return "", 0, err
	}
	taken := make([]bool, maxSeats)
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return "", 0, err
		}
		if n >= 0 && n < len(taken) {
			taken[n] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", 0, err
	}
	rows.Close()
	seatNo = -1
	for i, used := range taken {
		if !used {
			seatNo = i
			break
		}
	}
	if seatNo < 0 {
		return "", 0, ErrFull
	}
	if _, err := tx.Exec(ctx, `INSERT INTO room_seats(room_id, seat_no, user_id, ready) VALUES ($1,$2,$3,false)`, roomID, seatNo, userID); err != nil {
		return "", 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", 0, err
	}
	return roomID, seatNo, nil
}

func (s *Store) Leave(ctx context.Context, userID, roomID string) error {
	if userID == "" || roomID == "" {
		return ErrBadInput
	}
	var last error
	for attempt := 0; attempt < 8; attempt++ {
		err := s.leaveOnce(ctx, userID, roomID)
		if !errors.Is(err, errLockRetry) {
			return err
		}
		last = err
	}
	return last
}

func (s *Store) leaveOnce(ctx context.Context, userID, roomID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	roundID, mode, status, turnSeat, hasRound, err := lockOpenRound(ctx, tx, roomID)
	if err != nil {
		return err
	}
	var host string
	err = tx.QueryRow(ctx, `SELECT host_user_id FROM rooms WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&host)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	currentID, stillOpen, err := openRoundID(ctx, tx, roomID)
	if err != nil {
		return err
	}
	if hasRound != stillOpen || (hasRound && currentID != roundID) {
		return errLockRetry
	}
	tag, err := tx.Exec(ctx, `DELETE FROM room_seats WHERE room_id=$1 AND user_id=$2`, roomID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotIn
	}
	var remaining int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM room_seats WHERE room_id=$1`, roomID).Scan(&remaining); err != nil {
		return err
	}
	if hasRound {
		if err := repairRound(ctx, tx, roomID, roundID, mode, status, turnSeat, remaining); err != nil {
			return err
		}
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, `UPDATE rooms SET status='Closed' WHERE room_id=$1`, roomID); err != nil {
			return err
		}
	} else if host == userID {
		var nextHost string
		if err := tx.QueryRow(ctx, `SELECT user_id FROM room_seats WHERE room_id=$1 ORDER BY seat_no LIMIT 1`, roomID).Scan(&nextHost); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE rooms SET host_user_id=$1 WHERE room_id=$2`, nextHost, roomID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func lockOpenRound(ctx context.Context, tx pgx.Tx, roomID string) (roundID, mode, status string, turnSeat int, ok bool, err error) {
	err = tx.QueryRow(ctx, `SELECT round_id, mode, status, turn_seat FROM dice_rounds WHERE room_id=$1 AND status IN ('Rolling','Calling') ORDER BY created_at DESC, round_id DESC LIMIT 1 FOR UPDATE`, roomID).Scan(&roundID, &mode, &status, &turnSeat)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", 0, false, nil
	}
	if err != nil {
		return "", "", "", 0, false, err
	}
	return roundID, mode, status, turnSeat, true, nil
}

func openRoundID(ctx context.Context, tx pgx.Tx, roomID string) (string, bool, error) {
	var roundID string
	err := tx.QueryRow(ctx, `SELECT round_id FROM dice_rounds WHERE room_id=$1 AND status IN ('Rolling','Calling') ORDER BY created_at DESC, round_id DESC LIMIT 1`, roomID).Scan(&roundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return roundID, true, nil
}

func repairRound(ctx context.Context, tx pgx.Tx, roomID, roundID, mode, status string, turnSeat, remaining int) error {
	if remaining == 0 {
		_, err := tx.Exec(ctx, `UPDATE dice_rounds SET status='Resolved', outcome='Draw', winner_seat=-1 WHERE round_id=$1`, roundID)
		return err
	}
	if remaining == 1 {
		var seat int
		if err := tx.QueryRow(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1`, roomID).Scan(&seat); err != nil {
			return err
		}
		return finishRound(ctx, tx, roundID, roomID, "Win", seat)
	}
	seats, err := seatNos(ctx, tx, roomID)
	if err != nil {
		return err
	}
	if status == "Rolling" {
		hands, err := handsBySeat(ctx, tx, roundID)
		if err != nil {
			return err
		}
		ordered := make([][5]int, 0, len(seats))
		for _, seat := range seats {
			hand, ok := hands[seat]
			if !ok {
				return nil
			}
			ordered = append(ordered, hand)
		}
		if mode == "LiarsDice" {
			_, err = tx.Exec(ctx, `UPDATE dice_rounds SET status='Calling', turn_seat=$2 WHERE round_id=$1`, roundID, seats[0])
			return err
		}
		winner, draw := dice.CompareHighRoll(ordered)
		outcome := "Win"
		winnerSeat := -1
		if draw {
			outcome = "Draw"
		} else if winner >= 0 && winner < len(seats) {
			winnerSeat = seats[winner]
		}
		return finishRound(ctx, tx, roundID, roomID, outcome, winnerSeat)
	}
	if status == "Calling" {
		for _, seat := range seats {
			if seat == turnSeat {
				return nil
			}
		}
		_, err = tx.Exec(ctx, `UPDATE dice_rounds SET turn_seat=$2 WHERE round_id=$1`, roundID, nextSeat(seats, turnSeat))
		return err
	}
	return nil
}

func finishRound(ctx context.Context, tx pgx.Tx, roundID, roomID, outcome string, winnerSeat int) error {
	if _, err := tx.Exec(ctx, `UPDATE dice_rounds SET status='Resolved', outcome=$2, winner_seat=$3 WHERE round_id=$1`, roundID, outcome, winnerSeat); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET status='Open' WHERE room_id=$1`, roomID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE room_seats SET ready=false WHERE room_id=$1`, roomID)
	return err
}

func seatNos(ctx context.Context, tx pgx.Tx, roomID string) ([]int, error) {
	rows, err := tx.Query(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1 ORDER BY seat_no`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seats := []int{}
	for rows.Next() {
		var seat int
		if err := rows.Scan(&seat); err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}
	return seats, rows.Err()
}

func handsBySeat(ctx context.Context, tx pgx.Tx, roundID string) (map[int][5]int, error) {
	rows, err := tx.Query(ctx, `SELECT seat_no, die_1, die_2, die_3, die_4, die_5 FROM dice_hands WHERE round_id=$1 ORDER BY seat_no`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hands := map[int][5]int{}
	for rows.Next() {
		var seat int
		var hand [5]int
		if err := rows.Scan(&seat, &hand[0], &hand[1], &hand[2], &hand[3], &hand[4]); err != nil {
			return nil, err
		}
		hands[seat] = hand
	}
	return hands, rows.Err()
}

func nextSeat(seats []int, current int) int {
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

func (s *Store) Ready(ctx context.Context, userID, roomID string, ready bool) error {
	if userID == "" || roomID == "" {
		return ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM rooms WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "Open" {
		return ErrIllegal
	}
	tag, err := tx.Exec(ctx, `UPDATE room_seats SET ready=$1 WHERE room_id=$2 AND user_id=$3`, ready, roomID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotIn
	}
	return tx.Commit(ctx)
}

func (s *Store) Describe(ctx context.Context, roomID string) (RoomView, error) {
	var view RoomView
	view.Seats = []SeatView{}
	err := s.Pool.QueryRow(ctx, `SELECT room_id, room_code, mode, max_seats, host_user_id, status FROM rooms WHERE room_id=$1`, roomID).Scan(
		&view.RoomId, &view.RoomCode, &view.Mode, &view.MaxSeats, &view.HostUserId, &view.Status,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoomView{}, ErrNotFound
	}
	if err != nil {
		return RoomView{}, err
	}
	err = s.Pool.QueryRow(ctx, `SELECT round_id FROM dice_rounds WHERE room_id=$1 ORDER BY created_at DESC, round_id DESC LIMIT 1`, roomID).Scan(&view.RoundId)
	if errors.Is(err, pgx.ErrNoRows) {
		view.RoundId = ""
	} else if err != nil {
		return RoomView{}, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT s.seat_no, s.user_id, u.nickname, s.ready FROM room_seats s JOIN users u ON u.user_id = s.user_id WHERE s.room_id=$1 ORDER BY s.seat_no`, roomID)
	if err != nil {
		return RoomView{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var seat SeatView
		if err := rows.Scan(&seat.SeatNo, &seat.UserId, &seat.Nickname, &seat.Ready); err != nil {
			return RoomView{}, err
		}
		view.Seats = append(view.Seats, seat)
	}
	if err := rows.Err(); err != nil {
		return RoomView{}, err
	}
	return view, nil
}

func (s *Store) Start(ctx context.Context, userID, roomID string) (string, error) {
	if userID == "" || roomID == "" {
		return "", ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var host, status, mode string
	err = tx.QueryRow(ctx, `SELECT host_user_id, status, mode FROM rooms WHERE room_id=$1 FOR UPDATE`, roomID).Scan(&host, &status, &mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if host != userID {
		return "", ErrNotHost
	}
	if status != "Open" {
		return "", ErrIllegal
	}
	var seated, ready int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE ready) FROM room_seats WHERE room_id=$1`, roomID).Scan(&seated, &ready); err != nil {
		return "", err
	}
	if seated < 2 || ready != seated {
		return "", ErrNotReady
	}
	var turnSeat int
	if err := tx.QueryRow(ctx, `SELECT MIN(seat_no) FROM room_seats WHERE room_id=$1`, roomID).Scan(&turnSeat); err != nil {
		return "", err
	}
	roundID := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO dice_rounds(round_id, room_id, mode, status, turn_seat, winner_seat, outcome) VALUES ($1,$2,$3,'Rolling',$4,-1,'')`, roundID, roomID, mode, turnSeat); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET status='Playing' WHERE room_id=$1`, roomID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return roundID, nil
}

func newRoomCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
