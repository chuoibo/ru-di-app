// Package pyval validates requests for a Go-served route exactly as the
// Python API (FastAPI 0.115.6, pydantic 2.13.5, pydantic-core 2.46.5,
// Starlette 0.41.3) validates them: a bad request yields the same error list,
// in the same order, with the same type, loc, msg and ctx; a good request
// yields the values pydantic would have handed the endpoint.
//
// It interprets the contract IR (services/core/contract, rendered by
// scripts/render_contract_ir.py from the pinned image), which carries
// FastAPI's dependency tree and pydantic-core's own schema for each
// parameter. The IR is embedded rather than read at run time, so a binary
// can only validate against the contract it was built with.
//
// # Rules read from the library source in the image and measured there
//
//   - The body is read only when the route has a body parameter. An empty
//     body is "no body". Otherwise it is json.loads-ed when content-type is
//     absent or empty, or when email.message parses it (text before ";",
//     str.strip, lowercase, exactly one "/") as application/json or
//     application/*+json; any other content-type leaves the raw bytes, which
//     a model refuses with model_attributes_type. A JSON null body is no
//     body.
//   - JSONDecodeError is a 422 json_invalid, loc ["body", <code point
//     offset>], ctx.error the decoder message, raised before any dependency
//     runs: an anonymous caller sending broken JSON gets 422, not 401. Every
//     other decoding failure is a 400 (BodyError).
//   - solve_dependencies runs sub-dependencies first, depth first. A
//     dependency's own parameters are validated; when they have no error the
//     dependency is called, and an exception it raises (401 from get_actor)
//     ends the request before the endpoint's own parameters are looked at.
//     Errors then accumulate in the order path, query, header, cookie, body.
//     A cached dependency's parameters are validated again, so their errors
//     repeat.
//   - A missing required parameter is "missing" at (in, alias); a missing
//     body is "missing" at ["body"]. A scalar query parameter takes the last
//     value of a repeated key, a scalar header the first; sequence
//     parameters take all, and an empty list counts as absent. Header
//     aliases match case-insensitively; header values and the query string
//     are latin-1 decoded, query items percent-decoded as UTF-8 with
//     replacement.
//   - Validation is TypeAdapter.validate_python(value, from_attributes=True):
//     Python-mode lax/strict coercion over what json.loads produced, not
//     pydantic's JSON mode. Model fields are checked in declaration order,
//     then extra keys in input order. String lengths count code points; a
//     length or pattern constraint makes a string with a lone surrogate fail
//     string_unicode. UUIDs follow the uuid crate: 32 hex digits, the
//     hyphenated form, braced, or after a lowercase "urn:uuid:".
//   - Lax numbers from strings trim Rust White_Space (not U+001C..U+001F).
//     An int accepts a non-empty all-zero fraction and "_" only between
//     digits; a float refuses only a leading or trailing "_" or "__". A
//     float into bool is bool_type unless it is an integral i64.
//   - Dates and datetimes follow speedate: RFC 3339 text, or a unix
//     timestamp (milliseconds above 2e10). When one fails in lax mode the
//     other is tried (a date from a midnight datetime, a datetime from a
//     date) and the fallback's parse error is the one reported.
//
// # Form and File bodies
//
// Measured against python-multipart 0.0.20 in the same image
// (form.go, multipart.go, options_header.go, codecs.go):
//
//   - A route whose body field is Form or File always reads the body with
//     request.form(), empty or not, and never as JSON. Only the first
//     Content-Type header counts. Without ";" it is lowercased and stripped;
//     with ";" email.message decides, and the type keeps its case, so
//     "Application/X-WWW-Form-Urlencoded; charset=utf-8" is no form. Any
//     type other than multipart/form-data or
//     application/x-www-form-urlencoded, JSON included, is an empty form:
//     every field is missing. The charset parameter of an urlencoded body
//     is ignored.
//   - Urlencoded: "&" and ";" separate; "+" is a space; names and values are
//     latin-1 decoded, then percent escapes (only) decoded as UTF-8 with
//     replacement, so raw UTF-8 bytes arrive as latin-1 mojibake; an
//     invalid escape stays as typed; a field without "=" has the value ""
//     unless it ends the body, where it is dropped. A repeated name takes
//     its last value. There is no size limit.
//   - Multipart: Starlette's refusals are 400 with their own detail
//     ("Missing boundary in multipart.", a part without a name, a part over
//     1024 KB without a filename, more than 1000 files or fields); a parse
//     error, or header parameters that raise, is the generic 400. A part
//     with a filename parameter is an UploadFile, whatever the field's
//     type; text is decoded with the content type's charset, latin-1 when
//     that fails or names no text codec.
//   - Form fields: FastAPI treats "" as missing for a Form or File
//     parameter, then puts every form name it did not take back into the
//     dict, so a required str field sent empty is "" after all, and an
//     optional one gets its default only when that default is not None.
//     Fields of a model declared as one Form parameter get no such rule.
//     Errors are at ["body", name], after path, query and header errors;
//     extra fields of a forbidding model come after its fields, in form
//     order.
//
// Validator functions written in Python (field_validator, model_validator)
// cannot be generated; they are ported by hand into a Registry under the
// name the IR gives them, and Contract.Bind refuses a route whose schemas
// call a function the registry does not hold, or use an IR feature this
// package does not implement.
package pyval
