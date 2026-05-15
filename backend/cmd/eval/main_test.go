package main

import "testing"

// Tests for calcRefusalMetrics catch BLOCKER-2-class bugs where false positives
// from the golden loop are not counted in the precision denominator.

func TestCalcRefusalMetrics_AllCorrect(t *testing.T) {
	// 0 golden FPs, 3 true positives, 0 refusal FPs, 0 false negatives.
	precision, recall := calcRefusalMetrics(0, 3, 0, 0)
	if precision != 1.0 {
		t.Errorf("precision = %f, want 1.0", precision)
	}
	if recall != 1.0 {
		t.Errorf("recall = %f, want 1.0", recall)
	}
}

func TestCalcRefusalMetrics_AllWrong(t *testing.T) {
	// System refused all 3 golden items (goldenFPs=3) and answered all 3 refusal
	// items (fNs=3). True positives and refusal-loop FPs are 0.
	// precision = 0 / 3 = 0.0; recall = 0 / 3 = 0.0.
	precision, recall := calcRefusalMetrics(3, 0, 0, 3)
	if precision != 0.0 {
		t.Errorf("precision = %f, want 0.0", precision)
	}
	if recall != 0.0 {
		t.Errorf("recall = %f, want 0.0", recall)
	}
}

func TestCalcRefusalMetrics_EmptyRefusalSet(t *testing.T) {
	// No refusal-set items at all. Recall denominator is 0; return 0.
	// 2 true positives, 0 false negatives → but wait: no refusal items means
	// tPs=0 too. goldenFPs=1 exists.
	// precision = 0 / 1 = 0; recall = 0 / 0 → 0.
	precision, recall := calcRefusalMetrics(1, 0, 0, 0)
	if precision != 0.0 {
		t.Errorf("precision = %f, want 0.0", precision)
	}
	if recall != 0.0 {
		t.Errorf("recall = %f, want 0.0 (undefined → 0)", recall)
	}
}

func TestCalcRefusalMetrics_EmptyGoldenSet(t *testing.T) {
	// No golden items; precision counts only refusal-loop outcomes.
	// 2 tPs, 1 refusal FP → precision = 2/3; recall = 2/2 = 1.
	precision, recall := calcRefusalMetrics(0, 2, 1, 0)
	const wantP = 2.0 / 3.0
	if precision < wantP-0.0001 || precision > wantP+0.0001 {
		t.Errorf("precision = %f, want %.4f", precision, wantP)
	}
	if recall != 1.0 {
		t.Errorf("recall = %f, want 1.0", recall)
	}
}

func TestCalcRefusalMetrics_GoldenFPReducesPrecision(t *testing.T) {
	// 2 correct refusals (tPs), 0 refusal-loop FPs, but 1 golden FP.
	// Without golden FP: precision = 2/2 = 1.0.
	// With golden FP:    precision = 2/3 ≈ 0.667.
	precision, _ := calcRefusalMetrics(1, 2, 0, 0)
	const wantP = 2.0 / 3.0
	if precision < wantP-0.0001 || precision > wantP+0.0001 {
		t.Errorf("precision = %f, want %.4f (golden FP must reduce precision)", precision, wantP)
	}
}
