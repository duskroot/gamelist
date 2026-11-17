package dice

import (
	"context"
	"crypto/rand"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBadInput = errors.New("bad input")
	ErrNotFound = errors.New("round not found")
	ErrNotIn    = errors.New("not in room")
	ErrNotTurn  = errors.New("not your turn")
	ErrIllegal  = errors.New("illegal action")
)

type Roller func() [5]int

func CryptoRoller() [5]int {
	var hand [5]int
	buf := make([]byte, 5)
	_, _ = rand.Read(buf)
	for i, b := range buf {
		hand[i] = int(b)%6 + 1
	}
	return hand
}

type HandView struct {
	SeatNo int   `json:"SeatNo"`
	Dice   []int `json:"Dice"`
}

type RoundView struct {
	RoundId    string     `json:"RoundId"`
	Mode       string     `json:"Mode"`
	Status     string     `json:"Status"`
	Outcome    string     `json:"Outcome"`
	WinnerSeat int        `json:"WinnerSeat"`
	YourSeat   int        `json:"YourSeat"`
	YourDice   []int      `json:"YourDice"`
	Revealed   bool       `json:"Revealed"`
	Hands      []HandView `json:"Hands"`
}

type RollResult struct {
	RoomID     string
	RoundID    string
	SeatNo     int
	UserID     string
	Dice       []int
	Resolved   bool
	Outcome    string
	WinnerSeat int
}

type CallResult struct {
	RoomID  string
	RoundID string
	SeatNo  int
	Count   int
	Face    int
}

type OpenResult struct {
	RoomID     string
	RoundID    string
	Hands      []HandView
	Outcome    string
	WinnerSeat int
}

type Service struct {
	Pool   *pgxpool.Pool
	Roller Roller
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func (s *Service) rollHand() [5]int {
	if s.Roller == nil {
		return CryptoRoller()
	}
	return s.Roller()
}

func (s *Service) Roll(ctx context.Context, userID, roundID string) (RollResult, error) {
	if userID == "" || roundID == "" {
		return RollResult{}, ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return RollResult{}, err
	}
	defer tx.Rollback(ctx)

	roomID, mode, status, _, err := lockRound(ctx, tx, roundID)
	if err != nil {
		return RollResult{}, err
	}
	seatNo, err := seatOf(ctx, tx, roomID, userID)
	if err != nil {
		return RollResult{}, err
	}
	if status != "Rolling" {
		return RollResult{}, ErrIllegal
	}
	hand := s.rollHand()
	_, err = tx.Exec(ctx, `INSERT INTO dice_hands(round_id, seat_no, user_id, die_1, die_2, die_3, die_4, die_5) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		roundID, seatNo, userID, hand[0], hand[1], hand[2], hand[3], hand[4])
	if uniqueViolation(err) {
		return RollResult{}, ErrIllegal
	}
	if err != nil {
		return RollResult{}, err
	}

	seats, err := loadSeats(ctx, tx, roomID)
	if err != nil {
		return RollResult{}, err
	}
	hands, err := loadHands(ctx, tx, roundID)
	if err != nil {
		return RollResult{}, err
	}
	res := RollResult{
		RoomID:     roomID,
		RoundID:    roundID,
		SeatNo:     seatNo,
		UserID:     userID,
		Dice:       ints(hand),
		WinnerSeat: -1,
	}
	ordered, complete := orderedHands(seats, hands)
	if complete {
		switch mode {
		case "HighRoll":
			winner, draw := CompareHighRoll(ordered)
			outcome := "Win"
			winnerSeat := -1
			if draw {
				outcome = "Draw"
			} else if winner >= 0 && winner < len(seats) {
				winnerSeat = seats[winner]
			}
			if err := resolve(ctx, tx, roundID, roomID, outcome, winnerSeat); err != nil {
				return RollResult{}, err
			}
			res.Resolved = true
			res.Outcome = outcome
			res.WinnerSeat = winnerSeat
		case "LiarsDice":
			if _, err := tx.Exec(ctx, `UPDATE dice_rounds SET status='Calling' WHERE round_id=$1`, roundID); err != nil {
				return RollResult{}, err
			}
		default:
			return RollResult{}, ErrIllegal
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return RollResult{}, err
	}
	return res, nil
}

func (s *Service) Call(ctx context.Context, userID, roundID string, count, face int) (CallResult, error) {
	if userID == "" || roundID == "" {
		return CallResult{}, ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return CallResult{}, err
	}
	defer tx.Rollback(ctx)

	roomID, mode, status, turn, err := lockRound(ctx, tx, roundID)
	if err != nil {
		return CallResult{}, err
	}
	seatNo, err := seatOf(ctx, tx, roomID, userID)
	if err != nil {
		return CallResult{}, err
	}
	if mode != "LiarsDice" || status != "Calling" {
		return CallResult{}, ErrIllegal
	}
	if seatNo != turn {
		return CallResult{}, ErrNotTurn
	}
	prevCount, prevFace, seq, hasPrev, err := latestCall(ctx, tx, roundID)
	if err != nil {
		return CallResult{}, err
	}
	if !CallHigher(hasPrev, prevCount, prevFace, count, face) {
		return CallResult{}, ErrIllegal
	}
	seats, err := loadSeats(ctx, tx, roomID)
	if err != nil {
		return CallResult{}, err
	}
	if len(seats) == 0 {
		return CallResult{}, ErrIllegal
	}
	if _, err := tx.Exec(ctx, `INSERT INTO dice_calls(round_id, seq, seat_no, count, face) VALUES ($1,$2,$3,$4,$5)`, roundID, seq+1, seatNo, count, face); err != nil {
		return CallResult{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE dice_rounds SET turn_seat=$2 WHERE round_id=$1`, roundID, nextOccupied(seats, seatNo)); err != nil {
		return CallResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CallResult{}, err
	}
	return CallResult{RoomID: roomID, RoundID: roundID, SeatNo: seatNo, Count: count, Face: face}, nil
}

func (s *Service) Open(ctx context.Context, userID, roundID string) (OpenResult, error) {
	if userID == "" || roundID == "" {
		return OpenResult{}, ErrBadInput
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return OpenResult{}, err
	}
	defer tx.Rollback(ctx)

	roomID, mode, status, turn, err := lockRound(ctx, tx, roundID)
	if err != nil {
		return OpenResult{}, err
	}
	seatNo, err := seatOf(ctx, tx, roomID, userID)
	if err != nil {
		return OpenResult{}, err
	}
	if mode != "LiarsDice" || status != "Calling" {
		return OpenResult{}, ErrIllegal
	}
	if seatNo != turn {
		return OpenResult{}, ErrNotTurn
	}
	bidSeat, count, face, seq, hasPrev, err := latestBid(ctx, tx, roundID)
	if err != nil {
		return OpenResult{}, err
	}
	if !hasPrev || seq < 1 {
		return OpenResult{}, ErrIllegal
	}
	hands, err := loadHands(ctx, tx, roundID)
	if err != nil {
		return OpenResult{}, err
	}
	ordered := everyHand(hands)
	winnerSeat := seatNo
	if CountFace(ordered, face) >= count {
		winnerSeat = bidSeat
	}
	if err := resolve(ctx, tx, roundID, roomID, "Win", winnerSeat); err != nil {
		return OpenResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OpenResult{}, err
	}
	return OpenResult{
		RoomID:     roomID,
		RoundID:    roundID,
		Hands:      handViews(handSeats(hands), hands),
		Outcome:    "Win",
		WinnerSeat: winnerSeat,
	}, nil
}

func (s *Service) Describe(ctx context.Context, userID, roundID string) (RoundView, error) {
	view := RoundView{YourDice: []int{}, Hands: []HandView{}, WinnerSeat: -1}
	if userID == "" || roundID == "" {
		return RoundView{}, ErrBadInput
	}
	var roomID string
	err := s.Pool.QueryRow(ctx, `SELECT round_id, room_id, mode, status, outcome, winner_seat FROM dice_rounds WHERE round_id=$1`, roundID).Scan(
		&view.RoundId, &roomID, &view.Mode, &view.Status, &view.Outcome, &view.WinnerSeat,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RoundView{}, ErrNotFound
	}
	if err != nil {
		return RoundView{}, err
	}
	view.YourSeat, err = seatOf(ctx, s.Pool, roomID, userID)
	if err != nil {
		return RoundView{}, err
	}
	hands, err := loadHands(ctx, s.Pool, roundID)
	if err != nil {
		return RoundView{}, err
	}
	if hand, ok := hands[view.YourSeat]; ok {
		view.YourDice = ints(hand)
	}
	view.Revealed = view.Status == "Resolved"
	if view.Revealed {
		seats, err := loadSeats(ctx, s.Pool, roomID)
		if err != nil {
			return RoundView{}, err
		}
		view.Hands = handViews(seats, hands)
	}
	return view, nil
}

func lockRound(ctx context.Context, tx pgx.Tx, roundID string) (roomID, mode, status string, turn int, err error) {
	err = tx.QueryRow(ctx, `SELECT room_id, mode, status, turn_seat FROM dice_rounds WHERE round_id=$1 FOR UPDATE`, roundID).Scan(&roomID, &mode, &status, &turn)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", 0, ErrNotFound
	}
	return roomID, mode, status, turn, err
}

func seatOf(ctx context.Context, q querier, roomID, userID string) (int, error) {
	var seatNo int
	err := q.QueryRow(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1 AND user_id=$2`, roomID, userID).Scan(&seatNo)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotIn
	}
	return seatNo, err
}

func loadSeats(ctx context.Context, q querier, roomID string) ([]int, error) {
	rows, err := q.Query(ctx, `SELECT seat_no FROM room_seats WHERE room_id=$1 ORDER BY seat_no`, roomID)
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

func loadHands(ctx context.Context, q querier, roundID string) (map[int][5]int, error) {
	rows, err := q.Query(ctx, `SELECT seat_no, die_1, die_2, die_3, die_4, die_5 FROM dice_hands WHERE round_id=$1 ORDER BY seat_no`, roundID)
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

func latestCall(ctx context.Context, q querier, roundID string) (count, face, seq int, hasPrev bool, err error) {
	err = q.QueryRow(ctx, `SELECT count, face, seq FROM dice_calls WHERE round_id=$1 ORDER BY seq DESC LIMIT 1`, roundID).Scan(&count, &face, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, false, err
	}
	return count, face, seq, true, nil
}

func latestBid(ctx context.Context, q querier, roundID string) (seat, count, face, seq int, hasPrev bool, err error) {
	err = q.QueryRow(ctx, `SELECT seat_no, count, face, seq FROM dice_calls WHERE round_id=$1 ORDER BY seq DESC LIMIT 1`, roundID).Scan(&seat, &count, &face, &seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, 0, false, err
	}
	return seat, count, face, seq, true, nil
}

func everyHand(hands map[int][5]int) [][5]int {
	ordered, _ := orderedHands(handSeats(hands), hands)
	return ordered
}

func handSeats(hands map[int][5]int) []int {
	seats := make([]int, 0, len(hands))
	for seat := range hands {
		seats = append(seats, seat)
	}
	sort.Ints(seats)
	return seats
}

func orderedHands(seats []int, hands map[int][5]int) ([][5]int, bool) {
	ordered := make([][5]int, 0, len(seats))
	for _, seat := range seats {
		hand, ok := hands[seat]
		if !ok {
			return nil, false
		}
		ordered = append(ordered, hand)
	}
	return ordered, len(seats) > 0
}

func handViews(seats []int, hands map[int][5]int) []HandView {
	out := []HandView{}
	for _, seat := range seats {
		hand, ok := hands[seat]
		if !ok {
			continue
		}
		out = append(out, HandView{SeatNo: seat, Dice: ints(hand)})
	}
	return out
}

func resolve(ctx context.Context, tx pgx.Tx, roundID, roomID, outcome string, winnerSeat int) error {
	if _, err := tx.Exec(ctx, `UPDATE dice_rounds SET status='Resolved', outcome=$2, winner_seat=$3 WHERE round_id=$1`, roundID, outcome, winnerSeat); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE rooms SET status='Open' WHERE room_id=$1`, roomID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE room_seats SET ready=false WHERE room_id=$1`, roomID)
	return err
}

func ints(hand [5]int) []int {
	out := make([]int, len(hand))
	copy(out, hand[:])
	return out
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
