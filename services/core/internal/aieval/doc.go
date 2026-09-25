// Package aieval is the AI engine's measuring kit (design 06): it runs a turn
// through the same seam the worker calls, aiharness.Engine.Run, with a Sink
// that records what the turn emitted, and holds every request the model was
// handed to the invariants of design 06 §6.1.
//
// This slice (6b) builds tier T1 for Nếp at S1: the model is the scripted
// stub of aiharness/llm (stub.go), driven by hand-written scripts under
// testdata/kich_ban, over a hand-written corpus under testdata/corpus. T1
// proves the pipeline -- the invariants hold on every request and every Sink
// event, every script expectation is met, the canary case is red exactly where
// predicted and the identity case is green. It proves nothing about answer
// quality: the answers are the script author's words.
//
// Files, as design 06 §2 names them:
//
//   - ghi_lai.go: the recording Sink;
//   - bat_bien.go: the invariants of §6.1 that apply to Nếp at S1 (1, 2, 3,
//     7, 8); 10 is the binary's and is held by cmd/rudi-eval's tests and by
//     TestKhongDungClientGenai here;
//   - gieo.go: what a run is seeded with -- the turn exactly as the worker
//     builds it from a stored job, a deterministic id and canary marker;
//   - hang.go: the engine's numbers and closed sets, read from the engine;
//   - ca.go, kich_ban.go: the corpus and script schemas, decoded strictly;
//   - cham.go: the script expectations, one named check each;
//   - chay.go: one case, and a whole corpus with its verdict.
//
// Not here yet, each with the slice design 06 §13 gives it: the cassette
// (bang_ghi.go) and the trace plugin (vet.go), slice 18; the Postgres half of
// gieo.go, the first slice whose engine reads the database (the Nếp engine of
// S1 takes no pool, so a seeded database would be unreachable and any check on
// it vacuous); invariants 4, 5, 6 and 9, which need memory, a catalogue, the
// Understand step and the group card; and the Python runner and scorers
// (tests/evals, cham/*.py), slice 9.
//
// Nothing in this package builds a model client: the only model it knows is
// llm.Stub. cmd/core never imports it (aigate's TestCoreBinaryDoesNotLinkEval
// and scripts/eval_kich_ban.sh hold that line).
package aieval
