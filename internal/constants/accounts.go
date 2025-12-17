package constants

const (
	MaxSafeBalanceFloat = 9223372036854775.0
)

const (
	MaxNameLen   = 100
	CentsPerUnit = 100
)

const (
	SystemAccountOpeningBalance = "Equity:OpeningBalances"
	TypeEquity                  = "C"
	OpeningAccountMemo          = "Opening Balance"
)

var ReservedNames = map[string]bool{
	"assets":      true,
	"liabilities": true,
	"equity":      true,
	"revenue":     true,
	"expenses":    true,
}
