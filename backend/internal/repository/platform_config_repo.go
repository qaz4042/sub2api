package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type platformConfigRepository struct {
	db *sql.DB
}

func NewPlatformConfigRepository(db *sql.DB) service.PlatformConfigRepository {
	return &platformConfigRepository{db: db}
}

func (r *platformConfigRepository) List(ctx context.Context) ([]service.PlatformConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT key, label, description, enabled, core, sort_order, created_at, updated_at
		FROM platform_configs
		ORDER BY sort_order ASC, key ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.PlatformConfig, 0)
	for rows.Next() {
		item, err := scanPlatformConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *platformConfigRepository) Get(ctx context.Context, key string) (*service.PlatformConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT key, label, description, enabled, core, sort_order, created_at, updated_at
		FROM platform_configs
		WHERE key = $1
	`, strings.ToLower(strings.TrimSpace(key)))
	item, err := scanPlatformConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPlatformConfigNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *platformConfigRepository) SetEnabled(ctx context.Context, key string, enabled bool) (*service.PlatformConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE platform_configs
		SET enabled = $2, updated_at = NOW()
		WHERE key = $1
		RETURNING key, label, description, enabled, core, sort_order, created_at, updated_at
	`, strings.ToLower(strings.TrimSpace(key)), enabled)
	item, err := scanPlatformConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPlatformConfigNotFound
		}
		return nil, err
	}
	return &item, nil
}

type platformConfigScanner interface {
	Scan(dest ...any) error
}

func scanPlatformConfig(scanner platformConfigScanner) (service.PlatformConfig, error) {
	var item service.PlatformConfig
	err := scanner.Scan(
		&item.Key,
		&item.Label,
		&item.Description,
		&item.Enabled,
		&item.Core,
		&item.SortOrder,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}
