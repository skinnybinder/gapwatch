package main

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunBasicGap(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- run(pr, &out, 0, ".", 50*time.Millisecond, 0, 0, false)
	}()

	pw.Write([]byte("line1\n"))
	pw.Write([]byte("line2\n"))
	time.Sleep(200 * time.Millisecond)
	pw.Write([]byte("line3\n"))
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "line1") {
		t.Error("missing line1")
	}
	if !strings.Contains(output, "line2") {
		t.Error("missing line2")
	}
	if !strings.Contains(output, "line3") {
		t.Error("missing line3")
	}
	if !strings.Contains(output, ".") {
		t.Error("expected gap markers between line2 and line3")
	}
}

func TestRunNoGap(t *testing.T) {
	input := "line1\nline2\nline3\n"
	var out bytes.Buffer

	err := run(strings.NewReader(input), &out, 0, ".", 500*time.Millisecond, 0, 0, false)
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d: %q", len(lines), output)
	}
	if strings.Contains(output, ".") {
		t.Error("no gap markers expected when input is continuous")
	}
}

func TestRunStartDelay(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		// start-delay=150ms: no gap detection during warm-up
		// interval=30ms: markers after warm-up
		done <- run(pr, &out, 150*time.Millisecond, ".", 30*time.Millisecond, 0, 0, false)
	}()

	pw.Write([]byte("line1\n"))
	// silence starts immediately; warm-up is 150ms
	// gap detection begins at 150ms, first marker at 180ms
	time.Sleep(250 * time.Millisecond)
	pw.Write([]byte("line2\n"))
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, ".") {
		t.Error("expected gap markers after warm-up period")
	}
	// markers should appear after warm-up, not before
	idx1 := strings.Index(output, "line1")
	idxDot := strings.Index(output, ".")
	idxLine2 := strings.Index(output, "line2")
	if idxDot < idx1 {
		t.Error("marker appeared before line1")
	}
	if idxDot > idxLine2 {
		t.Error("marker appeared after line2")
	}
}

func TestRunMax(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- run(pr, &out, 0, ".", 30*time.Millisecond, 0, 2, false)
	}()

	pw.Write([]byte("hello\n"))
	time.Sleep(200 * time.Millisecond)
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	dotCount := strings.Count(out.String(), ".\n")
	if dotCount > 2 {
		t.Errorf("expected at most 2 gap markers, got %d", dotCount)
	}
}

func TestRunMaxGlobal(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		// max=2 is global: two gaps should not each get 2 markers
		done <- run(pr, &out, 0, ".", 30*time.Millisecond, 0, 2, false)
	}()

	pw.Write([]byte("line1\n"))
	time.Sleep(100 * time.Millisecond) // first gap: uses up to 2 markers
	pw.Write([]byte("line2\n"))
	time.Sleep(100 * time.Millisecond) // second gap: max already reached
	pw.Write([]byte("line3\n"))
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	dotCount := strings.Count(out.String(), ".\n")
	if dotCount > 2 {
		t.Errorf("expected max=2 to be global across gaps, got %d markers", dotCount)
	}
}

func TestRunFold(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- run(pr, &out, 0, ".", 20*time.Millisecond, 3, 0, false)
	}()

	pw.Write([]byte("hello\n"))
	time.Sleep(400 * time.Millisecond)
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	dotCount := strings.Count(out.String(), ".\n")
	if dotCount == 0 {
		t.Error("expected gap markers with fold=3")
	}
	// ~400ms / 20ms = ~20 ticks; fold=3 → ~6 markers; assert well below unfold count
	if dotCount > 10 {
		t.Errorf("expected fold=3 to suppress markers, got %d", dotCount)
	}
}

func TestRunFoldResetsPerGap(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		// fold=3, interval=20ms: marker at tick 3, 6, ...
		// after a line arrives tickCount resets, so next gap also starts at tick 1
		done <- run(pr, &out, 0, ".", 20*time.Millisecond, 3, 0, false)
	}()

	pw.Write([]byte("line1\n"))
	time.Sleep(70 * time.Millisecond) // ~3 ticks → 1 marker (tick 3)
	pw.Write([]byte("line2\n"))
	time.Sleep(70 * time.Millisecond) // tickCount resets → again 1 marker at tick 3
	pw.Write([]byte("line3\n"))
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	dotCount := strings.Count(out.String(), ".\n")
	// two gaps of ~70ms at fold=3, interval=20ms → ~1 marker each = 2 total
	// allow variance: 1–4
	if dotCount == 0 {
		t.Error("expected gap markers in both gaps")
	}
	if dotCount > 4 {
		t.Errorf("expected ~2 markers across two gaps with fold=3, got %d", dotCount)
	}
}

func TestRunFoldWithMax(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- run(pr, &out, 0, ".", 20*time.Millisecond, 3, 2, false)
	}()

	pw.Write([]byte("hello\n"))
	time.Sleep(400 * time.Millisecond)
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	dotCount := strings.Count(out.String(), ".\n")
	if dotCount > 2 {
		t.Errorf("expected at most 2 markers with fold=3 max=2, got %d", dotCount)
	}
}

func TestRunCustomMarker(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- run(pr, &out, 0, "---", 30*time.Millisecond, 0, 0, false)
	}()

	pw.Write([]byte("hello\n"))
	time.Sleep(100 * time.Millisecond)
	pw.Close()

	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "---") {
		t.Error("expected custom marker '---' in output")
	}
}

func TestRunEmptyInput(t *testing.T) {
	var out bytes.Buffer
	err := run(strings.NewReader(""), &out, 0, ".", 50*time.Millisecond, 0, 0, false)
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.String() != "" {
		t.Errorf("expected empty output, got %q", out.String())
	}
}
