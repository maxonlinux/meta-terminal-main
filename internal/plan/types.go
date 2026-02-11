package plan

import "github.com/maxonlinux/meta-terminal-go/pkg/types"

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

// Progress stores fixed-point amounts to avoid rounding errors.
type Progress struct {
	Current     Name           `json:"current"`
	Next        Name           `json:"next"`
	Remaining   types.Quantity `json:"remaining"`
	NetDeposits types.Quantity `json:"netDeposits"`
}

// Rule uses fixed-point thresholds and leverage limits for accuracy.
type Rule struct {
	Name        Name
	Threshold   types.Quantity
	MaxLeverage types.Leverage
	AssetTypes  map[string]struct{}
}
