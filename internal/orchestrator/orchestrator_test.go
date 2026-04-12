package orchestrator

import (
	"sync"
	"testing"

	"rlangga/internal/bot"
	"rlangga/internal/config"
)

func TestNextBot_RoundRobin(t *testing.T) {
	Init([]bot.BotConfig{
		{Name: "x", MinHold: 1, MaxHold: 2, TakeProfit: 1, StopLoss: 1, PanicLoss: 1, MomentumDrop: 1, GraceSeconds: 0},
		{Name: "y", MinHold: 1, MaxHold: 2, TakeProfit: 1, StopLoss: 1, PanicLoss: 1, MomentumDrop: 1, GraceSeconds: 0},
	})
	if a, b := NextBot(), NextBot(); a.Name != "x" || b.Name != "y" {
		t.Fatalf("got %s %s", a.Name, b.Name)
	}
	if c := NextBot(); c.Name != "x" {
		t.Fatalf("got %s", c.Name)
	}
}

func TestNextBot_Concurrent(t *testing.T) {
	Init([]bot.BotConfig{
		{Name: "a", MinHold: 1, MaxHold: 2, TakeProfit: 1, StopLoss: 1, PanicLoss: 1, MomentumDrop: 1, GraceSeconds: 0},
		{Name: "b", MinHold: 1, MaxHold: 2, TakeProfit: 1, StopLoss: 1, PanicLoss: 1, MomentumDrop: 1, GraceSeconds: 0},
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			NextBot()
		}()
	}
	wg.Wait()
}

func TestRecoveryBot_FirstProfile(t *testing.T) {
	Init([]bot.BotConfig{{Name: "first", MinHold: 1, MaxHold: 2, TakeProfit: 1, StopLoss: 1, PanicLoss: 1, MomentumDrop: 1, GraceSeconds: 0}})
	if RecoveryBot().Name != "first" {
		t.Fatal(RecoveryBot().Name)
	}
}

func TestInit_EmptyUsesConfig(t *testing.T) {
	config.C = &config.Config{
		MinHold: 5, MaxHold: 15, TakeProfit: 7, StopLoss: 5, PanicSL: 8, MomentumDrop: 2.5, GraceSeconds: 2,
	}
	t.Cleanup(func() { config.C = nil })
	Init(nil)
	if NextBot().Name != "default" {
		t.Fatal(NextBot().Name)
	}
}

func TestInit_NilConfigUsesDefaultBots(t *testing.T) {
	config.C = nil
	bots = nil
	idx.Store(0)
	Init(nil)
	if len(bots) != 2 {
		t.Fatalf("len=%d", len(bots))
	}
}

func TestNextBot_UninitializedNoConfig(t *testing.T) {
	config.C = nil
	bots = nil
	idx.Store(0)
	if NextBot().Name != "bot-10s" {
		t.Fatal(NextBot().Name)
	}
}

func TestRecoveryBot_UninitializedNoConfig(t *testing.T) {
	config.C = nil
	bots = nil
	if RecoveryBot().Name != "bot-10s" {
		t.Fatal(RecoveryBot().Name)
	}
}
