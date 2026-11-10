package model

import (
	"encoding/json"
	"fmt"
)

// NoValNumber models Slurm's set/infinite/number wrapper used for optional
// numeric fields.
type NoValNumber struct {
	Set      bool  `json:"set"`
	Infinite bool  `json:"infinite"`
	Number   int64 `json:"number"`
}

// Value reports the number only when Slurm supplied a finite value.
func (n NoValNumber) Value() (int64, bool) {
	if !n.Set || n.Infinite {
		return 0, false
	}
	return n.Number, true
}

func (n NoValNumber) String() string {
	if n.Infinite {
		return "∞"
	}
	if !n.Set {
		return "—"
	}
	return fmt.Sprintf("%d", n.Number)
}

func (n *NoValNumber) UnmarshalJSON(data []byte) error {
	type wrapped NoValNumber
	var value wrapped
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*n = NoValNumber(value)
	return nil
}
