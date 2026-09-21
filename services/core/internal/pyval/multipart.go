package pyval

import (
	"bytes"
	"errors"
	"strconv"

	"mobile/services/core/internal/pyjson"
)

// A multipart/form-data body is read by Starlette's MultiPartParser
// (starlette/formparsers.py:124-271) driving python-multipart's streaming
// MultipartParser (python_multipart/multipart.py:956-1466). Both are ported
// statement by statement, fed the way Request.stream() feeds them for a body
// that arrived in one ASGI message: the whole body (when not empty), then an
// empty chunk. The part size and count limits count cumulatively, so they do
// not depend on how a server chunks the body.
//
// Starlette's own refusals (MultiPartException) become HTTPException(400,
// message) inside Request.form() (starlette/requests.py:260-272) and reach
// the client with that message. A MultipartParseError, or any other
// exception, escapes to get_request_handler (fastapi/routing.py:284-288) and
// becomes the generic 400.

const (
	multipartMaxPartSize = 1024 * 1024
	multipartMaxFiles    = 1000
	multipartMaxFields   = 1000
)

// errMultipartParse is a python_multipart MultipartParseError, which FastAPI
// answers with its generic 400.
var errMultipartParse error = &BodyError{Err: errors.New("pyval: MultipartParseError")}

// multiPartException is starlette.formparsers.MultiPartException.
func multiPartException(message string) error {
	return &BodyError{Err: errors.New(message), Detail: message}
}

type multipartPart struct {
	contentDisposition []byte
	fieldName          string
	data               []byte
	file               *UploadFile
	itemHeaders        [][2]string
}

// multipartReader is starlette.formparsers.MultiPartParser together with
// the python_multipart.MultipartParser it drives.
type multipartReader struct {
	charset            string
	items              []formItem
	currentFiles       int
	currentFields      int
	partialHeaderName  []byte
	partialHeaderValue []byte
	current            *multipartPart

	parserState, index, flags int
	boundary                  []byte
	marks                     map[string]int
}

// parseMultipart is MultiPartParser.parse for the request's content type
// (first header value, latin-1) and body.
func parseMultipart(contentType string, body []byte) (*formData, error) {
	_, params, err := parseOptionsHeader(contentType)
	if err != nil {
		return nil, &BodyError{Err: err}
	}
	charset, ok := params["charset"]
	if !ok {
		charset = "utf-8"
	}
	boundary, ok := params["boundary"]
	if !ok {
		return nil, multiPartException("Missing boundary in multipart.")
	}
	m := &multipartReader{
		charset:  charset,
		current:  &multipartPart{},
		boundary: append([]byte("\r\n--"), boundary...),
		marks:    map[string]int{},
	}
	if len(body) > 0 {
		if err := m.write(body); err != nil {
			return nil, err
		}
	}
	if err := m.write([]byte{}); err != nil {
		return nil, err
	}
	return &formData{items: m.items}, nil
}

// ---- Starlette callbacks ----

// data is BaseParser.callback for a data callback: an empty range is not
// reported, and the slice clamps as Python's does.
func (m *multipartReader) data(name string, data []byte, start, end int) error {
	start, end = min(start, len(data)), min(end, len(data))
	if start >= end {
		return nil
	}
	chunk := data[start:end]
	switch name {
	case "part_data":
		if m.current.file == nil {
			if len(m.current.data)+len(chunk) > multipartMaxPartSize {
				return multiPartException("Part exceeded maximum size of " + strconv.Itoa(multipartMaxPartSize/1024) + "KB.")
			}
			m.current.data = append(m.current.data, chunk...)
			return nil
		}
		m.current.file.Content = append(m.current.file.Content, chunk...)
	case "header_field":
		m.partialHeaderName = append(m.partialHeaderName, chunk...)
	case "header_value":
		m.partialHeaderValue = append(m.partialHeaderValue, chunk...)
	}
	return nil
}

// notify is BaseParser.callback for a notification callback. header_begin
// and end are not set or do nothing.
func (m *multipartReader) notify(name string) error {
	switch name {
	case "part_begin":
		m.current = &multipartPart{}
	case "part_end":
		if m.current.file != nil {
			m.items = append(m.items, formItem{key: m.current.fieldName, value: m.current.file})
			return nil
		}
		value, err := userSafeDecode(m.current.data, m.charset)
		if err != nil {
			return err
		}
		m.items = append(m.items, formItem{key: m.current.fieldName, value: pyjson.String(value)})
	case "header_end":
		field := string(bytes.ToLower(m.partialHeaderName)) // ASCII only, as bytes.lower
		value := string(m.partialHeaderValue)
		if field == "content-disposition" {
			m.current.contentDisposition = []byte(value)
		}
		m.current.itemHeaders = append(m.current.itemHeaders, [2]string{field, value})
		m.partialHeaderName, m.partialHeaderValue = nil, nil
	case "headers_finished":
		return m.headersFinished()
	}
	return nil
}

func (m *multipartReader) headersFinished() error {
	_, options, err := parseOptionsHeader(string(m.current.contentDisposition))
	if err != nil {
		return &BodyError{Err: err}
	}
	name, ok := options["name"]
	if !ok {
		return multiPartException(`The Content-Disposition header field "name" must be provided.`)
	}
	if m.current.fieldName, err = userSafeDecode([]byte(name), m.charset); err != nil {
		return err
	}
	if raw, isFile := options["filename"]; isFile {
		m.currentFiles++
		if m.currentFiles > multipartMaxFiles {
			return multiPartException("Too many files. Maximum number of files is " + strconv.Itoa(multipartMaxFiles) + ".")
		}
		filename, err := userSafeDecode([]byte(raw), m.charset)
		if err != nil {
			return err
		}
		m.current.file = &UploadFile{Filename: filename, Headers: m.current.itemHeaders, Content: []byte{}}
		return nil
	}
	m.currentFields++
	if m.currentFields > multipartMaxFields {
		return multiPartException("Too many fields. Maximum number of fields is " + strconv.Itoa(multipartMaxFields) + ".")
	}
	m.current.file = nil
	return nil
}

// ---- python_multipart.MultipartParser ----

const (
	mpStart = iota
	mpStartBoundary
	mpHeaderFieldStart
	mpHeaderField
	mpHeaderValueStart
	mpHeaderValue
	mpHeaderValueAlmostDone
	mpHeadersAlmostDone
	mpPartDataStart
	mpPartData
	mpEndBoundary
	mpEnd
)

const (
	flagPartBoundary = 1
	flagLastBoundary = 2
)

// isTokenChar is membership in TOKEN_CHARS_SET (RFC 7230 tchar).
func isTokenChar(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	}
	switch c {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	}
	return false
}

// write is MultipartParser.write (no size limit) and _internal_write.
func (m *multipartReader) write(data []byte) error {
	length := len(data)
	boundary := m.boundary
	state, index, flags := m.parserState, m.index, m.flags
	i := 0

	setMark := func(name string) { m.marks[name] = i }
	dataCallback := func(name string, endI int, remaining bool) error {
		marked, ok := m.marks[name]
		if !ok {
			return nil
		}
		switch {
		case endI <= marked:
		case marked >= 0:
			if err := m.data(name, data, marked, endI); err != nil {
				return err
			}
		default:
			// Part of the data is a partial boundary match from an earlier
			// chunk; m.flags is the flags as they were on entry.
			lookbehind := -marked
			var err error
			switch {
			case lookbehind <= len(boundary):
				err = m.data(name, boundary, 0, lookbehind)
			case m.flags&flagPartBoundary != 0:
				err = m.data(name, append(append([]byte(nil), boundary...), "\r\n"...), 0, lookbehind)
			case m.flags&flagLastBoundary != 0:
				err = m.data(name, append(append([]byte(nil), boundary...), "--\r\n"...), 0, lookbehind)
			}
			if err != nil {
				return err
			}
			if endI > 0 {
				if err := m.data(name, data, 0, endI); err != nil {
					return err
				}
			}
		}
		if remaining {
			m.marks[name] = endI - length
		} else {
			delete(m.marks, name)
		}
		return nil
	}

loop:
	for i < length {
		c := data[i]
		switch state {
		case mpStart:
			// Skip leading newlines.
			if c == '\r' || c == '\n' {
				i++
				continue
			}
			index = 0
			state = mpStartBoundary
			i--

		case mpStartBoundary:
			switch index {
			case len(boundary) - 2:
				if c == '-' {
					state = mpEndBoundary // a potential empty message
				} else if c != '\r' {
					return errMultipartParse
				}
				index++
			case len(boundary) - 2 + 1:
				if c != '\n' {
					return errMultipartParse
				}
				index = 0
				if err := m.notify("part_begin"); err != nil {
					return err
				}
				state = mpHeaderFieldStart
			default:
				if c != boundary[index+2] {
					return errMultipartParse
				}
				index++
			}

		case mpHeaderFieldStart:
			index = 0
			setMark("header_field")
			state = mpHeaderField
			i--

		case mpHeaderField:
			if c == '\r' && index == 0 {
				delete(m.marks, "header_field")
				state = mpHeadersAlmostDone
				i++
				continue
			}
			index++
			if c == ':' {
				if index == 1 {
					return errMultipartParse // a 0-length header
				}
				if err := dataCallback("header_field", i, false); err != nil {
					return err
				}
				state = mpHeaderValueStart
			} else if !isTokenChar(c) {
				return errMultipartParse
			}

		case mpHeaderValueStart:
			if c == ' ' {
				i++
				continue
			}
			setMark("header_value")
			state = mpHeaderValue
			i--

		case mpHeaderValue:
			if c == '\r' {
				if err := dataCallback("header_value", i, false); err != nil {
					return err
				}
				if err := m.notify("header_end"); err != nil {
					return err
				}
				state = mpHeaderValueAlmostDone
			}

		case mpHeaderValueAlmostDone:
			if c != '\n' {
				return errMultipartParse
			}
			state = mpHeaderFieldStart

		case mpHeadersAlmostDone:
			if c != '\n' {
				return errMultipartParse
			}
			if err := m.notify("headers_finished"); err != nil {
				return err
			}
			state = mpPartDataStart

		case mpPartDataStart:
			setMark("part_data")
			state = mpPartData
			i--

		case mpPartData:
			prevIndex := index
			boundaryLength := len(boundary)
			if index == 0 {
				if rel := bytes.Index(data[i:length], boundary); rel >= 0 {
					index = boundaryLength - 1
					i = i + rel + boundaryLength - 1
				} else {
					i = max(i, length-boundaryLength)
					for i < length-1 && data[i] != boundary[0] {
						i++
					}
				}
				c = data[i]
			}
			switch {
			case index < boundaryLength:
				if boundary[index] == c {
					index++
				} else {
					index = 0
				}
			case index == boundaryLength:
				index++
				switch c {
				case '\r':
					flags |= flagPartBoundary
				case '-':
					flags |= flagLastBoundary
				default:
					index = 0
				}
			case index == boundaryLength+1:
				if flags&flagPartBoundary != 0 {
					if c == '\n' {
						flags &^= flagPartBoundary
						if err := dataCallback("part_data", i-index, false); err != nil {
							return err
						}
						if err := m.notify("part_end"); err != nil {
							return err
						}
						if err := m.notify("part_begin"); err != nil {
							return err
						}
						index = 0
						state = mpHeaderFieldStart
						i++
						continue
					}
					index = 0
					flags &^= flagPartBoundary
				} else if flags&flagLastBoundary != 0 {
					if c == '-' {
						if err := dataCallback("part_data", i-index, false); err != nil {
							return err
						}
						if err := m.notify("part_end"); err != nil {
							return err
						}
						state = mpEnd
					} else {
						index = 0
					}
				}
			}
			if index == 0 && prevIndex > 0 {
				// Reconsider this byte: it may start the boundary itself.
				i--
			}

		case mpEndBoundary:
			if index == len(boundary)-2+1 {
				if c != '-' {
					return errMultipartParse
				}
				index++
				state = mpEnd
			}

		case mpEnd:
			if c == '\r' && i+1 < length && data[i+1] == '\n' {
				i += 2
				continue
			}
			// Data after the last boundary is skipped.
			break loop
		}
		i++
	}

	if err := dataCallback("header_field", length, true); err != nil {
		return err
	}
	if err := dataCallback("header_value", length, true); err != nil {
		return err
	}
	if err := dataCallback("part_data", length-index, true); err != nil {
		return err
	}
	m.parserState, m.index, m.flags = state, index, flags
	return nil
}
