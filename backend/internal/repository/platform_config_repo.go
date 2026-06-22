package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
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

	var items []service.PlatformConfig
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

func (r *platformConfigRepository) Create(ctx context.Context, input service.PlatformConfigInput) (*service.PlatformConfig, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO platform_configs (key, label, description, enabled, core, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, FALSE, $5, NOW(), NOW())
		RETURNING key, label, description, enabled, core, sort_order, created_at, updated_at
	`, input.Key, input.Label, input.Description, input.Enabled, input.SortOrder)
	item, err := scanPlatformConfig(row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, infraerrors.Conflict("PLATFORM_CONFIG_EXISTS", "platform config already exists")
		}
		return nil, err
	}
	return &item, nil
}

func (r *platformConfigRepository) Update(ctx context.Context, key string, input service.PlatformConfigUpdate) (*service.PlatformConfig, error) {
	existing, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	label := existing.Label
	description := existing.Description
	enabled := existing.Enabled
	sortOrder := existing.SortOrder
	if input.Label != nil {
		label = *input.Label
	}
	if input.Description != nil {
		description = *input.Description
	}
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if input.SortOrder != nil {
		sortOrder = *input.SortOrder
	}
	if existing.Core {
		enabled = true
	}

	row := r.db.QueryRowContext(ctx, `
		UPDATE platform_configs
		SET label = $2,
		    description = $3,
		    enabled = $4,
		    sort_order = $5,
		    updated_at = NOW()
		WHERE key = $1
		RETURNING key, label, description, enabled, core, sort_order, created_at, updated_at
	`, existing.Key, label, description, enabled, sortOrder)
	item, err := scanPlatformConfig(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrPlatformConfigNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *platformConfigRepository) Delete(ctx context.Context, key string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM platform_configs WHERE key = $1`, strings.ToLower(strings.TrimSpace(key)))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return service.ErrPlatformConfigNotFound
	}
	return nil
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
