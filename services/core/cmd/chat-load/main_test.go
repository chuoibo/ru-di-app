package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestSequenceWitnessRejectsConflictingDeliveries(t *testing.T) {
	w := &world{}
	for i := 0; i < 500; i++ {
		if !w.recordSequence("group-a", 1, "send-a") {
			t.Fatal("identical delivery rejected")
		}
	}
	if w.recordSequence("group-a", 1, "send-b") {
		t.Fatal("different ciphertext identity accepted at the same sequence")
	}
	if w.recordSequence("group-a", 2, "send-a") {
		t.Fatal("same ciphertext identity accepted at a new sequence")
	}
	if w.recordSequence("group-b", 1, "send-a") {
		t.Fatal("same ciphertext identity accepted in another conversation")
	}
}

func TestSequenceWitnessConcurrentFirstObservation(t *testing.T) {
	w := &world{}
	start := make(chan struct{})
	var wg sync.WaitGroup
	var accepted [2]atomic.Int64
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if w.recordSequence("group", 7, []string{"send-a", "send-b"}[i%2]) {
				accepted[i%2].Add(1)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if (accepted[0].Load() == 250) == (accepted[1].Load() == 250) || accepted[0].Load()+accepted[1].Load() != 250 {
		t.Fatalf("conflicting deliveries were not consistently rejected: %d, %d", accepted[0].Load(), accepted[1].Load())
	}
}
