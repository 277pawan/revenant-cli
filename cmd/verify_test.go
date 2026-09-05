package cmd

import (
	"os"
	"testing"

	"github.com/pawan-bisht/revenant/internal/config"
)

func TestBuildConnectionStringUsesDiscoveredEndpoint(t *testing.T) {
	connection, err := buildConnectionString(
		"sandbox.example.com:5432",
		"postgres://user:password@${SANDBOX_ENDPOINT}:5432/app?sslmode=require",
	)
	if err != nil {
		t.Fatalf("buildConnectionString() error = %v", err)
	}

	const want = "postgres://user:password@sandbox.example.com:5432/app?sslmode=require"
	if connection != want {
		t.Fatalf("buildConnectionString() = %q, want %q", connection, want)
	}
}

func TestConfigUsesSandboxEnvironmentValues(t *testing.T) {
	t.Setenv("SANDBOX_USER", "aws-user")
	t.Setenv("SANDBOX_PASSWORD", "aws-password")
	t.Setenv("SANDBOX_DBNAME", "restored")
	os.Unsetenv("SANDBOX_ENDPOINT")

	loaded, err := config.Load("../revenant-aws-freetier.yaml")
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}

	want := "postgres://aws-user:aws-password@${SANDBOX_ENDPOINT}:5432/restored?sslmode=require"
	if loaded.Database.Connection != want {
		t.Fatalf("connection = %q, want %q", loaded.Database.Connection, want)
	}
}

func TestRecoveryInstanceClass(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		freeTier   bool
		want       string
	}{
		{name: "configured class wins", configured: "db.t4g.micro", freeTier: true, want: "db.t4g.micro"},
		{name: "free tier default supports encrypted snapshots", freeTier: true, want: "db.t3.micro"},
		{name: "standard default", want: "db.t4g.micro"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := recoveryInstanceClass(test.configured, test.freeTier); got != test.want {
				t.Fatalf("recoveryInstanceClass() = %q, want %q", got, test.want)
			}
		})
	}
}
