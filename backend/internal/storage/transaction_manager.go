package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AvitoCodeLab20/case-1-tamagotchi/backend/internal/activity"
)

type TransactionManager struct {
	db *pgxpool.Pool
}

func NewTransactionManager(db *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{db: db}
}

func (manager *TransactionManager) WithinTransaction(
	ctx context.Context,
	fn func(activity.PetRepository, activity.ActionRepository) error,
) error {
	tx, err := manager.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	petRepository := NewPetRepository(tx)
	actionRepository := NewPetActionRepository(tx)

	if err := fn(petRepository, actionRepository); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
