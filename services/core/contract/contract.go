// Package contract carries the request contract of the Python API as JSON IR,
// one file per router group under ir/, rendered from the pinned API image by
// scripts/render_contract_ir.py. It is embedded for the reason
// services/core/ownership embeds routes.json: a binary can never validate
// requests against a contract it was not built with.
package contract

import "embed"

// IR holds ir/*.json.
//
//go:embed ir/*.json
var IR embed.FS
