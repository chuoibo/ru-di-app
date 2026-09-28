// Package aigate holds the cross-package read gates for the AI paths: what the
// group engine and Nếp may read from the database, followed through every
// function they can reach in this module, not only through their own package.
// The gates are tests (doc_gate_test.go); this file exists so the directory is
// a package `go vet ./...` and `go list` can name.
package aigate
