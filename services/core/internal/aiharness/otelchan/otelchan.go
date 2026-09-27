// Package otelchan keeps ADK's message-content capture off in every process
// that links the engine (ADR-0044 §2.8).
//
// ADK v2 records whole prompts and answers on spans and log records when
// OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT says so
// (adk/v2/internal/telemetry/logger.go). It reads the variable once, lazily,
// on its first traced call. The core installs no provider, so today nothing
// would leave the process either way; clearing the variable before ADK can
// read it means that a provider turning up later, or an auto-instrumentation
// agent attaching from outside, still gets no content from ADK's
// content-gated attributes. Tool arguments and results that ADK puts on spans
// unconditionally are the provider gate's business (aigate/otel_gate_test.go).
//
// The engine's packages that reach ADK (llm, agent) import this one for its
// init, which runs before any of theirs and so before any ADK call.
package otelchan

import "os"

// BienNoiDung is the variable ADK reads to decide whether to capture content.
const BienNoiDung = "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT"

func init() { Chan() }

// Chan clears BienNoiDung from the process environment.
func Chan() { _ = os.Unsetenv(BienNoiDung) }
