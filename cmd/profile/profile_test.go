package profile

import (
	"os"
	"testing"

	"github.com/PhilipKram/jenkins-cli/internal/config"
)

func setupTestEnv(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("JENKINS_URL", "")
	t.Setenv("JENKINS_USER", "")
	t.Setenv("JENKINS_TOKEN", "")
	t.Setenv("JENKINS_BEARER_TOKEN", "")
	t.Setenv("JENKINS_PROFILE", "")
	config.ActiveProfile = ""
}

func createProfiles(t *testing.T, profiles map[string]config.Config, current string) {
	t.Helper()
	mc := &config.MultiConfig{
		CurrentProfile: current,
		Profiles:       profiles,
	}
	if err := config.SaveMulti(mc); err != nil {
		t.Fatalf("SaveMulti: %v", err)
	}
}

func TestListCommand(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"images": {URL: "http://images:8080", User: "img-user"},
		"helm":   {URL: "http://helm:8080", User: "helm-user"},
	}, "images")

	Cmd.SetOut(os.Stdout)
	Cmd.SetArgs([]string{"list"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("list: %v", err)
	}
}

func TestUseCommand(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"alpha": {URL: "http://alpha:8080"},
		"beta":  {URL: "http://beta:8080"},
	}, "alpha")

	Cmd.SetArgs([]string{"use", "beta"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("use: %v", err)
	}

	mc, err := config.LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	if mc.CurrentProfile != "beta" {
		t.Errorf("expected current profile beta, got %s", mc.CurrentProfile)
	}
}

func TestUseCommandNonexistent(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"alpha": {URL: "http://alpha:8080"},
	}, "alpha")

	Cmd.SetArgs([]string{"use", "nonexistent"})

	err := Cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}

func TestDeleteCommand(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"keep":   {URL: "http://keep:8080"},
		"remove": {URL: "http://remove:8080"},
	}, "keep")

	Cmd.SetArgs([]string{"delete", "remove"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	mc, err := config.LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	if _, ok := mc.Profiles["remove"]; ok {
		t.Error("expected remove profile to be deleted")
	}
	if _, ok := mc.Profiles["keep"]; !ok {
		t.Error("expected keep profile to remain")
	}
}

func TestDeleteLastProfileFails(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"only": {URL: "http://only:8080"},
	}, "only")

	Cmd.SetArgs([]string{"delete", "only"})

	err := Cmd.Execute()
	if err == nil {
		t.Fatal("expected error when deleting last profile")
	}
}

func TestDeleteActiveProfileSwitches(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"active": {URL: "http://active:8080"},
		"other":  {URL: "http://other:8080"},
	}, "active")

	Cmd.SetArgs([]string{"delete", "active"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("delete: %v", err)
	}

	mc, err := config.LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	if mc.CurrentProfile == "active" {
		t.Error("current profile should have switched away from deleted profile")
	}
	if mc.CurrentProfile != "other" {
		t.Errorf("expected current profile to switch to other, got %s", mc.CurrentProfile)
	}
}

func TestShowCommand(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"myprofile": {URL: "http://myprofile:8080", User: "myuser", Insecure: true},
	}, "myprofile")

	Cmd.SetArgs([]string{"show", "myprofile"})

	if err := Cmd.Execute(); err != nil {
		t.Fatalf("show: %v", err)
	}
}

func TestShowCommandNonexistent(t *testing.T) {
	setupTestEnv(t)
	createProfiles(t, map[string]config.Config{
		"exists": {URL: "http://exists:8080"},
	}, "exists")

	Cmd.SetArgs([]string{"show", "nope"})

	err := Cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}
