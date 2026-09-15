package runner

import (
	"bytes"
	"fmt"

	"mobile/parity/internal/scenario"
)

// multipartBody assembles a step's body_parts in exactly the layout
// scenario.Part documents. Nothing is escaped or reordered: validation already
// keeps quotes and line breaks out of names, and a body that needs them is
// written in body_raw.
func multipartBody(boundary string, parts []scenario.Part, vars map[string]string) ([]byte, error) {
	var body bytes.Buffer
	render := func(text string) (string, error) { return scenario.Render(text, vars) }
	for i, part := range parts {
		name, err := render(part.Name)
		if err != nil {
			return nil, err
		}
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString(`Content-Disposition: form-data; name="` + name + `"`)
		if part.Filename != nil {
			filename, err := render(*part.Filename)
			if err != nil {
				return nil, err
			}
			body.WriteString(`; filename="` + filename + `"`)
		}
		body.WriteString("\r\n")
		if part.ContentType != nil {
			contentType, err := render(*part.ContentType)
			if err != nil {
				return nil, err
			}
			body.WriteString("Content-Type: " + contentType + "\r\n")
		}
		body.WriteString("\r\n")
		if part.Text != nil {
			text, err := render(*part.Text)
			if err != nil {
				return nil, err
			}
			body.WriteString(text)
		} else {
			data, err := GenerateImage(*part.Image)
			if err != nil {
				return nil, fmt.Errorf("body_parts[%d]: %w", i, err)
			}
			body.Write(data)
		}
		body.WriteString("\r\n")
	}
	body.WriteString("--" + boundary + "--\r\n")
	return body.Bytes(), nil
}
