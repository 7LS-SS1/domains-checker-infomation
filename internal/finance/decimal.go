package finance

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^[+-]?[0-9]+(?:\.[0-9]+)?$`)

// Decimal keeps money and rates as exact rational numbers. JSON and database
// boundaries use strings so binary floating point never enters a calculation.
type Decimal struct {
	value *big.Rat
}

func ParseDecimal(raw string, maxScale int) (Decimal, error) {
	raw = strings.TrimSpace(raw)
	if !decimalPattern.MatchString(raw) {
		return Decimal{}, errors.New("must be a plain decimal string")
	}
	if dot := strings.IndexByte(raw, '.'); dot >= 0 && len(raw)-dot-1 > maxScale {
		return Decimal{}, errors.New("decimal scale exceeds policy")
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok {
		return Decimal{}, errors.New("invalid decimal")
	}
	return Decimal{value: value}, nil
}

func decimalFromInt(value int64) Decimal {
	return Decimal{value: new(big.Rat).SetInt64(value)}
}

func (d Decimal) valid() bool { return d.value != nil }

func (d Decimal) Sign() int {
	if !d.valid() {
		return 0
	}
	return d.value.Sign()
}

func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Add(d.value, other.value)}
}

func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Sub(d.value, other.value)}
}

func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Mul(d.value, other.value)}
}

func (d Decimal) Quo(other Decimal) Decimal {
	return Decimal{value: new(big.Rat).Quo(d.value, other.value)}
}

func (d Decimal) String(scale int) string {
	if !d.valid() {
		return ""
	}
	return d.value.FloatString(scale)
}
