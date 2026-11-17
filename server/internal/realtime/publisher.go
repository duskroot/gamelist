package realtime

type Event struct {
	Service string
	Action  string
	Result  any
}

type Publisher interface {
	EmitRoom(roomID string, ev Event)
	EmitUser(roomID, userID string, ev Event)
}

type Nop struct{}

func (Nop) EmitRoom(string, Event)         {}
func (Nop) EmitUser(string, string, Event) {}
