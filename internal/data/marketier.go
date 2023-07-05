package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type MarketierUserAccount struct {
	BaseUserAccount BaseUserAccount
	DisplayName     string `json:"display_name"`
	About           string `json:"about"`
	SalesGenerated  int    `json:"sales_generated"`
	Tier            int    `json:"tier"`
}

type MarketierAccountModel struct {
	DB *sql.DB
}

func (marketierUserModel MarketierAccountModel) Insert(marketierUser *MarketierUserAccount) error {
	baseQuery := `
        INSERT INTO base_users (first_name, last_name, email, date_of_birth, gender, address, password, account_type) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING user_id, account_creation_time, account_status, version`

	baseArgs := []interface{}{marketierUser.BaseUserAccount.FirstName, marketierUser.BaseUserAccount.LastName, marketierUser.BaseUserAccount.Email, marketierUser.BaseUserAccount.DateOfBirth, marketierUser.BaseUserAccount.Gender, marketierUser.BaseUserAccount.Address, marketierUser.BaseUserAccount.Password.hash, marketierUser.BaseUserAccount.AccountType}

	marketierQuery := `
	INSERT INTO marketiers (user_id, display_name, about, sales_generated, tier) 
	VALUES ($1, $2, $3, $4, $5)`

	marketierArgs := []interface{}{marketierUser.DisplayName, marketierUser.About, marketierUser.SalesGenerated, marketierUser.Tier}

	// Begin a transaction
	tx, err := marketierUserModel.DB.Begin()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Execute the first insertion
	err = tx.QueryRowContext(ctx, baseQuery, baseArgs...).Scan(&marketierUser.BaseUserAccount.UserId, &marketierUser.BaseUserAccount.AccountCreationTime, &marketierUser.BaseUserAccount.AccountStatus, &marketierUser.BaseUserAccount.Version)
	if err != nil {
		tx.Rollback()
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "base_users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	updatedMarketierArgs := append([]interface{}{marketierUser.BaseUserAccount.UserId}, marketierArgs...)

	// Execute the second insertion
	_, err = tx.ExecContext(ctx, marketierQuery, updatedMarketierArgs...)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil

}

func (m MarketierAccountModel) GetByEmail(email string) (*MarketierUserAccount, error) {
	query := `
        SELECT base_user.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type, display_name, about, sales_generated, tier
        FROM base_users INNER JOIN marketier ON base_users.user_id = marketiers.user_id WHERE users.email = $1`

	var marketier MarketierUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&marketier.BaseUserAccount.UserId,
		&marketier.BaseUserAccount.FirstName,
		&marketier.BaseUserAccount.LastName,
		&marketier.BaseUserAccount.Email,
		&marketier.BaseUserAccount.DateOfBirth,
		&marketier.BaseUserAccount.Gender,
		&marketier.BaseUserAccount.Address,
		&marketier.BaseUserAccount.Password.hash,
		&marketier.BaseUserAccount.AccountCreationTime,
		&marketier.BaseUserAccount.LastLoginTime,
		&marketier.BaseUserAccount.AccountStatus,
		&marketier.BaseUserAccount.Version,
		&marketier.BaseUserAccount.AccountType,
		&marketier.DisplayName,
		&marketier.About,
		&marketier.SalesGenerated,
		&marketier.Tier,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &marketier, nil
}

/*
*

	type BaseUserAccount struct {
		UserId              int64     `json:"user_id"`
		FirstName           string    `json:"first_name"`
		LastName            string    `json:"last_name"`
		Email               string    `json:"email"`
		DateOfBirth         time.Time `json:"date_of_birth"`
		Gender              string    `json:"gender"`
		Address             string    `json:"address"`
		Password            password  `json:"-"`
		AccountCreationTime time.Time `json:"account_creation_time"`
		LastLoginTime       time.Time `json:"last_login_time"`
		AccountStatus       string    `json:"account_status"`
		Version             int64     `json:"version"`
		AccountType         int8      `json:"account_type"`
	}



type MarketierUserAccount struct {
	BaseUserAccount BaseUserAccount
	DisplayName     string `json:"display_name"`
	About           string `json:"about"`
	SalesGenerated  int    `json:"sales_generated"`
	Tier            int    `json:"tier"`
}
*
*/

func (marketierUserModel MarketierAccountModel) Update(marketier *MarketierUserAccount) error {
	baseQuery := `
        UPDATE base_users
        SET first_name = $1, last_name = $2, email = $3, address = $4, password = $5, last_login_time = $6, account_status = $7, version = version + 1
        WHERE user_id = $8 AND version = $9
        RETURNING version`

	baseArgs := []interface{}{
		marketier.BaseUserAccount.FirstName,
		marketier.BaseUserAccount.LastName,
		marketier.BaseUserAccount.Email,
		marketier.BaseUserAccount.Address,
		marketier.BaseUserAccount.Password.hash,
		marketier.BaseUserAccount.LastLoginTime,
		marketier.BaseUserAccount.AccountStatus,
		marketier.BaseUserAccount.UserId,
		marketier.BaseUserAccount.Version,
	}

	marketierQuery := `
		UPDATE marketiers
		SET display_name = $1, about = $2, sales_generated = $3, tier = $4
		WHERE user_id = $5`

	marketierArgs := []interface{}{
		marketier.DisplayName,
		marketier.About,
		marketier.SalesGenerated,
		marketier.Tier,
		marketier.BaseUserAccount.UserId,
	}

	// Begin a transaction
	tx, err := marketierUserModel.DB.Begin()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Execute the first insertion
	err = tx.QueryRowContext(ctx, baseQuery, baseArgs...).Scan(&marketier.BaseUserAccount.Version)
	if err != nil {
		tx.Rollback()
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "base_users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	// Execute the second insertion
	_, err = tx.ExecContext(ctx, marketierQuery, marketierArgs...)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (marketierUserModel MarketierAccountModel) GetForToken(tokenScope, tokenPlaintext string) (*MarketierUserAccount, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT base_user.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type, display_name, about, sales_generated, tier
		FROM base_users 
		INNER JOIN marketiers ON base_users.user_id = marketiers.user_id
		INNER JOIN tokens ON base_users.user_id = tokens.user_id    
        WHERE tokens.hash = $1
        AND tokens.scope = $2
        AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var marketier MarketierUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := marketierUserModel.DB.QueryRowContext(ctx, query, args...).Scan(
		&marketier.BaseUserAccount.UserId,
		&marketier.BaseUserAccount.FirstName,
		&marketier.BaseUserAccount.LastName,
		&marketier.BaseUserAccount.Email,
		&marketier.BaseUserAccount.DateOfBirth,
		&marketier.BaseUserAccount.Gender,
		&marketier.BaseUserAccount.Address,
		&marketier.BaseUserAccount.Password.hash,
		&marketier.BaseUserAccount.AccountCreationTime,
		&marketier.BaseUserAccount.LastLoginTime,
		&marketier.BaseUserAccount.AccountStatus,
		&marketier.BaseUserAccount.Version,
		&marketier.BaseUserAccount.AccountType,
		&marketier.DisplayName,
		&marketier.About,
		&marketier.SalesGenerated,
		&marketier.Tier,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &marketier, nil
}

func ValidateMarketierUser(v *validator.Validator, marketierUserAccount *MarketierUserAccount) {
	ValidateBaseUser(v, &marketierUserAccount.BaseUserAccount)
	v.Check(marketierUserAccount.DisplayName != "", "display_name", "must be provided")
	v.Check(len(marketierUserAccount.DisplayName) <= 500, "display_name", "must not be more than 500 bytes long")
	v.Check(marketierUserAccount.About != "", "about", "must be provided")
	v.Check(len(marketierUserAccount.About) <= 5000, "about", "must not be more than 5000 bytes long")
	v.Check(marketierUserAccount.SalesGenerated >= 0, "sales_generated", "must be >= 0")
	v.Check(marketierUserAccount.SalesGenerated <= 10_000_000_000, "sales_generated", "must be <= 10,000,000,000")
	v.Check(marketierUserAccount.Tier >= 0, "tier", "must be >= 0")
	v.Check(marketierUserAccount.Tier <= 10, "tier", "must be <= 10")
}

func (marketierUserModel MarketierAccountModel) GetById(id int64) (*MarketierUserAccount, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
	SELECT base_user.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type, display_name, about, sales_generated, tier
	FROM base_users INNER JOIN marketier ON base_users.user_id = marketiers.user_id WHERE users.user_id = $1`

	var marketier MarketierUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := marketierUserModel.DB.QueryRowContext(ctx, query, id).Scan(
		&marketier.BaseUserAccount.UserId,
		&marketier.BaseUserAccount.FirstName,
		&marketier.BaseUserAccount.LastName,
		&marketier.BaseUserAccount.Email,
		&marketier.BaseUserAccount.DateOfBirth,
		&marketier.BaseUserAccount.Gender,
		&marketier.BaseUserAccount.Address,
		&marketier.BaseUserAccount.Password.hash,
		&marketier.BaseUserAccount.AccountCreationTime,
		&marketier.BaseUserAccount.LastLoginTime,
		&marketier.BaseUserAccount.AccountStatus,
		&marketier.BaseUserAccount.Version,
		&marketier.BaseUserAccount.AccountType,
		&marketier.DisplayName,
		&marketier.About,
		&marketier.SalesGenerated,
		&marketier.Tier,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &marketier, nil
}
