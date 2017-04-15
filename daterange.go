package phonelab

import (
	"fmt"
	"strings"

	"github.com/gurupras/go-daterange"
)

func ParseDateRange(str string) (*daterange.DateRange, error) {
	// Format is "YYYYmmDD - YYYYmmDD"
	tokens := strings.Split(str, "-")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("Expected exactly 2 tokens. Got: %v", tokens)
	}

	start := strings.TrimSpace(tokens[0])
	end := strings.TrimSpace(tokens[1])

	dr := daterange.NewDateRange(start, end)
	if dr == nil {
		// XXX: Currently, this only fails if range is negative
		return nil, fmt.Errorf("Start occurs before End: %v", str)
	}
	return dr, nil
}
