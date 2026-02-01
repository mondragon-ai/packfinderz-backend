package enums

import "fmt"

// Currency represents supported monetary denominations for cart totals.
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyBTC Currency = "BTC"
	CurrencyETH Currency = "ETH"
)

var validCurrencies = []Currency{
	CurrencyUSD,
	CurrencyBTC,
	CurrencyETH,
}

// String implements fmt.Stringer.
func (c Currency) String() string {
	return string(c)
}

// IsValid reports whether the currency is recognized.
func (c Currency) IsValid() bool {
	for _, candidate := range validCurrencies {
		if candidate == c {
			return true
		}
	}
	return false
}

// ParseCurrency converts a raw string into a Currency.
func ParseCurrency(value string) (Currency, error) {
	for _, candidate := range validCurrencies {
		if string(candidate) == value {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("invalid currency %q", value)
}
