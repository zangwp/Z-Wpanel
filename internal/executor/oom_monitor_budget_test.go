package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

type oomTestNoiseReader struct{ bytes int }

func (r *oomTestNoiseReader) Read(p []byte) (int, error) {
	for i := range p {
		if i%8 == 7 {
			p[i] = '\n'
		} else {
			p[i] = 'x'
		}
	}
	r.bytes += len(p)
	return len(p), nil
}

func TestOOMJournalStreamingBudgetsPreservePartialEvidence(t *testing.T) {
	line := "1785067200.125000 host kernel: Out of memory: Killed process 4321 (mariadbd) total-vm:123kB\n"
	events, incomplete, err := scanOOMJournal(strings.NewReader(line + "irrelevant\n"))
	if err != nil || incomplete || len(events) != 1 {
		t.Fatalf("normal events=%v incomplete=%v err=%v", events, incomplete, err)
	}
	noise := &oomTestNoiseReader{}
	events, incomplete, err = scanOOMJournal(io.MultiReader(strings.NewReader(line), noise))
	if err != nil || !incomplete || len(events) != 1 || noise.bytes > oomJournalByteLimit+1 {
		t.Fatalf("byte budget events=%d partial=%v bytes=%d err=%v", len(events), incomplete, noise.bytes, err)
	}
	events, incomplete, err = scanOOMJournal(strings.NewReader(line + strings.Repeat("x", oomJournalLineLimit+1)))
	if err != nil || !incomplete || len(events) != 1 {
		t.Fatalf("line budget events=%d partial=%v err=%v", len(events), incomplete, err)
	}
	events, incomplete, err = scanOOMJournal(strings.NewReader(strings.Repeat(line, oomJournalEventLimit+1)))
	if err != nil || !incomplete || len(events) != oomJournalEventLimit {
		t.Fatalf("event budget events=%d partial=%v err=%v", len(events), incomplete, err)
	}
}

func TestOOMJournalProcessFixture(t *testing.T) {
	if os.Getenv("YUB_TEST_OOM_CHILD") != "1" {
		return
	}
	fmt.Println("1785067200.125000 host kernel: Out of memory: Killed process 4321 (mariadbd) total-vm:123kB")
	if os.Getenv("YUB_TEST_OOM_MODE") == "hang" {
		time.Sleep(time.Minute)
	}
	if os.Getenv("YUB_TEST_OOM_MODE") == "large" {
		_, _ = io.CopyN(os.Stdout, &oomTestNoiseReader{}, oomJournalByteLimit*4)
	}
	os.Exit(0)
}

func TestOOMJournalChildIsWaitedOnCancellationAndBudget(t *testing.T) {
	for _, mode := range []string{"complete", "hang", "large"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("YUB_TEST_OOM_CHILD", "1")
			t.Setenv("YUB_TEST_OOM_MODE", mode)
			timeout := 5 * time.Second
			if mode == "hang" {
				timeout = time.Second
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			started := time.Now()
			events, incomplete, err := readOOMJournalCommand(ctx, os.Args[0], "-test.run=^TestOOMJournalProcessFixture$")
			if time.Since(started) > 6*time.Second {
				t.Fatal("journal child did not end promptly")
			}
			if mode == "complete" {
				if err != nil || incomplete || len(events) != 1 {
					t.Fatalf("complete: %v %v %v", events, incomplete, err)
				}
				return
			}
			if !incomplete || len(events) != 1 {
				t.Fatalf("partial events=%d incomplete=%v err=%v", len(events), incomplete, err)
			}
			if mode == "hang" && err == nil {
				t.Fatal("cancelled journal was reported complete")
			}
		})
	}
}
