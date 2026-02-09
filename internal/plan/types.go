package plan

type Name string

const (
	PlanLowBase      Name = "LOW_BASE"
	PlanBase         Name = "BASE"
	PlanStandard     Name = "STANDARD"
	PlanSilver       Name = "SILVER"
	PlanGold         Name = "GOLD"
	PlanPlatinum     Name = "PLATINUM"
	PlanAdvanced     Name = "ADVANCED"
	PlanProfessional Name = "PROFESSIONAL"
)

type Progress struct {
	Current     Name    `json:"current"`
	Next        Name    `json:"next"`
	Remaining   float64 `json:"remaining"`
	NetDeposits float64 `json:"netDeposits"`
}

type Rule struct {
	Name        Name
	Threshold   float64
	MaxLeverage float64
	AssetTypes  map[string]struct{}
}
