package bot

import (
	"os"
	"testing"

	"rlangga/internal/config"
)

func TestFromConfig_Nil(t *testing.T) {
	b := FromConfig(nil)
	if b.Name != "default" {
		t.Fatal(b.Name)
	}
}

func TestFromConfig_Full(t *testing.T) {
	c := &config.Config{
		MinHold: 5, MaxHold: 10, TakeProfit: 7, StopLoss: 4, PanicSL: 9, MomentumDrop: 2, GraceSeconds: 3,
	}
	b := FromConfig(c)
	if b.Name != "default" || b.MinHold != 5 || b.PanicLoss != 9 || b.GraceSeconds != 3 {
		t.Fatalf("%+v", b)
	}
}

func TestLoadBots_DefaultWhenEmpty(t *testing.T) {
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	bots, err := LoadBots()
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 2 || bots[0].Name != "bot-10s" || bots[1].Name != "bot-15s" {
		t.Fatalf("%+v", bots)
	}
}

func TestLoadBots_CustomJSON(t *testing.T) {
	t.Setenv("BOTS_JSON", `[{"name":"a","min_hold":1,"max_hold":5,"take_profit":7,"stop_loss":5,"panic_loss":8,"momentum_drop":2,"grace_seconds":2}]`)
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	bots, err := LoadBots()
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 1 || bots[0].Name != "a" {
		t.Fatalf("%+v", bots)
	}
}

func TestLoadBots_EmptyNameGetsIndex(t *testing.T) {
	t.Setenv("BOTS_JSON", `[{"min_hold":1,"max_hold":5,"take_profit":7,"stop_loss":5,"panic_loss":8,"momentum_drop":2,"grace_seconds":2}]`)
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	bots, err := LoadBots()
	if err != nil || bots[0].Name != "bot-0" {
		t.Fatalf("%v %+v", err, bots)
	}
}

func TestLoadBots_InvalidJSON(t *testing.T) {
	t.Setenv("BOTS_JSON", `{`)
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	if _, err := LoadBots(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBots_MinGreaterThanMax(t *testing.T) {
	t.Setenv("BOTS_JSON", `[{"name":"bad","min_hold":10,"max_hold":5,"take_profit":7,"stop_loss":5,"panic_loss":8,"momentum_drop":2,"grace_seconds":2}]`)
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	if _, err := LoadBots(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBots_EmptyArray(t *testing.T) {
	t.Setenv("BOTS_JSON", `[]`)
	t.Cleanup(func() { _ = os.Unsetenv("BOTS_JSON") })
	if _, err := LoadBots(); err == nil {
		t.Fatal("expected error")
	}
}
