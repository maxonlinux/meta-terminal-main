package plan

import (
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type Service struct {
	repo     *Repository
	registry *registry.Registry
	plans    []Rule
}

func NewService(repo *Repository, reg *registry.Registry) *Service {
	return &Service{
		repo:     repo,
		registry: reg,
		plans:    defaultRules(),
	}
}

func (s *Service) GetUserPlanProgress(userID types.UserID) (Progress, error) {
	record, err := s.repo.GetUserPlan(userID)
	if err != nil {
		return Progress{}, err
	}
	if record != nil && record.IsManual {
		return Progress{Current: Name(record.Plan)}, nil
	}

	netDeposits, err := s.repo.NetDeposits(userID)
	if err != nil {
		return Progress{}, err
	}

	current, next := s.calculatePlan(netDeposits)
	// Keep remaining in fixed-point to avoid float rounding.
	remaining := types.Quantity(math.Zero)
	if next != nil {
		remaining = types.Quantity(math.Sub(next.Threshold, netDeposits))
		if math.Sign(remaining) < 0 {
			remaining = types.Quantity(math.Zero)
		}
	}

	if current != nil {
		_ = s.repo.UpsertUserPlan(userID, string(current.Name), false)
	}

	return Progress{
		Current:     nameOrEmpty(current),
		Next:        nameOrEmpty(next),
		Remaining:   remaining,
		NetDeposits: netDeposits,
	}, nil
}

func (s *Service) SetManualPlan(userID types.UserID, plan Name) error {
	return s.repo.UpsertUserPlan(userID, string(plan), true)
}

func (s *Service) ResetManualPlan(userID types.UserID) error {
	return s.repo.ResetManualPlan(userID)
}

func (s *Service) CheckOrder(userID types.UserID, category int8, symbol string) error {
	rule, err := s.userRule(userID)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}
	if category == constants.CATEGORY_LINEAR && math.Sign(rule.MaxLeverage) <= 0 {
		return constants.ErrPlanLinearNotAllowed
	}
	if !s.assetTypeAllowed(rule, symbol) {
		return constants.ErrPlanAssetNotAllowed
	}
	return nil
}

func (s *Service) CheckLeverage(userID types.UserID, symbol string, leverage types.Leverage) error {
	rule, err := s.userRule(userID)
	if err != nil {
		return err
	}
	if rule == nil {
		return nil
	}
	if math.Sign(rule.MaxLeverage) <= 0 {
		return constants.ErrPlanLinearNotAllowed
	}
	if !s.assetTypeAllowed(rule, symbol) {
		return constants.ErrPlanAssetNotAllowed
	}
	// Compare leverage using fixed-point values.
	limit := rule.MaxLeverage
	if math.Cmp(leverage, limit) > 0 {
		return constants.ErrPlanLeverageTooHigh
	}
	return nil
}

func (s *Service) userRule(userID types.UserID) (*Rule, error) {
	record, err := s.repo.GetUserPlan(userID)
	if err != nil {
		return nil, err
	}
	if record != nil && record.IsManual {
		return s.ruleFor(Name(record.Plan)), nil
	}

	netDeposits, err := s.repo.NetDeposits(userID)
	if err != nil {
		return nil, err
	}
	current, _ := s.calculatePlan(netDeposits)
	if current == nil {
		return nil, nil
	}
	return current, nil
}

func (s *Service) assetTypeAllowed(rule *Rule, symbol string) bool {
	if rule == nil || len(rule.AssetTypes) == 0 {
		return true
	}
	if s.registry == nil {
		return true
	}
	inst := s.registry.GetInstrument(symbol)
	if inst == nil || inst.AssetType == "" {
		return true
	}
	_, ok := rule.AssetTypes[inst.AssetType]
	return ok
}

func (s *Service) calculatePlan(netDeposits types.Quantity) (*Rule, *Rule) {
	// Single pass keeps the current and next rules without extra scans.
	var current *Rule
	var next *Rule
	for i := range s.plans {
		plan := &s.plans[i]
		if math.Cmp(netDeposits, plan.Threshold) >= 0 {
			current = plan
			continue
		}
		if next == nil {
			next = plan
		}
	}
	return current, next
}

func (s *Service) ruleFor(name Name) *Rule {
	for i := range s.plans {
		if s.plans[i].Name == name {
			return &s.plans[i]
		}
	}
	return nil
}

func nameOrEmpty(rule *Rule) Name {
	if rule == nil {
		return ""
	}
	return rule.Name
}

// defaultRules defines fixed-point thresholds and leverage limits.
func defaultRules() []Rule {
	return []Rule{
		{
			Name:        PlanLowBase,
			Threshold:   types.Quantity(fixed.NewI(10, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(0, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanBase,
			Threshold:   types.Quantity(fixed.NewI(500, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(0, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanStandard,
			Threshold:   types.Quantity(fixed.NewI(1000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(5, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanSilver,
			Threshold:   types.Quantity(fixed.NewI(5000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(10, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanGold,
			Threshold:   types.Quantity(fixed.NewI(10000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(15, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
				"commodity",
				"index",
				"indices",
			),
		},
		{
			Name:        PlanPlatinum,
			Threshold:   types.Quantity(fixed.NewI(25000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(20, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
				"commodity",
				"index",
				"indices",
			),
		},
		{
			Name:        PlanAdvanced,
			Threshold:   types.Quantity(fixed.NewI(50000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(25, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
				"commodity",
				"index",
				"indices",
			),
		},
		{
			Name:        PlanProfessional,
			Threshold:   types.Quantity(fixed.NewI(100000, 0)),
			MaxLeverage: types.Leverage(fixed.NewI(30, 0)),
			AssetTypes: assetSet(
				"crypto",
				"stock",
				"commodity",
				"index",
				"indices",
				"fund",
			),
		},
	}
}

func assetSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}
