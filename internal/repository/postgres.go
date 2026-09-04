package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tandem/tandem/internal/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type Postgres struct {
	DB *gorm.DB
}

func NewPostgres(cfg config.DatabaseConfig, opts ...Option) (*Postgres, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}

	gormCfg := &gorm.Config{}
	if o.Silent {
		gormCfg.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	}

	db, err := gorm.Open(postgres.Open(dsn(cfg)), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(o.MaxOpenConns)
	sqlDB.SetMaxIdleConns(o.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(o.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), o.ConnectTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Postgres{DB: db}, nil
}

func (p *Postgres) AutoMigrate(models ...interface{}) error {
	return p.DB.AutoMigrate(models...)
}

func (p *Postgres) Close() error {
	sqlDB, err := p.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func dsn(cfg config.DatabaseConfig) string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)
}

type options struct {
	Silent          bool
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnectTimeout  time.Duration
}

type Option func(*options)

func WithPool(maxOpen, maxIdle int, lifetime time.Duration) Option {
	return func(o *options) {
		o.MaxOpenConns = maxOpen
		o.MaxIdleConns = maxIdle
		o.ConnMaxLifetime = lifetime
	}
}

func WithConnectTimeout(d time.Duration) Option {
	return func(o *options) {
		o.ConnectTimeout = d
	}
}

func WithSilentLogger() Option {
	return func(o *options) {
		o.Silent = true
	}
}
