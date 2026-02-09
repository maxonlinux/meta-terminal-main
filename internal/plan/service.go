package plan

import (
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
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
	remaining := 0.0
	if next != nil {
		remaining = next.Threshold - netDeposits
		if remaining < 0 {
			remaining = 0
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
	if category == constants.CATEGORY_LINEAR && rule.MaxLeverage <= 0 {
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
	if rule.MaxLeverage <= 0 {
		return constants.ErrPlanLinearNotAllowed
	}
	if !s.assetTypeAllowed(rule, symbol) {
		return constants.ErrPlanAssetNotAllowed
	}
	limit := fixed.NewF(rule.MaxLeverage)
	if leverage.Cmp(limit) > 0 {
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

func (s *Service) calculatePlan(netDeposits float64) (*Rule, *Rule) {
	var current *Rule
	for i := range s.plans {
		plan := &s.plans[i]
		if netDeposits >= plan.Threshold {
			current = plan
		}
	}
	var next *Rule
	for i := range s.plans {
		plan := &s.plans[i]
		if netDeposits < plan.Threshold {
			next = plan
			break
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

func defaultRules() []Rule {
	return []Rule{
		{
			Name:        PlanLowBase,
			Threshold:   10,
			MaxLeverage: 0,
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanBase,
			Threshold:   500,
			MaxLeverage: 0,
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanStandard,
			Threshold:   1000,
			MaxLeverage: 5,
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanSilver,
			Threshold:   5000,
			MaxLeverage: 10,
			AssetTypes: assetSet(
				"crypto",
				"stock",
			),
		},
		{
			Name:        PlanGold,
			Threshold:   10000,
			MaxLeverage: 15,
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
			Threshold:   25000,
			MaxLeverage: 20,
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
			Threshold:   50000,
			MaxLeverage: 25,
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
			Threshold:   100000,
			MaxLeverage: 30,
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

func (s *Service) Debug() string {
	return fmt.Sprintf("plans=%d", len(s.plans))
}
