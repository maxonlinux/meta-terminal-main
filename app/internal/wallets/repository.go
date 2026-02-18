package wallets

import (
	"database/sql"
	"fmt"

	"github.com/maxonlinux/meta-terminal-go/pkg/snowflake"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/maxonlinux/meta-terminal-go/pkg/utils"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) (*Repository, error) {
	if db == nil {
		return nil, fmt.Errorf("wallet repository requires db")
	}
	if err := ensureSchema(db); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

func (r *Repository) CreateWallet(wallet Wallet) (int64, error) {
	if wallet.Name == "" || wallet.Address == "" || wallet.Network == "" || wallet.Currency == "" {
		return 0, fmt.Errorf("wallet fields are required")
	}
	now := utils.NowNano()
	wallet.ID = snowflake.Next()
	res, err := r.db.Exec(
		`insert into wallets (id, name, address, network, currency, is_custom, is_active, created_at, updated_at)
       values (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		wallet.ID,
		wallet.Name,
		wallet.Address,
		wallet.Network,
		wallet.Currency,
		boolToInt(wallet.IsCustom),
		boolToInt(wallet.IsActive),
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	_ = res
	return wallet.ID, nil
}

func (r *Repository) UpdateWallet(id int64, wallet Wallet) error {
	if id == 0 {
		return fmt.Errorf("invalid wallet id")
	}
	if wallet.Name == "" || wallet.Address == "" || wallet.Network == "" || wallet.Currency == "" {
		return fmt.Errorf("wallet fields are required")
	}
	now := utils.NowNano()
	_, err := r.db.Exec(
		`update wallets set name = ?, address = ?, network = ?, currency = ?, is_custom = ?, is_active = ?, updated_at = ? where id = ?`,
		wallet.Name,
		wallet.Address,
		wallet.Network,
		wallet.Currency,
		boolToInt(wallet.IsCustom),
		boolToInt(wallet.IsActive),
		now,
		id,
	)
	return err
}

func (r *Repository) GetWallet(id int64) (*Wallet, error) {
	if id == 0 {
		return nil, nil
	}
	row := r.db.QueryRow(
		`select id, name, address, network, currency, is_custom, is_active, created_at, updated_at from wallets where id = ?`,
		id,
	)
	var wallet Wallet
	var isCustom int
	var isActive int
	if err := row.Scan(
		&wallet.ID,
		&wallet.Name,
		&wallet.Address,
		&wallet.Network,
		&wallet.Currency,
		&isCustom,
		&isActive,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	wallet.IsCustom = isCustom == 1
	wallet.IsActive = isActive == 1
	return &wallet, nil
}

func (r *Repository) ListWallets() ([]Wallet, error) {
	rows, err := r.db.Query(
		`select id, name, address, network, currency, is_custom, is_active, created_at, updated_at from wallets order by id desc`,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []Wallet
	for rows.Next() {
		var wallet Wallet
		var isCustom int
		var isActive int
		if err := rows.Scan(
			&wallet.ID,
			&wallet.Name,
			&wallet.Address,
			&wallet.Network,
			&wallet.Currency,
			&isCustom,
			&isActive,
			&wallet.CreatedAt,
			&wallet.UpdatedAt,
		); err != nil {
			return nil, err
		}
		wallet.IsCustom = isCustom == 1
		wallet.IsActive = isActive == 1
		out = append(out, wallet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) ListStandardWallets() ([]Wallet, error) {
	rows, err := r.db.Query(
		`select id, name, address, network, currency, is_custom, is_active, created_at, updated_at
       from wallets where is_custom = 0 and is_active = 1`,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []Wallet
	for rows.Next() {
		var wallet Wallet
		var isCustom int
		var isActive int
		if err := rows.Scan(
			&wallet.ID,
			&wallet.Name,
			&wallet.Address,
			&wallet.Network,
			&wallet.Currency,
			&isCustom,
			&isActive,
			&wallet.CreatedAt,
			&wallet.UpdatedAt,
		); err != nil {
			return nil, err
		}
		wallet.IsCustom = isCustom == 1
		wallet.IsActive = isActive == 1
		out = append(out, wallet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) AssignWallet(userID types.UserID, wallet Wallet, assignedBy string) error {
	if userID == 0 {
		return fmt.Errorf("invalid user id")
	}
	if wallet.ID == 0 {
		return fmt.Errorf("invalid wallet")
	}
	now := utils.NowNano()
	_, err := r.db.Exec(
		`insert into user_wallets (user_id, wallet_id, network, currency, assigned_by, assigned_at)
       values (?, ?, ?, ?, ?, ?)
       on conflict(user_id, network, currency)
       do update set wallet_id = excluded.wallet_id, assigned_by = excluded.assigned_by, assigned_at = excluded.assigned_at`,
		uint64(userID),
		wallet.ID,
		wallet.Network,
		wallet.Currency,
		assignedBy,
		now,
	)
	return err
}

func (r *Repository) ListUserWallets(userID types.UserID, activeOnly bool) ([]UserWallet, error) {
	if userID == 0 {
		return nil, nil
	}
	query := `select uw.user_id, uw.wallet_id, w.name, w.address, uw.network, uw.currency, w.is_custom, w.is_active, uw.assigned_at, uw.assigned_by
      from user_wallets uw
      join wallets w on w.id = uw.wallet_id
      where uw.user_id = ?`
	if activeOnly {
		query += " and w.is_active = 1"
	}
	query += " order by uw.assigned_at desc"
	rows, err := r.db.Query(query, uint64(userID))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []UserWallet
	for rows.Next() {
		var item UserWallet
		var isCustom int
		var isActive int
		if err := rows.Scan(
			&item.UserID,
			&item.WalletID,
			&item.Name,
			&item.Address,
			&item.Network,
			&item.Currency,
			&isCustom,
			&isActive,
			&item.AssignedAt,
			&item.AssignedBy,
		); err != nil {
			return nil, err
		}
		item.IsCustom = isCustom == 1
		item.IsActive = isActive == 1
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *Repository) GetUserWallet(userID types.UserID, walletID int64) (*UserWallet, error) {
	if userID == 0 || walletID == 0 {
		return nil, nil
	}
	row := r.db.QueryRow(
		`select uw.user_id, uw.wallet_id, w.name, w.address, uw.network, uw.currency, w.is_custom, w.is_active, uw.assigned_at, uw.assigned_by
      from user_wallets uw
      join wallets w on w.id = uw.wallet_id
      where uw.user_id = ? and uw.wallet_id = ?`,
		uint64(userID),
		walletID,
	)
	var item UserWallet
	var isCustom int
	var isActive int
	if err := row.Scan(
		&item.UserID,
		&item.WalletID,
		&item.Name,
		&item.Address,
		&item.Network,
		&item.Currency,
		&isCustom,
		&isActive,
		&item.AssignedAt,
		&item.AssignedBy,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	item.IsCustom = isCustom == 1
	item.IsActive = isActive == 1
	return &item, nil
}

func (r *Repository) CountWallets() (int, error) {
	row := r.db.QueryRow("select count(1) from wallets")
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`
    create table if not exists wallets (
      id integer primary key,
      name text not null unique,
      address text not null,
      network text not null,
      currency text not null,
      is_custom integer not null,
      is_active integer not null,
      created_at integer not null,
      updated_at integer not null
    );

    create index if not exists wallets_network_idx on wallets (network, currency);

    create table if not exists user_wallets (
      user_id integer not null,
      wallet_id integer not null,
      network text not null,
      currency text not null,
      assigned_by text not null,
      assigned_at integer not null,
      primary key (user_id, network, currency)
    );

    create index if not exists user_wallets_user_idx on user_wallets (user_id);
  `)
	return err
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
