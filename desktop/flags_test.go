package main

import (
	"testing"

	"github.com/jameskbb/casebook/internal/demo"
)

func TestParseFlags(t *testing.T) {
	cfg, err := parseFlags(nil)
	if err != nil || cfg.DataDir != "" || cfg.SeedDemo {
		t.Fatalf("no args = %+v, %v", cfg, err)
	}
	cfg, err = parseFlags([]string{"--data-dir", `C:\casebook-dev\demo01`, "--seed-demo"})
	if err != nil || cfg.DataDir != `C:\casebook-dev\demo01` || !cfg.SeedDemo {
		t.Fatalf("data dir and seed = %+v, %v", cfg, err)
	}
	cfg, err = parseFlags([]string{"-psn_0_12345", "-data-dir=/tmp/x"})
	if err != nil || cfg.DataDir != "/tmp/x" {
		t.Fatalf("psn argument = %+v, %v", cfg, err)
	}
	if _, err := parseFlags([]string{"--nope"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	if _, err := parseFlags([]string{"stray"}); err == nil {
		t.Fatal("stray argument accepted")
	}
}

func TestParseFlagsSeedDemoStage(t *testing.T) {
	cfg, err := parseFlags([]string{"--seed-demo"})
	if err != nil || cfg.SeedDemoStage != demo.StageFull {
		t.Fatalf("default stage = %+v, %v", cfg, err)
	}
	cfg, err = parseFlags([]string{"--seed-demo", "--seed-demo-stage", "before-thursday"})
	if err != nil || cfg.SeedDemoStage != demo.StageBeforeThursday {
		t.Fatalf("before-thursday = %+v, %v", cfg, err)
	}
	if _, err := parseFlags([]string{"--seed-demo", "--seed-demo-stage", "friday"}); err == nil {
		t.Fatal("unknown stage accepted")
	}
	if _, err := parseFlags([]string{"--seed-demo-stage", "full"}); err == nil {
		t.Fatal("a stage without --seed-demo was accepted")
	}
}
