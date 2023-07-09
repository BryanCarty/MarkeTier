package data

import (
	"context"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type Product struct {
	ProductId int64  `json:"product_id"`
	Name      string `json:"name"`
	About     string `json:"about"`
	Stars     int8   `json:"stars"`
	Version   int    `json:"version"`
}

type ProductModel struct {
	DB *sql.DB
}

func ValidateProduct(v *validator.Validator, product *Product) {
	v.Check(product.Name != "", "name", "must be provided")
	v.Check(len(product.Name) <= 250, "name", "must not be more than 250 bytes long")
	v.Check(product.About != "", "about", "must be provided")
	v.Check(len(product.About) <= 2500, "name", "must not be more than 2500 bytes long")
}

func (p ProductModel) Insert(product *Product) error {
	query := `
        INSERT INTO products (name, about) 
        VALUES ($1, $2)
        RETURNING product_id, stars, version`

	args := []interface{}{product.Name, product.About}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.DB.QueryRowContext(ctx, query, args...).Scan(&product.ProductId, &product.Stars, &product.Version)
}

func (p ProductModel) Get(id int64) (*Product, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
        SELECT product_id, name, about, stars, version
        FROM products
        WHERE id = $1`

	var product Product

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := p.DB.QueryRowContext(ctx, query, id).Scan(
		&product.ProductId,
		&product.Name,
		&product.About,
		&product.Stars,
		&product.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &product, nil
}

func (p ProductModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM products
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := p.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (p ProductModel) Update(product *Product) error {
	query := `
        UPDATE products 
        SET name = $1, about = $2, stars = $3, version = version + 1
        WHERE id = $4 AND version = $5
        RETURNING version`

	args := []interface{}{
		product.Name,
		product.About,
		product.Stars,
		product.ProductId,
		product.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := p.DB.QueryRowContext(ctx, query, args...).Scan(&product.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}
