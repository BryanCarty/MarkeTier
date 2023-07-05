package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type ProductOwnerUserAccount struct {
	BaseUserAccount BaseUserAccount
	DisplayName     string `json:"display_name"`
	About           string `json:"about"`
	SalesGenerated  int    `json:"sales_generated"`
}

type ProductOwnerAccountModel struct {
	DB *sql.DB
}

func (productOwnerModel ProductOwnerAccountModel) Insert(productOwnerUser *ProductOwnerUserAccount) error {
	baseQuery := `
        INSERT INTO base_users (first_name, last_name, email, date_of_birth, gender, address, password, account_type) 
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        RETURNING user_id, account_creation_time, account_status, version`

	baseArgs := []interface{}{productOwnerUser.BaseUserAccount.FirstName, productOwnerUser.BaseUserAccount.LastName, productOwnerUser.BaseUserAccount.Email, productOwnerUser.BaseUserAccount.DateOfBirth, productOwnerUser.BaseUserAccount.Gender, productOwnerUser.BaseUserAccount.Address, productOwnerUser.BaseUserAccount.Password.hash, productOwnerUser.BaseUserAccount.AccountType}

	marketierQuery := `
	INSERT INTO product_owners (user_id, display_name, about, sales_generated) 
	VALUES ($1, $2, $3, $4)`

	marketierArgs := []interface{}{productOwnerUser.DisplayName, productOwnerUser.About, productOwnerUser.SalesGenerated}

	// Begin a transaction
	tx, err := productOwnerModel.DB.Begin()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Execute the first insertion
	err = tx.QueryRowContext(ctx, baseQuery, baseArgs...).Scan(&productOwnerUser.BaseUserAccount.UserId, &productOwnerUser.BaseUserAccount.AccountCreationTime, &productOwnerUser.BaseUserAccount.AccountStatus, &productOwnerUser.BaseUserAccount.Version)
	if err != nil {
		tx.Rollback()
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "base_users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	updatedMarketierArgs := append([]interface{}{productOwnerUser.BaseUserAccount.UserId}, marketierArgs...)

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

func (productOwnerModel ProductOwnerAccountModel) GetByEmail(email string) (*ProductOwnerUserAccount, error) {
	query := `
        SELECT base_user.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type, display_name, about, sales_generated, tier
        FROM base_users INNER JOIN product_owners ON base_users.user_id = product_owners.user_id WHERE users.email = $1`

	var productOwner ProductOwnerUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := productOwnerModel.DB.QueryRowContext(ctx, query, email).Scan(
		&productOwner.BaseUserAccount.UserId,
		&productOwner.BaseUserAccount.FirstName,
		&productOwner.BaseUserAccount.LastName,
		&productOwner.BaseUserAccount.Email,
		&productOwner.BaseUserAccount.DateOfBirth,
		&productOwner.BaseUserAccount.Gender,
		&productOwner.BaseUserAccount.Address,
		&productOwner.BaseUserAccount.Password.hash,
		&productOwner.BaseUserAccount.AccountCreationTime,
		&productOwner.BaseUserAccount.LastLoginTime,
		&productOwner.BaseUserAccount.AccountStatus,
		&productOwner.BaseUserAccount.Version,
		&productOwner.BaseUserAccount.AccountType,
		&productOwner.DisplayName,
		&productOwner.About,
		&productOwner.SalesGenerated,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &productOwner, nil
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

func (productOwnerModel ProductOwnerAccountModel) Update(productOwnerUser *ProductOwnerUserAccount) error {
	baseQuery := `
        UPDATE base_users
        SET first_name = $1, last_name = $2, email = $3, address = $4, password = $5, last_login_time = $6, account_status = $7, version = version + 1
        WHERE user_id = $8 AND version = $9
        RETURNING version`

	baseArgs := []interface{}{
		productOwnerUser.BaseUserAccount.FirstName,
		productOwnerUser.BaseUserAccount.LastName,
		productOwnerUser.BaseUserAccount.Email,
		productOwnerUser.BaseUserAccount.Address,
		productOwnerUser.BaseUserAccount.Password.hash,
		productOwnerUser.BaseUserAccount.LastLoginTime,
		productOwnerUser.BaseUserAccount.AccountStatus,
		productOwnerUser.BaseUserAccount.UserId,
		productOwnerUser.BaseUserAccount.Version,
	}

	marketierQuery := `
		UPDATE product_owners
		SET display_name = $1, about = $2, sales_generated = $3
		WHERE user_id = $4`

	marketierArgs := []interface{}{
		productOwnerUser.DisplayName,
		productOwnerUser.About,
		productOwnerUser.SalesGenerated,
		productOwnerUser.BaseUserAccount.UserId,
	}

	// Begin a transaction
	tx, err := productOwnerModel.DB.Begin()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	// Execute the first insertion
	err = tx.QueryRowContext(ctx, baseQuery, baseArgs...).Scan(&productOwnerUser.BaseUserAccount.Version)
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

func (productOwnerModel ProductOwnerAccountModel) GetForToken(tokenScope, tokenPlaintext string) (*ProductOwnerUserAccount, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `
		SELECT base_user.user_id, first_name, last_name, email, date_of_birth, gender, address, password, account_creation_time, last_login_time, account_status, version, account_type, display_name, about, sales_generated, tier
		FROM base_users 
		INNER JOIN product_owners ON base_users.user_id = marketiers.user_id
		INNER JOIN tokens ON base_users.user_id = tokens.user_id    
        WHERE tokens.hash = $1
        AND tokens.scope = $2
        AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var productOwner ProductOwnerUserAccount

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := productOwnerModel.DB.QueryRowContext(ctx, query, args...).Scan(
		&productOwner.BaseUserAccount.UserId,
		&productOwner.BaseUserAccount.FirstName,
		&productOwner.BaseUserAccount.LastName,
		&productOwner.BaseUserAccount.Email,
		&productOwner.BaseUserAccount.DateOfBirth,
		&productOwner.BaseUserAccount.Gender,
		&productOwner.BaseUserAccount.Address,
		&productOwner.BaseUserAccount.Password.hash,
		&productOwner.BaseUserAccount.AccountCreationTime,
		&productOwner.BaseUserAccount.LastLoginTime,
		&productOwner.BaseUserAccount.AccountStatus,
		&productOwner.BaseUserAccount.Version,
		&productOwner.BaseUserAccount.AccountType,
		&productOwner.DisplayName,
		&productOwner.About,
		&productOwner.SalesGenerated,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &productOwner, nil
}

func ValidateProductOwnerUser(v *validator.Validator, productOwnerUserAccount *ProductOwnerUserAccount) {
	ValidateBaseUser(v, &productOwnerUserAccount.BaseUserAccount)
	v.Check(productOwnerUserAccount.DisplayName != "", "display_name", "must be provided")
	v.Check(len(productOwnerUserAccount.DisplayName) <= 500, "display_name", "must not be more than 500 bytes long")
	v.Check(productOwnerUserAccount.About != "", "about", "must be provided")
	v.Check(len(productOwnerUserAccount.About) <= 5000, "about", "must not be more than 5000 bytes long")
	v.Check(productOwnerUserAccount.SalesGenerated >= 0, "sales_generated", "must be >= 0")
	v.Check(productOwnerUserAccount.SalesGenerated <= 10_000_000_000, "sales_generated", "must be <= 10,000,000,000")
}
