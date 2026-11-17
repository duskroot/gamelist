package match

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"den/internal/room"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	ErrAlready  = errors.New("already matching")
	ErrClosed   = errors.New("match closed")
	ErrNotFound = errors.New("match not found")
	ErrBadInput = errors.New("bad input")
)

type View struct {
	MatchId  string `json:"MatchId"`
	Status   string `json:"Status"`
	RoomId   string `json:"RoomId"`
	RoomCode string `json:"RoomCode"`
}

type Service struct {
	mu    sync.Mutex
	Pool  *pgxpool.Pool
	Redis *redis.Client
	Rooms *room.Store
}

func (s *Service) Start(ctx context.Context, userID, mode string, maxSeats int) (string, string, error) {
	if userID == "" || !validMode(mode) || maxSeats < 2 || maxSeats > 6 {
		return "", "", ErrBadInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureNotWaiting(ctx, userID); err != nil {
		return "", "", err
	}
	matchID := uuid.NewString()
	_, err := s.Pool.Exec(ctx, `INSERT INTO matches(match_id, user_id, mode, max_seats, status) VALUES ($1,$2,$3,$4,'Waiting')`, matchID, userID, mode, maxSeats)
	if err != nil {
		return "", "", err
	}
	if err := s.Redis.Set(ctx, userKey(userID), matchID, 0).Err(); err != nil {
		return "", "", err
	}
	key := queueKey(mode, maxSeats)
	if err := s.Redis.RPush(ctx, key, userID).Err(); err != nil {
		return "", "", err
	}
	n, err := s.Redis.LLen(ctx, key).Result()
	if err != nil {
		return "", "", err
	}
	if n >= int64(maxSeats) {
		if err := s.matchFilled(ctx, key, mode, maxSeats); err != nil {
			return "", "", err
		}
	}
	return matchID, "Waiting", nil
}

func (s *Service) Describe(ctx context.Context, matchID string) (View, error) {
	if matchID == "" {
		return View{}, ErrNotFound
	}
	view := View{RoomId: "", RoomCode: ""}
	err := s.Pool.QueryRow(ctx, `SELECT match_id, status, room_id FROM matches WHERE match_id=$1`, matchID).Scan(&view.MatchId, &view.Status, &view.RoomId)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, ErrNotFound
	}
	if err != nil {
		return View{}, err
	}
	if view.RoomId == "" {
		return view, nil
	}
	roomView, err := s.Rooms.Describe(ctx, view.RoomId)
	if err != nil {
		return View{}, err
	}
	view.RoomCode = roomView.RoomCode
	return view, nil
}

func (s *Service) Cancel(ctx context.Context, userID, matchID string) error {
	if userID == "" || matchID == "" {
		return ErrBadInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var mode, status string
	var maxSeats int
	err := s.Pool.QueryRow(ctx, `SELECT mode, max_seats, status FROM matches WHERE match_id=$1 AND user_id=$2`, matchID, userID).Scan(&mode, &maxSeats, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "Waiting" {
		return ErrClosed
	}
	if err := s.Redis.LRem(ctx, queueKey(mode, maxSeats), 0, userID).Err(); err != nil {
		return err
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE matches SET status='Canceled' WHERE match_id=$1 AND status='Waiting'`, matchID); err != nil {
		return err
	}
	if err := s.Redis.Del(ctx, userKey(userID)).Err(); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureNotWaiting(ctx context.Context, userID string) error {
	matchID, err := s.Redis.Get(ctx, userKey(userID)).Result()
	if errors.Is(err, redis.Nil) || matchID == "" {
		return nil
	}
	if err != nil {
		return err
	}
	var status string
	err = s.Pool.QueryRow(ctx, `SELECT status FROM matches WHERE match_id=$1`, matchID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == "Waiting" {
		return ErrAlready
	}
	return nil
}

func (s *Service) matchFilled(ctx context.Context, key, mode string, maxSeats int) error {
	// LRANGE the prefix, then LTRIM from maxSeats so those ids are gone before anyone is seated.
	ids, err := s.Redis.LRange(ctx, key, 0, int64(maxSeats-1)).Result()
	if err != nil {
		return err
	}
	if len(ids) < maxSeats {
		return nil
	}
	if err := s.Redis.LTrim(ctx, key, int64(maxSeats), -1).Err(); err != nil {
		return err
	}
	unique := make([]string, 0, maxSeats)
	seen := make(map[string]struct{}, maxSeats)
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	// A later duplicate would be seated again on the next fill. Remove every copy now.
	for _, id := range unique {
		if err := s.Redis.LRem(ctx, key, 0, id).Err(); err != nil {
			return err
		}
	}
	waiting, err := s.stillWaiting(ctx, unique, mode, maxSeats)
	if err != nil {
		return err
	}
	if len(waiting) < maxSeats {
		for i := len(waiting) - 1; i >= 0; i-- {
			if err := s.Redis.LPush(ctx, key, waiting[i]).Err(); err != nil {
				return err
			}
		}
		return nil
	}
	roomID, roomCode, err := s.Rooms.Create(ctx, waiting[0], mode, maxSeats)
	if err != nil {
		return err
	}
	for _, id := range waiting[1:] {
		if _, _, err := s.Rooms.Join(ctx, id, roomCode); err != nil {
			return err
		}
	}
	for _, id := range waiting {
		tag, err := s.Pool.Exec(ctx, `UPDATE matches SET status='Matched', room_id=$1 WHERE user_id=$2 AND mode=$3 AND max_seats=$4 AND status='Waiting'`, roomID, id, mode, maxSeats)
		if err != nil {
			return err
		}
		if tag.RowsAffected() < 1 {
			return fmt.Errorf("match update for %s affected %d", id, tag.RowsAffected())
		}
		if err := s.Redis.Del(ctx, userKey(id)).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) stillWaiting(ctx context.Context, ids []string, mode string, maxSeats int) ([]string, error) {
	waiting := make([]string, 0, len(ids))
	for _, id := range ids {
		var one int
		err := s.Pool.QueryRow(ctx, `SELECT 1 FROM matches WHERE user_id=$1 AND mode=$2 AND max_seats=$3 AND status='Waiting'`, id, mode, maxSeats).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		waiting = append(waiting, id)
	}
	return waiting, nil
}

func validMode(mode string) bool {
	return mode == "HighRoll" || mode == "LiarsDice"
}

func userKey(userID string) string {
	return "den:match:user:" + userID
}

func queueKey(mode string, maxSeats int) string {
	return fmt.Sprintf("den:match:queue:%s:%d", mode, maxSeats)
}
