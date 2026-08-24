package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProtectDefaultDatabaseDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "karya.db")
	if err := protectDefaultDatabaseDir(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("directory permissions = %o, want 700", info.Mode().Perm())
	}
}

func TestAgentGuideCoversMutationSafety(t *testing.T) {
	for _, required := range []string{"status:   backlog | in-progress | review | done | cancelled", "--reason", "--parent", "ticket note add", "Karya generates the UTC timestamp", "never blindly retry"} {
		if !strings.Contains(agentGuide, required) {
			t.Errorf("agent guide does not contain %q", required)
		}
	}
	if strings.Contains(agentGuide, "task | bug | spike | scope") {
		t.Fatal("agent guide still advertises the removed scope type")
	}
}

func TestDatabasePathUsesUserConfigDirectory(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	path, err := databasePath("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configDir, "karya", "karya.db")
	if path != want {
		t.Fatalf("databasePath() = %q, want %q", path, want)
	}

	path, err = databasePath("/tmp/test.db")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/test.db" {
		t.Fatalf("databasePath override = %q", path)
	}
}

func TestWriteResourceAndDeletedJSON(t *testing.T) {
	command := &cobra.Command{}
	var output bytes.Buffer
	command.SetOut(&output)
	previous := jsonOutput
	t.Cleanup(func() { jsonOutput = previous })
	jsonOutput = true

	if err := writeResource(command, struct {
		Name string `json:"name"`
	}{Name: "Karya"}, func() error {
		t.Fatal("human output called for JSON")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "{\"name\":\"Karya\"}\n"; got != want {
		t.Fatalf("writeResource() = %q, want %q", got, want)
	}

	output.Reset()
	if err := writeDeleted(command, "APP-1"); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "{\"deleted\":\"APP-1\"}\n"; got != want {
		t.Fatalf("writeDeleted() = %q, want %q", got, want)
	}
}
