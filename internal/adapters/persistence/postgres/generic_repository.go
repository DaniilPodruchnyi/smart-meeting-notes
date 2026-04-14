package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type rowScanner[T any] func(pgx.Row) (*T, error)
type rowsScanner[T any] func(pgx.Rows) (T, error)

type Repository[T any] struct {
	db *DB
}

func newRepository[T any](db *DB) *Repository[T] {
	return &Repository[T]{db: db}
}

// create keeps INSERT ... RETURNING id in one place.
// Репозитории вызывают его и сами решают, что передавать в args.
func (r *Repository[T]) create(ctx context.Context, query string, idDest *int64, args ...any) error {
	return r.db.QueryRow(ctx, query, args...).Scan(idDest)
}

func (r *Repository[T]) getOne(ctx context.Context, query string, scan rowScanner[T], args ...any) (*T, error) {
	return scan(r.db.QueryRow(ctx, query, args...))
}

func (r *Repository[T]) getMany(ctx context.Context, query string, scan rowsScanner[T], args ...any) ([]T, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []T
	for rows.Next() {
		item, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
