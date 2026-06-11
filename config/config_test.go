package config

import "testing"

func TestDBConfigDSN(t *testing.T) {
	t.Run("composes a postgres url", func(t *testing.T) {
		cfg := DBConfig{
			Host:     "localhost",
			Port:     "5432",
			Name:     "relay",
			User:     "relay",
			Password: "secret",
		}
		want := "postgres://relay:secret@localhost:5432/relay?sslmode=disable"
		if got := cfg.DSN(); got != want {
			t.Errorf("DSN() = %q, want %q", got, want)
		}
	})
}

func TestLoadDB(t *testing.T) {
	t.Run("reads DB_* env vars", func(t *testing.T) {
		t.Setenv("DB_HOST", "localhost")
		t.Setenv("DB_PORT", "5433")
		t.Setenv("DB_NAME", "relay")
		t.Setenv("DB_USER", "relay")
		t.Setenv("DB_PASSWORD", "secret")

		want := DBConfig{Host: "localhost", Port: "5433", Name: "relay", User: "relay", Password: "secret"}
		got, err := LoadDB()
		if err != nil {
			t.Fatalf("LoadDB() error = %v", err)
		}
		if got != want {
			t.Errorf("LoadDB() = %+v, want %+v", got, want)
		}
	})
}
