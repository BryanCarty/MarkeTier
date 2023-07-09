package data

import (
	"context"
	"database/sql"
	"errors"
	"marketier/internal/validator"
	"time"
)

type Proposal struct {
	ProposalId int64  `json:"proposal_id"`
	Title      string `json:"title"`
	About      string `json:"about"`
	Version    int    `json:"version"`
}

type ProposalModel struct {
	DB *sql.DB
}

func ValidateProposal(v *validator.Validator, proposal *Proposal) {
	v.Check(proposal.Title != "", "title", "must be provided")
	v.Check(len(proposal.Title) <= 250, "title", "must not be more than 250 bytes long")
	v.Check(proposal.About != "", "about", "must be provided")
	v.Check(len(proposal.About) <= 4000, "name", "must not be more than 4000 bytes long")
}

func (p ProposalModel) Insert(proposal *Proposal) error {
	query := `
        INSERT INTO proposals (title, about) 
        VALUES ($1, $2)
        RETURNING product_id, version`

	args := []interface{}{proposal.Title, proposal.About}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.DB.QueryRowContext(ctx, query, args...).Scan(&proposal.ProposalId, &proposal.Version)
}

func (p ProposalModel) Get(id int64) (*Proposal, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `
        SELECT proposal_id, title, about, version
        FROM products
        WHERE id = $1`

	var proposal Proposal

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := p.DB.QueryRowContext(ctx, query, id).Scan(
		&proposal.ProposalId,
		&proposal.Title,
		&proposal.About,
		&proposal.Version,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &proposal, nil
}

func (p ProposalModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `
        DELETE FROM proposals
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

func (p ProposalModel) Update(proposal *Proposal) error {
	query := `
        UPDATE proposals 
        SET title = $1, about = $2, version = version + 1
        WHERE id = $4 AND version = $5
        RETURNING version`

	args := []interface{}{
		proposal.Title,
		proposal.About,
		proposal.ProposalId,
		proposal.Version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := p.DB.QueryRowContext(ctx, query, args...).Scan(&proposal.Version)
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
