package guest

import (
	"embed"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The guest templates are rendered by a purpose-built interpreter for the one
// Jinja subset they use, configured as Starlette's Jinja2Templates configures
// jinja2.Environment for app/api/routes/guests.py: autoescape on, the default
// Undefined, trim_blocks and lstrip_blocks off, keep_trailing_newline off,
// "\n" as the newline sequence, no finalize.
//
// Supported, and nothing else (anything else fails Parse):
//
//	{{ expr }}  {% if expr %} {% elif expr %} {% else %} {% endif %}
//	{% for name in expr %} / {% for a, b in expr %} ... {% endfor %}
//	{# comment #}   "-" whitespace control on all three delimiters
//	expr: name, 'str' or "str" (no backslash), a dict literal of str to str
//	      subscripted once, .attribute, loop.first, x != 'str', a or b
//
// html/template is not used: it escapes by context, and Jinja does not.

//go:embed templates/*.html
var templateFiles embed.FS

// TemplateNames are the embedded templates, copied from
// services/api/app/web/templates.
var TemplateNames = []string{"guest.html", "guest_link_broken.html", "guest_not_me.html", "guest_wrong_amount.html"}

var loadedTemplates = mustLoadTemplates()

func mustLoadTemplates() map[string]*Template {
	out := map[string]*Template{}
	for _, name := range TemplateNames {
		raw, err := templateFiles.ReadFile("templates/" + name)
		if err != nil {
			panic(err)
		}
		t, err := Parse(name, string(raw))
		if err != nil {
			panic(err)
		}
		out[name] = t
	}
	return out
}

// Lookup returns an embedded template, parsed when the package loaded.
func Lookup(name string) (*Template, error) {
	t, ok := loadedTemplates[name]
	if !ok {
		return nil, &PyError{Class: "TemplateNotFound", Message: name}
	}
	return t, nil
}

// Template is one parsed template.
type Template struct {
	name  string
	nodes []node
}

// ParseError is a template construct outside the supported subset.
type ParseError struct {
	Template string
	Reason   string
}

func (e *ParseError) Error() string { return "guest template " + e.Template + ": " + e.Reason }

type node interface{}

type textNode struct{ text string }

type outputNode struct{ value expr }

type ifNode struct {
	conds   []expr
	bodies  [][]node // one per cond, plus the else body when hasElse
	hasElse bool
}

type forNode struct {
	targets []string
	iter    expr
	body    []node
}

type expr interface{}

type nameExpr struct{ name string }

type strExpr struct{ value string }

type dictExpr struct {
	keys   []string
	values map[string]string
}

type attrExpr struct {
	base expr
	name string
}

type itemExpr struct {
	base *dictExpr
	key  expr
}

type neExpr struct {
	left  expr
	right string
}

type orExpr struct{ left, right expr }

// dictAttributes are the attributes of dict that Jinja's getattr would find
// before the key; none may be read as a key here.
var dictAttributes = map[string]bool{
	"clear": true, "copy": true, "fromkeys": true, "get": true, "items": true, "keys": true,
	"pop": true, "popitem": true, "setdefault": true, "update": true, "values": true,
}

// isPySpace is str.isspace, which both str.rstrip() and the lexer's \s use.
func isPySpace(r rune) bool {
	switch {
	case r >= 0x09 && r <= 0x0d, r >= 0x1c && r <= 0x20, r == 0x85, r == 0xa0, r == 0x1680,
		r >= 0x2000 && r <= 0x200a, r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000:
		return true
	}
	return false
}

// normalizeSource is the start of Lexer.tokeniter: every "\r\n", "\r" and
// "\n" becomes "\n", and one trailing line break is dropped.
func normalizeSource(source string) string {
	var lines []string
	start := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\r':
			lines = append(lines, source[start:i])
			if i+1 < len(source) && source[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n':
			lines = append(lines, source[start:i])
			start = i + 1
		}
	}
	lines = append(lines, source[start:])
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

type rawTag struct {
	kind    byte // '{' variable, '%' block
	content string
}

type piece struct {
	text string
	tag  *rawTag
}

// Parse reads a template of the supported subset.
func Parse(name, source string) (*Template, error) {
	pieces, err := lex(name, normalizeSource(source))
	if err != nil {
		return nil, err
	}
	p := &parser{name: name, pieces: pieces}
	nodes, end, err := p.body(0)
	if err != nil {
		return nil, err
	}
	if end != "" {
		return nil, p.fail("unexpected {%% %s %%}", end)
	}
	return &Template{name: name, nodes: nodes}, nil
}

func lex(name, src string) ([]piece, error) {
	fail := func(format string, args ...any) error {
		return &ParseError{Template: name, Reason: fmt.Sprintf(format, args...)}
	}
	var out []piece
	pos := 0
	for pos < len(src) {
		start := -1
		for i := pos; i+1 < len(src); i++ {
			if src[i] == '{' && (src[i+1] == '{' || src[i+1] == '%' || src[i+1] == '#') {
				start = i
				break
			}
		}
		if start < 0 {
			out = append(out, piece{text: src[pos:]})
			break
		}
		kind := src[start+1]
		text := src[pos:start]
		cursor := start + 2
		if cursor < len(src) {
			switch src[cursor] {
			case '-':
				text = strings.TrimRightFunc(text, isPySpace)
				cursor++
			case '+':
				return nil, fail("the + whitespace modifier")
			}
		}
		if text != "" {
			out = append(out, piece{text: text})
		}
		var closer string
		switch kind {
		case '#':
			closer = "#}"
		case '%':
			closer = "%}"
		default:
			closer = "}}"
		}
		endAt, stripAfter, next, err := findEnd(src, cursor, kind, closer)
		if err != nil {
			return nil, fail("%v", err)
		}
		if kind != '#' {
			out = append(out, piece{tag: &rawTag{kind: kind, content: src[cursor:endAt]}})
		}
		if stripAfter {
			rest := strings.TrimLeftFunc(src[next:], isPySpace)
			next = len(src) - len(rest)
		}
		pos = next
	}
	return out, nil
}

// findEnd finds the closing delimiter of a tag whose content starts at from.
// It returns where the content ends, whether the closer carried "-", and
// where the text after the closer starts.
func findEnd(src string, from int, kind byte, closer string) (int, bool, int, error) {
	if kind == '#' {
		idx := strings.Index(src[from:], closer)
		if idx < 0 {
			return 0, false, 0, fmt.Errorf("missing end of comment tag")
		}
		at := from + idx
		if at > from && src[at-1] == '-' {
			return at - 1, true, at + 2, nil
		}
		if at > from && src[at-1] == '+' {
			return 0, false, 0, fmt.Errorf("the + whitespace modifier")
		}
		return at, false, at + 2, nil
	}
	depth := 0
	for i := from; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\'' || c == '"':
			j := strings.IndexByte(src[i+1:], c)
			if j < 0 {
				return 0, false, 0, fmt.Errorf("unterminated string")
			}
			i += j + 1
		case depth == 0 && strings.HasPrefix(src[i:], "-"+closer):
			return i, true, i + 1 + len(closer), nil
		case depth == 0 && strings.HasPrefix(src[i:], "+"+closer):
			return 0, false, 0, fmt.Errorf("the + whitespace modifier")
		case depth == 0 && strings.HasPrefix(src[i:], closer):
			return i, false, i + len(closer), nil
		case c == '{' || c == '[' || c == '(':
			depth++
		case c == '}' || c == ']' || c == ')':
			if depth == 0 {
				return 0, false, 0, fmt.Errorf("unbalanced %q", c)
			}
			depth--
		}
	}
	return 0, false, 0, fmt.Errorf("missing end of tag")
}

type token struct {
	kind  string // "name", "str", "op"
	value string
}

func tokenize(content string) ([]token, error) {
	var out []token
	for i := 0; i < len(content); {
		c := content[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v':
			i++
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(content) && (content[j] == '_' || content[j] >= 'a' && content[j] <= 'z' ||
				content[j] >= 'A' && content[j] <= 'Z' || content[j] >= '0' && content[j] <= '9') {
				j++
			}
			out = append(out, token{"name", content[i:j]})
			i = j
		case c == '\'' || c == '"':
			j := strings.IndexByte(content[i+1:], c)
			if j < 0 {
				return nil, fmt.Errorf("unterminated string")
			}
			value := content[i+1 : i+1+j]
			if strings.ContainsAny(value, "\\\r") {
				return nil, fmt.Errorf("a string literal with a backslash or carriage return")
			}
			out = append(out, token{"str", value})
			i += j + 2
		case strings.HasPrefix(content[i:], "!="):
			out = append(out, token{"op", "!="})
			i += 2
		case strings.IndexByte(".[]{}:,", c) >= 0:
			out = append(out, token{"op", string(c)})
			i++
		default:
			r, _ := utf8.DecodeRuneInString(content[i:])
			return nil, fmt.Errorf("unsupported character %q in a tag", r)
		}
	}
	return out, nil
}

type parser struct {
	name   string
	pieces []piece
	pos    int
	loops  int
}

func (p *parser) fail(format string, args ...any) error {
	return &ParseError{Template: p.name, Reason: fmt.Sprintf(format, args...)}
}

// body parses nodes until a block tag that closes the caller's construct and
// returns that tag's keyword ("" at the end of the template).
func (p *parser) body(depth int) ([]node, string, error) {
	var nodes []node
	for p.pos < len(p.pieces) {
		pc := p.pieces[p.pos]
		if pc.tag == nil {
			nodes = append(nodes, textNode{text: pc.text})
			p.pos++
			continue
		}
		tokens, err := tokenize(pc.tag.content)
		if err != nil {
			return nil, "", p.fail("%v", err)
		}
		if pc.tag.kind == '{' {
			p.pos++
			value, err := p.expression(tokens)
			if err != nil {
				return nil, "", err
			}
			nodes = append(nodes, outputNode{value: value})
			continue
		}
		if len(tokens) == 0 || tokens[0].kind != "name" {
			return nil, "", p.fail("an empty or unnamed block tag")
		}
		switch keyword := tokens[0].value; keyword {
		case "if":
			p.pos++
			n, err := p.parseIf(tokens[1:], depth)
			if err != nil {
				return nil, "", err
			}
			nodes = append(nodes, n)
		case "for":
			p.pos++
			n, err := p.parseFor(tokens[1:], depth)
			if err != nil {
				return nil, "", err
			}
			nodes = append(nodes, n)
		case "elif", "else", "endif", "endfor":
			if depth == 0 {
				return nil, "", p.fail("{%% %s %%} outside a block", keyword)
			}
			return nodes, keyword, nil
		default:
			return nil, "", p.fail("the %q tag", keyword)
		}
	}
	return nodes, "", nil
}

func (p *parser) parseIf(cond []token, depth int) (node, error) {
	n := &ifNode{}
	for {
		value, err := p.expression(cond)
		if err != nil {
			return nil, err
		}
		body, end, err := p.body(depth + 1)
		if err != nil {
			return nil, err
		}
		n.conds = append(n.conds, value)
		n.bodies = append(n.bodies, body)
		if end == "" {
			return nil, p.fail("missing {%% endif %%}")
		}
		tokens, _ := tokenize(p.pieces[p.pos].tag.content)
		p.pos++
		switch end {
		case "elif":
			cond = tokens[1:]
			continue
		case "else":
			if len(tokens) != 1 {
				return nil, p.fail("{%% else %%} with arguments")
			}
			elseBody, end, err := p.body(depth + 1)
			if err != nil {
				return nil, err
			}
			if end != "endif" {
				return nil, p.fail("{%% else %%} closed by %q", end)
			}
			closing, _ := tokenize(p.pieces[p.pos].tag.content)
			p.pos++
			if len(closing) != 1 {
				return nil, p.fail("{%% endif %%} with arguments")
			}
			n.bodies = append(n.bodies, elseBody)
			n.hasElse = true
			return n, nil
		case "endif":
			if len(tokens) != 1 {
				return nil, p.fail("{%% endif %%} with arguments")
			}
			return n, nil
		default:
			return nil, p.fail("{%% if %%} closed by %q", end)
		}
	}
}

func (p *parser) parseFor(tokens []token, depth int) (node, error) {
	n := &forNode{}
	i := 0
	for {
		if i >= len(tokens) || tokens[i].kind != "name" || isKeyword(tokens[i].value) {
			return nil, p.fail("a for target that is not a plain name")
		}
		if tokens[i].value == "loop" {
			return nil, p.fail("assigning to the special loop variable")
		}
		n.targets = append(n.targets, tokens[i].value)
		i++
		if i < len(tokens) && tokens[i].kind == "op" && tokens[i].value == "," {
			i++
			continue
		}
		break
	}
	if i >= len(tokens) || tokens[i].kind != "name" || tokens[i].value != "in" {
		return nil, p.fail("a for loop without in")
	}
	for _, t := range tokens[i+1:] {
		if t.kind == "name" && (t.value == "if" || t.value == "recursive") {
			return nil, p.fail("a for loop with %s", t.value)
		}
	}
	iter, err := p.expression(tokens[i+1:])
	if err != nil {
		return nil, err
	}
	n.iter = iter
	p.loops++
	body, end, err := p.body(depth + 1)
	p.loops--
	if err != nil {
		return nil, err
	}
	if end != "endfor" {
		return nil, p.fail("{%% for %%} closed by %q", end)
	}
	n.body = body
	closing, _ := tokenize(p.pieces[p.pos].tag.content)
	p.pos++
	if len(closing) != 1 {
		return nil, p.fail("{%% endfor %%} with arguments")
	}
	return n, nil
}

func isKeyword(name string) bool {
	switch name {
	case "and", "or", "not", "in", "is", "if", "else", "true", "false", "none", "True", "False", "None":
		return true
	}
	return false
}

type exprParser struct {
	p      *parser
	tokens []token
	pos    int
}

func (p *parser) expression(tokens []token) (expr, error) {
	e := &exprParser{p: p, tokens: tokens}
	value, err := e.or()
	if err != nil {
		return nil, err
	}
	if e.pos != len(tokens) {
		return nil, p.fail("unsupported expression near %q", tokens[e.pos].value)
	}
	return value, nil
}

func (e *exprParser) peek(kind, value string) bool {
	return e.pos < len(e.tokens) && e.tokens[e.pos].kind == kind && e.tokens[e.pos].value == value
}

func (e *exprParser) or() (expr, error) {
	left, err := e.compare()
	if err != nil {
		return nil, err
	}
	for e.peek("name", "or") {
		e.pos++
		right, err := e.compare()
		if err != nil {
			return nil, err
		}
		left = orExpr{left: left, right: right}
	}
	return left, nil
}

func (e *exprParser) compare() (expr, error) {
	left, err := e.postfix()
	if err != nil {
		return nil, err
	}
	if !e.peek("op", "!=") {
		return left, nil
	}
	e.pos++
	if e.pos >= len(e.tokens) || e.tokens[e.pos].kind != "str" {
		return nil, e.p.fail("!= against something other than a str literal")
	}
	right := e.tokens[e.pos].value
	e.pos++
	if e.peek("op", "!=") {
		return nil, e.p.fail("a chained comparison")
	}
	return neExpr{left: left, right: right}, nil
}

func (e *exprParser) postfix() (expr, error) {
	if e.pos >= len(e.tokens) {
		return nil, e.p.fail("a missing expression")
	}
	t := e.tokens[e.pos]
	var base expr
	switch {
	case t.kind == "name" && !isKeyword(t.value):
		e.pos++
		base = nameExpr{name: t.value}
		if t.value == "loop" {
			if e.p.loops == 0 {
				return nil, e.p.fail("loop outside a for loop")
			}
			if !e.peek("op", ".") || e.pos+1 >= len(e.tokens) || e.tokens[e.pos+1].value != "first" {
				return nil, e.p.fail("a loop attribute other than first")
			}
		}
	case t.kind == "str":
		e.pos++
		return strExpr{value: t.value}, nil
	case t.kind == "op" && t.value == "{":
		d, err := e.dict()
		if err != nil {
			return nil, err
		}
		if !e.peek("op", "[") {
			return nil, e.p.fail("a dict literal that is not subscripted")
		}
		e.pos++
		key, err := e.or()
		if err != nil {
			return nil, err
		}
		if !e.peek("op", "]") {
			return nil, e.p.fail("an unclosed subscript")
		}
		e.pos++
		base = itemExpr{base: d, key: key}
		if e.peek("op", ".") || e.peek("op", "[") {
			return nil, e.p.fail("a chained lookup on a dict literal item")
		}
		return base, nil
	default:
		return nil, e.p.fail("unsupported expression %q", t.value)
	}
	for e.peek("op", ".") {
		e.pos++
		if e.pos >= len(e.tokens) || e.tokens[e.pos].kind != "name" {
			return nil, e.p.fail("an attribute that is not a name")
		}
		name := e.tokens[e.pos].value
		if dictAttributes[name] {
			return nil, e.p.fail("the attribute %q, which Jinja resolves on dict itself", name)
		}
		e.pos++
		base = attrExpr{base: base, name: name}
	}
	if e.peek("op", "[") {
		return nil, e.p.fail("a subscript on anything but a dict literal")
	}
	return base, nil
}

func (e *exprParser) dict() (*dictExpr, error) {
	e.pos++ // {
	d := &dictExpr{values: map[string]string{}}
	for !e.peek("op", "}") {
		if len(d.keys) > 0 {
			if !e.peek("op", ",") {
				return nil, e.p.fail("a malformed dict literal")
			}
			e.pos++
		}
		if e.pos+2 >= len(e.tokens) || e.tokens[e.pos].kind != "str" || !(e.tokens[e.pos+1].kind == "op" && e.tokens[e.pos+1].value == ":") || e.tokens[e.pos+2].kind != "str" {
			return nil, e.p.fail("a dict literal entry that is not 'str': 'str'")
		}
		key, value := e.tokens[e.pos].value, e.tokens[e.pos+2].value
		if _, dup := d.values[key]; dup {
			return nil, e.p.fail("a duplicate dict literal key")
		}
		d.keys = append(d.keys, key)
		d.values[key] = value
		e.pos += 3
	}
	e.pos++ // }
	return d, nil
}

// undefined is jinja2.Undefined.
type undefined struct{}

type loopContext struct{ index0 int }

type frame struct {
	vars map[string]any
	loop *loopContext
}

type renderer struct {
	context map[string]any
	frames  []frame
	out     strings.Builder
}

// Render renders the template over context, as Template.render(context).
// On error nothing is returned, as a raising render returns nothing.
func (t *Template) Render(context map[string]any) (string, error) {
	r := &renderer{context: context}
	if err := r.nodes(t.nodes); err != nil {
		return "", err
	}
	return r.out.String(), nil
}

func (r *renderer) nodes(nodes []node) error {
	for _, n := range nodes {
		switch n := n.(type) {
		case textNode:
			r.out.WriteString(n.text)
		case outputNode:
			value, err := r.eval(n.value)
			if err != nil {
				return err
			}
			text, err := display(value)
			if err != nil {
				return err
			}
			r.out.WriteString(Escape(text))
		case *ifNode:
			done := false
			for i, cond := range n.conds {
				value, err := r.eval(cond)
				if err != nil {
					return err
				}
				ok, err := renderTruthy(value)
				if err != nil {
					return err
				}
				if ok {
					if err := r.nodes(n.bodies[i]); err != nil {
						return err
					}
					done = true
					break
				}
			}
			if !done && n.hasElse {
				if err := r.nodes(n.bodies[len(n.conds)]); err != nil {
					return err
				}
			}
		case *forNode:
			if err := r.loop(n); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *renderer) loop(n *forNode) error {
	iterable, err := r.eval(n.iter)
	if err != nil {
		return err
	}
	var items []any
	switch v := iterable.(type) {
	case undefined:
	case []any:
		items = v
	case nil, bool, int, int64, *big.Int:
		return typeError("'%s' object is not iterable", typeName(v))
	default:
		return &UnsupportedError{Reason: fmt.Sprintf("iterating a %s", typeName(v))}
	}
	for i, it := range items {
		vars := map[string]any{}
		if len(n.targets) == 1 {
			vars[n.targets[0]] = it
		} else {
			switch v := it.(type) {
			case []any:
				if len(v) != len(n.targets) {
					return &PyError{Class: "ValueError", Message: "wrong number of values to unpack"}
				}
				for j, target := range n.targets {
					vars[target] = v[j]
				}
			case undefined:
				return &PyError{Class: "ValueError", Message: "not enough values to unpack"}
			case nil, bool, int, int64, *big.Int:
				return typeError("cannot unpack non-iterable %s object", typeName(v))
			default:
				return &UnsupportedError{Reason: fmt.Sprintf("unpacking a %s", typeName(v))}
			}
		}
		r.frames = append(r.frames, frame{vars: vars, loop: &loopContext{index0: i}})
		err := r.nodes(n.body)
		r.frames = r.frames[:len(r.frames)-1]
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) resolve(name string) any {
	for i := len(r.frames) - 1; i >= 0; i-- {
		if name == "loop" {
			return r.frames[i].loop
		}
		if v, ok := r.frames[i].vars[name]; ok {
			return v
		}
	}
	if v, ok := r.context[name]; ok {
		return v
	}
	return undefined{}
}

func (r *renderer) eval(e expr) (any, error) {
	switch e := e.(type) {
	case nameExpr:
		return r.resolve(e.name), nil
	case strExpr:
		return e.value, nil
	case attrExpr:
		base, err := r.eval(e.base)
		if err != nil {
			return nil, err
		}
		switch b := base.(type) {
		case undefined:
			return nil, &PyError{Class: "UndefinedError", Message: "attribute " + e.name + " of an undefined value"}
		case *loopContext:
			return b.index0 == 0, nil
		case map[string]any:
			if v, ok := b[e.name]; ok {
				return v, nil
			}
			return undefined{}, nil
		case nil:
			return undefined{}, nil
		}
		return nil, &UnsupportedError{Reason: fmt.Sprintf("an attribute of a %s", typeName(base))}
	case itemExpr:
		key, err := r.eval(e.key)
		if err != nil {
			return nil, err
		}
		switch k := key.(type) {
		case string:
			if v, ok := e.base.values[k]; ok {
				return v, nil
			}
			if dictAttributes[k] {
				return nil, &UnsupportedError{Reason: "a dict literal subscripted by the name of a dict method"}
			}
			return undefined{}, nil
		case *loopContext:
			return nil, &UnsupportedError{Reason: "subscripting by loop"}
		}
		return undefined{}, nil
	case neExpr:
		left, err := r.eval(e.left)
		if err != nil {
			return nil, err
		}
		s, ok := left.(string)
		return !ok || s != e.right, nil
	case orExpr:
		left, err := r.eval(e.left)
		if err != nil {
			return nil, err
		}
		ok, err := renderTruthy(left)
		if err != nil || ok {
			return left, err
		}
		return r.eval(e.right)
	}
	return nil, &UnsupportedError{Reason: fmt.Sprintf("expression %T", e)}
}

func renderTruthy(v any) (bool, error) {
	switch v.(type) {
	case undefined:
		return false, nil
	case *loopContext:
		return true, nil
	}
	return truthy(v)
}

// display is str(value) for the values a template prints.
func display(v any) (string, error) {
	switch x := v.(type) {
	case undefined:
		return "", nil
	case nil:
		return "None", nil
	case bool:
		if x {
			return "True", nil
		}
		return "False", nil
	case string:
		return x, nil
	case int:
		return strconv.Itoa(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case *big.Int:
		digits := x.String()
		if len(strings.TrimPrefix(digits, "-")) > maxStrDigits {
			return "", &PyError{Class: "ValueError", Message: "Exceeds the limit (4300 digits) for integer string conversion"}
		}
		return digits, nil
	}
	return "", &UnsupportedError{Reason: fmt.Sprintf("printing a %s", typeName(v))}
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", ">", "&gt;", "<", "&lt;", "'", "&#39;", `"`, "&#34;")

// Escape is markupsafe.escape on a str.
func Escape(s string) string { return htmlEscaper.Replace(s) }
