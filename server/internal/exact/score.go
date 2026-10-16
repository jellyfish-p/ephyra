package exact

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Score is a non-scientific decimal with at most two fractional digits,
// stored as integer hundredths. Binary floating point is not used.
type Score struct {
	hundredths int64
}

func Zero() Score { return Score{} }

func FromInt(n int64) Score { return Score{hundredths: n * 100} }

func FromHundredths(h int64) Score { return Score{hundredths: h} }

func (s Score) Hundredths() int64 { return s.hundredths }

func (s Score) IsZero() bool { return s.hundredths == 0 }

func (s Score) Cmp(o Score) int {
	switch {
	case s.hundredths < o.hundredths:
		return -1
	case s.hundredths > o.hundredths:
		return 1
	default:
		return 0
	}
}

func (s Score) Add(o Score) Score {
	return Score{hundredths: s.hundredths + o.hundredths}
}

func (s Score) String() string {
	sign := ""
	h := s.hundredths
	if h < 0 {
		sign = "-"
		h = -h
	}
	return fmt.Sprintf("%s%d.%02d", sign, h/100, h%100)
}

func Parse(raw string) (Score, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Score{}, fmt.Errorf("empty score")
	}
	if strings.ContainsAny(s, "eE") {
		return Score{}, fmt.Errorf("scientific notation is not allowed")
	}
	neg := false
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	if s == "" {
		return Score{}, fmt.Errorf("invalid score")
	}
	wholeStr, fracStr, ok := strings.Cut(s, ".")
	if !ok {
		n, err := parseDigits(wholeStr)
		if err != nil {
			return Score{}, err
		}
		if neg {
			n = -n
		}
		return FromInt(n), nil
	}
	if fracStr == "" {
		return Score{}, fmt.Errorf("trailing decimal point is not allowed")
	}
	if len(fracStr) > 2 {
		return Score{}, fmt.Errorf("score has more than 2 decimal places")
	}
	if wholeStr == "" {
		return Score{}, fmt.Errorf("invalid score")
	}
	whole, err := parseDigits(wholeStr)
	if err != nil {
		return Score{}, err
	}
	frac, err := parseFraction(fracStr)
	if err != nil {
		return Score{}, err
	}
	h := whole*100 + frac
	if neg {
		h = -h
	}
	return Score{hundredths: h}, nil
}

func parseDigits(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid number")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid number %q", s)
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("leading zeros are not allowed")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number")
	}
	return n, nil
}

func parseFraction(s string) (int64, error) {
	if len(s) == 0 || len(s) > 2 {
		return 0, fmt.Errorf("invalid fraction")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid fraction")
		}
	}
	if len(s) == 1 {
		s += "0"
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid fraction")
	}
	return n, nil
}

func (s Score) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Score) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("score must be a decimal string")
	}
	parsed, err := Parse(raw)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}
