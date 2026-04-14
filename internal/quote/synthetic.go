package quote

import (
	"hash/fnv"
	"math"

	"rlangga/internal/config"
)

// SyntheticSellQuoteForEngine menghitung harga jual simulasi (SOL) saat quote HTTP tidak ada
// (SIMULATE_ENGINE). Osilasi deterministik per mint supaya mesin exit adaptif tetap bisa trigger.
// Amplitudo/periode/drift diatur lewat SIMULATE_SYNTH_* di .env (lihat config.Config).
func SyntheticSellQuoteForEngine(mint string, buySOL float64, elapsedSec int) float64 {
	if buySOL <= 0 {
		return 0
	}
	amp := 12.0
	period := 4.0
	drift := 0.0
	if cfg := config.C; cfg != nil {
		if cfg.SimulateSynthAmplitudePct > 0 {
			amp = cfg.SimulateSynthAmplitudePct
		}
		if cfg.SimulateSynthPeriodSec > 0 {
			period = cfg.SimulateSynthPeriodSec
		}
		drift = cfg.SimulateSynthDriftPct
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(mint))
	phase := float64(h.Sum32()%628) / 100.0
	pct := amp*math.Sin(float64(elapsedSec)/period+phase) + drift
	return buySOL * (1 + pct/100.0)
}
