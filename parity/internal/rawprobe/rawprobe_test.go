package rawprobe

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
)

// rawServer answers every connection with a fixed response chosen by target.
func rawServer(t *testing.T, answer func(requestLine string) string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				buf := make([]byte, 4096)
				n, _ := conn.Read(buf)
				line, _, _ := strings.Cut(string(buf[:n]), "\r\n")
				_, _ = io.WriteString(conn, answer(line))
			}(conn)
		}
	}()
	return "http://" + listener.Addr().String()
}

const ok = "HTTP/1.1 200 OK\r\ndate: x\r\nserver: y\r\ncontent-type: application/json\r\ncontent-length: 15\r\n\r\n{\"status\":\"ok\"}"
const notFound = "HTTP/1.1 404 Not Found\r\ncontent-type: application/json\r\ncontent-length: 22\r\n\r\n{\"detail\":\"Not Found\"}"

func TestIdenticalServersHaveNoDivergence(t *testing.T) {
	answer := func(string) string { return ok }
	results, err := Run(context.Background(), rawServer(t, answer), rawServer(t, answer))
	if err != nil {
		t.Fatal(err)
	}
	unexpected, stale := Verdict(results, map[string]string{})
	if len(unexpected) != 0 || len(stale) != 0 {
		t.Fatalf("unexpected=%v stale=%v", unexpected, stale)
	}
}

func TestVerdictCatchesNewAndStaleDivergence(t *testing.T) {
	reference := rawServer(t, func(string) string { return ok })
	candidate := rawServer(t, func(line string) string {
		if strings.Contains(line, "%zz") {
			return notFound
		}
		return ok
	})
	results, err := Run(context.Background(), reference, candidate)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"bad-escape-root": "accepted", "fragment": "accepted but no longer differs"}
	unexpected, stale := Verdict(results, expected)
	// %zz appears in three cases; one is accepted, the others are new.
	if strings.Join(unexpected, ",") != "bad-escape-query,bad-escape-route,bad-escape-suffix" {
		t.Fatalf("unexpected = %v", unexpected)
	}
	if strings.Join(stale, ",") != "fragment" {
		t.Fatalf("stale = %v", stale)
	}
}

func TestVolatileHeadersAndReasonPhraseAreIgnored(t *testing.T) {
	a := parse([]byte("HTTP/1.1 400 Bad Request\r\nDate: a\r\nConnection: close\r\ncontent-length: 2\r\n\r\nno"))
	b := parse([]byte("HTTP/1.1 400 Something Else\r\nServer: b\r\ncontent-length: 2\r\n\r\nno"))
	if !Equal(a, b) {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
	c := parse([]byte("HTTP/1.1 400 Bad Request\r\ncontent-length: 2\r\n\r\nyo"))
	if Equal(a, c) {
		t.Fatal("different bodies compared equal")
	}
}

func TestExpectedDivergenceNamesRealCases(t *testing.T) {
	names := map[string]bool{}
	for _, c := range Cases {
		if names[c.Name] {
			t.Fatalf("duplicate case %s", c.Name)
		}
		names[c.Name] = true
	}
	for name := range ExpectedDivergence {
		if !names[name] {
			t.Fatalf("ExpectedDivergence names %q, which is not a case", name)
		}
	}
	// 10 since 2026-09-20: the `fragment` line left the list because
	// router/pystr.go, the port of Python's parse_url, cuts the path at "#" and
	// drops the fragment, so core answers 200 exactly as uvicorn does. It had
	// been right since 71156526 (W0); the probe only said so once a gate run
	// finally reached the probe stage. The count is pinned on purpose: the list
	// must not grow, or shrink, without the ADR moving with it.
	if len(ExpectedDivergence) != 10 {
		t.Fatalf("ADR-0029 §2.4 lists 10 accepted request lines, code lists %d", len(ExpectedDivergence))
	}
}
