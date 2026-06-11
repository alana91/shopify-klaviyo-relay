package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/alana91/shopify-klaviyo-relay/config"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func New(t *testing.T, migrate func(context.Context, *sql.DB) error) string {
	t.Helper()
	ctx := context.Background()

	dbCfg, err := config.LoadDB()
	if err != nil {
		t.Fatalf("load db config: %v", err)
	}

	adminCfg := dbCfg
	adminCfg.Name = "postgres"
	admin, err := sql.Open("pgx", adminCfg.DSN())
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("ping admin connection: %v", err)
	}

	name := fmt.Sprintf("relay_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}

	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
		_ = admin.Close()
	})

	testCfg := dbCfg
	testCfg.Name = name
	dsn := testCfg.DSN()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	_ = db.Close()

	return dsn
}
