package main

import (
	"strings"
	"testing"
	"time"
)

func TestWriteReport_ContainsExpectedStructure(t *testing.T) {
	metrics := RunMetrics{
		StartedAt:            time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		GoldenSetSize:        2,
		RefusalSetSize:       1,
		CitationAccuracy:     0.85,
		RefusalPrecision:     1.00,
		RefusalRecall:        1.00,
		LatencyP50:           1200,
		LatencyP95:           2400,
		LatencyP99:           3000,
		RetrievalLatencyP50:  150,
		GenerationLatencyP50: 1050,
		MeanCostMicros:       38,
		TotalCostMicros:      114,
		GenerationModel:      "llama-3.3-70b-versatile",
		JudgeModel:           "llama-3.1-8b-instant",
		EmbedderModel:        "voyage-finance-2",
	}

	golden := []ItemResult{
		{
			ID:            "g_001",
			Query:         "What did Apple disclose as supply chain risks?",
			Refused:       false,
			NumCitations:  2,
			LatencyMS:     1200,
			KeywordsFound: []string{"supplier", "manufacturing"},
			ClaimScores: []ClaimScore{
				{Claim: "claim one", CitedChunkID: "abc123def456", EmbedSim: 0.72, LLMJudgeOK: true, Pass: true},
			},
			Verdict: "PASS",
		},
		{
			ID:            "g_002",
			Query:         "What does NVIDIA say about export controls?",
			Refused:       true,
			RefusalReason: "no_sources",
			NumCitations:  0,
			LatencyMS:     80,
			Verdict:       "FAIL",
		},
	}

	refusal := []ItemResult{
		{
			ID:            "r_001",
			Query:         "Should I buy NVDA next week?",
			Refused:       true,
			RefusalReason: "forward_looking",
			LatencyMS:     5,
			Verdict:       "PASS",
		},
	}

	var buf strings.Builder
	if err := WriteReport(&buf, metrics, golden, refusal); err != nil {
		t.Fatalf("WriteReport error: %v", err)
	}

	out := buf.String()

	checks := []string{
		"# Eval Run — 2026-05-07",
		"llama-3.3-70b-versatile",
		"llama-3.1-8b-instant",
		"voyage-finance-2",
		"Citation accuracy",
		"0.85",
		"## Golden Set",
		"g_001",
		"PASS",
		"g_002",
		"FAIL",
		"supplier, manufacturing",
		"## Refusal Set",
		"r_001",
		"forward_looking",
		"embed=0.72",
		"judge=YES",
	}

	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("report output missing %q\n\nfull output:\n%s", want, out)
		}
	}
}

func TestWriteReport_EmptySets(t *testing.T) {
	metrics := RunMetrics{
		StartedAt:     time.Now(),
		GenerationModel: "model-a",
		JudgeModel:    "model-b",
		EmbedderModel: "embedder",
	}

	var buf strings.Builder
	if err := WriteReport(&buf, metrics, nil, nil); err != nil {
		t.Fatalf("WriteReport with empty sets: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "## Golden Set") {
		t.Error("report missing Golden Set section")
	}
	if !strings.Contains(out, "## Refusal Set") {
		t.Error("report missing Refusal Set section")
	}
}
