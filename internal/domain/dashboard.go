package domain

type TaskPage struct {
	Tasks []Task `json:"tasks"`
	Next  int64  `json:"next"`
}

type TaskSnapshot struct {
	Task         Task            `json:"task"`
	Agents       []Agent         `json:"agents"`
	Events       []EventEnvelope `json:"events"`
	NextSequence uint64          `json:"next_sequence"`
	More         bool            `json:"more"`
}
