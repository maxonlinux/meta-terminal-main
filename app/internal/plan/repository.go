package plan

import (
	"database/sql"
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/math"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
	"github.com/robaho/fixed"
)

// Repository stores user plan data.
type Repository struct {
	db *sql.DB
}

// UserPlanRecord stores plan metadata for a user.
type UserPlanRecord struct {
	Plan      string
	IsManual  bool
	CreatedAt uint64
	UpdatedAt uint64
}

// NewRepository creates a plan repository and ensures schema.
func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("plan repository requires db")
	}
	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

// GetUserPlan returns the plan record for a user.
func (r *Repository) GetUserPlan(userID types.UserID) (*UserPlanRecord, error) {
	row := r.db.QueryRow(
		"select plan, is_manual, created_at, updated_at from user_plans where user_id = ?",
		uint64(userID),
	)
	var plan string
	var isManual int
	var createdAt uint64
	var updatedAt uint64
	if err := row.Scan(&plan, &isManual, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &UserPlanRecord{
		Plan:      plan,
		IsManual:  isManual == 1,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

// UpsertUserPlan saves or updates a user plan.
func (r *Repository) UpsertUserPlan(userID types.UserID, plan string, isManual bool) error {
	now := utils.NowNano()
	_, err := r.db.Exec(
		`insert into user_plans (user_id, plan, is_manual, created_at, updated_at)
        values (?, ?, ?, ?, ?)
        on conflict(user_id)
        do update set plan = excluded.plan, is_manual = excluded.is_manual, updated_at = excluded.updated_at`,
		uint64(userID),
		plan,
		boolToInt(isManual),
		now,
		now,
	)
	return err
}

// ResetManualPlan clears manual override for a user.
func (r *Repository) ResetManualPlan(userID types.UserID) error {
	now := utils.NowNano()
	res, err := r.db.Exec(
		"update user_plans set is_manual = 0, updated_at = ? where user_id = ?",
		now,
		uint64(userID),
	)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("user plan not found")
	}
	return nil
}

// NetDeposits calculates net deposits for a user.

func (r *Repository) NetDeposits(userID types.UserID) (types.Quantity, error) {
	// Sum deposits/withdrawals in fixed-point to avoid float rounding.
	rows, err := r.db.Query(
		`select type, amount, asset from fundings where user_id = ? and status = ?`,
		uint64(userID),
		string(types.FundingStatusCompleted),
	)
	if err != nil {
		return types.Quantity{}, err
	}
	defer func() {
		_ = rows.Close()
	}()

	deposits := types.Quantity(math.Zero)
	withdrawals := types.Quantity(math.Zero)
	for rows.Next() {
		var fundingType string
		var amountRaw string
		var asset string
		if err := rows.Scan(&fundingType, &amountRaw, &asset); err != nil {
			return types.Quantity{}, err
		}
		if !isStableAsset(asset) {
			return types.Quantity{}, fmt.Errorf("unsupported funding asset: %s", asset)
		}
		amount, err := parseFundingAmount(amountRaw)
		if err != nil {
			return types.Quantity{}, err
		}
		if fundingType == string(types.FundingTypeDeposit) {
			deposits = types.Quantity(math.Add(deposits, amount))
			continue
		}
		if fundingType == string(types.FundingTypeWithdrawal) {
			withdrawals = types.Quantity(math.Add(withdrawals, amount))
		}
	}
	if err := rows.Err(); err != nil {
		return types.Quantity{}, err
	}
	return types.Quantity(math.Sub(deposits, withdrawals)), nil
}

// isStableAsset determines whether an asset is treated as 1:1 with USDT.
func isStableAsset(asset string) bool {
	switch asset {
	case "USDT", "USDC", "BUSD", "DAI":
		return true
	default:
		return false
	}
}

func parseFundingAmount(value string) (types.Quantity, error) {
	// Parse stored decimal strings into fixed-point quantities.
	if value == "" {
		return types.Quantity(math.Zero), nil
	}
	parsed, err := fixed.Parse(value)
	if err != nil {
		return types.Quantity{}, err
	}
	return types.Quantity(parsed), nil
}

// ensureSchema creates user plan tables and indexes.
func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
    create table if not exists user_plans (
      user_id integer primary key,
      plan text not null,
      is_manual integer not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists user_plans_user_idx on user_plans (user_id);
  `)
	return err
}

// boolToInt converts a boolean to 0/1 for sqlite.
func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
