package runtime

type SessionTurn struct {
	Input  string
	Output string
}

type Session struct {
	turns []SessionTurn
}

func NewSession() *Session {
	return &Session{turns: []SessionTurn{}}
}

func (s *Session) AddTurn(turn SessionTurn) {
	s.turns = append(s.turns, turn)
}

func (s *Session) History() []SessionTurn {
	return append([]SessionTurn(nil), s.turns...)
}

func (s *Session) Count() int {
	return len(s.turns)
}

func (s *Session) LastTurn() (SessionTurn, bool) {
	if len(s.turns) == 0 {
		return SessionTurn{}, false
	}
	return s.turns[len(s.turns)-1], true
}
