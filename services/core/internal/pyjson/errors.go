package pyjson

import (
	"errors"
	"fmt"
)

// IntMaxStrDigits is CPython's default sys.get_int_max_str_digits(): int to
// str and str to int conversions refuse numbers with more decimal digits.
const IntMaxStrDigits = 4300

// MaxDepth is the deepest container nesting accepted before RecursionError.
//
// CPython 3.12.14 counts C recursion against a limit of 10000, so the real
// limit is 10000 minus the C frames already on the stack when json runs.
// Measured in the parity image under uvicorn (uvloop, httptools):
//
//	9984  Request.json() in a FastAPI route, and the idempotency middleware's
//	      json.loads + canonical json.dumps
//	9981  JSONResponse(...) rendered inside an async route handler
//	9997  json.loads / json.dumps called at the top level of `python -c`
//
// MaxDepth uses the request-body value. It is exact only for those call
// sites and may drift by a few levels if the Python middleware stack changes.
const MaxDepth = 9984

// DecodeError mirrors json.JSONDecodeError. Msg and Pos equal CPython's
// .msg and .pos; Pos, Line and Col count code points of the decoded
// document, not bytes of the input.
type DecodeError struct {
	Msg  string
	Pos  int
	Line int
	Col  int
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("%s: line %d column %d (char %d)", e.Msg, e.Line, e.Col, e.Pos)
}

// ValueError mirrors a plain Python ValueError: a float outside JSON's range
// with allow_nan=False, or an int beyond IntMaxStrDigits. Msg is CPython's
// message text.
type ValueError struct {
	Msg string
}

func (e *ValueError) Error() string { return e.Msg }

// UnicodeDecodeError mirrors the UnicodeDecodeError raised when json.loads
// decodes bytes in the encoding json.detect_encoding picked. Offset is the
// byte offset into the input. The message is not CPython's text.
type UnicodeDecodeError struct {
	Encoding string
	Offset   int
	Reason   string
}

func (e *UnicodeDecodeError) Error() string {
	return fmt.Sprintf("'%s' codec can't decode input at byte %d: %s", e.Encoding, e.Offset, e.Reason)
}

// UnicodeEncodeError mirrors str.encode("utf-8") refusing surrogate code
// points in an encoded document. Start and End are the code point range of
// the first run of surrogates in that document; Error() is CPython's text.
type UnicodeEncodeError struct {
	Start int
	End   int
	First rune
}

func (e *UnicodeEncodeError) Error() string {
	if e.End-e.Start == 1 {
		return fmt.Sprintf("'utf-8' codec can't encode character '\\u%04x' in position %d: surrogates not allowed", e.First, e.Start)
	}
	return fmt.Sprintf("'utf-8' codec can't encode characters in position %d-%d: surrogates not allowed", e.Start, e.End-1)
}

// RecursionError mirrors RecursionError from the C json accelerators.
type RecursionError struct {
	Msg string
}

func (e *RecursionError) Error() string { return e.Msg }

// InvalidStringError reports a Go string that is not generalized UTF-8 and so
// has no Python str counterpart. Python itself can never raise this.
type InvalidStringError struct {
	Offset int
}

func (e *InvalidStringError) Error() string {
	return fmt.Sprintf("pyjson: string is not valid UTF-8 at byte %d", e.Offset)
}

// IsValueError reports whether Python would have raised a ValueError
// subclass (JSONDecodeError, UnicodeDecodeError, UnicodeEncodeError or a
// plain ValueError). `except ValueError` catches exactly these; a
// RecursionError is not one.
func IsValueError(err error) bool {
	var (
		de *DecodeError
		ve *ValueError
		ud *UnicodeDecodeError
		ue *UnicodeEncodeError
	)
	return errors.As(err, &de) || errors.As(err, &ve) || errors.As(err, &ud) || errors.As(err, &ue)
}
