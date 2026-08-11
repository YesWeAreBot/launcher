package cmd

import "testing"

func TestNewUpdateCmdHasAppFlag(t *testing.T) {
	cmd := newUpdateCmd()
	if cmd.Use != "update" {
		t.Fatalf("Use = %q, want update", cmd.Use)
	}
	if err := cmd.Args(cmd, []string{"extra"}); err == nil {
		t.Fatal("update command accepts unexpected arguments")
	}
	if cmd.Flags().Lookup("app") == nil {
		t.Fatal("update command is missing --app")
	}
}
