package main

import "testing"

func TestNetRateDelta(t *testing.T) {
	h1 := collectHost(Host{Name: "sentry", Address: "sentry", HasGPU: false})
	t.Logf("poll1 netRX=%.0f netTX=%.0f", h1.NetRX, h1.NetTX)
	h2 := collectHost(Host{Name: "sentry", Address: "sentry", HasGPU: false})
	dRx := h2.NetRX - h1.NetRX
	dTx := h2.NetTX - h1.NetTX
	rateRx := dRx * 8 / 1e6 / 2.0
	rateTx := dTx * 8 / 1e6 / 2.0
	t.Logf("poll2 netRX=%.0f deltaRx=%.0f rateRx=%.2f Mbps deltaTx=%.0f rateTx=%.2f Mbps",
		h2.NetRX, dRx, rateRx, dTx, rateTx)
}
