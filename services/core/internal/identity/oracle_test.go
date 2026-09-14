package identity

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// testdata/python_*.json is rendered by scripts/render_domain_w2_goldens.py
// from the real app.api.person_identity in the parity API image. Every case is
// replayed here; an error must match Python's exception type and message.

type goldenFile struct {
	Mode      string          `json:"mode"`
	Constants json.RawMessage `json:"constants"`
	Fuzz      *struct {
		Shard  int `json:"shard"`
		Shards int `json:"shards"`
		Total  int `json:"total"`
	} `json:"fuzz"`
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Fn     string         `json:"fn"`
	Name   string         `json:"name"`
	Args   map[string]any `json:"args"`
	Result map[string]any `json:"result"`
}

func loadGoldens(t *testing.T) []goldenFile {
	t.Helper()
	paths, err := filepath.Glob("testdata/python_*.json")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	sort.Strings(paths)
	files := make([]goldenFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file goldenFile
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		files = append(files, file)
	}
	return files
}

// text reads a str of the oracle encoding: plain, or "$sp:" groups of six
// code points joined by "|".
func text(raw any) (string, error) {
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("want a str, got %T", raw)
	}
	if !strings.HasPrefix(s, "$") {
		return s, nil
	}
	body, found := strings.CutPrefix(s, "$sp:")
	if !found {
		return "", fmt.Errorf("not a str: %q", s)
	}
	var out strings.Builder
	count := 0
	for i := 0; i < len(body); {
		r, size := utf8.DecodeRuneInString(body[i:])
		i += size
		if count == 6 {
			if r != '|' {
				return "", fmt.Errorf("bad group separator in %q", s)
			}
			count = 0
			continue
		}
		out.WriteRune(r)
		count++
	}
	return out.String(), nil
}

// octets reads "$hex:" bytes, the digits grouped by ":".
func octets(raw any) ([]byte, error) {
	s, _ := raw.(string)
	body, found := strings.CutPrefix(s, "$hex:")
	if !found {
		return nil, fmt.Errorf("not bytes: %#v", raw)
	}
	return hex.DecodeString(strings.ReplaceAll(body, ":", ""))
}

type outcome struct {
	value   any
	errType string
	message string
}

func (o outcome) String() string {
	if o.errType != "" {
		return fmt.Sprintf("raise %s(%q)", o.errType, o.message)
	}
	return fmt.Sprintf("return %#v", o.value)
}

func goOutcome(value any, err error) outcome {
	if err == nil {
		return outcome{value: value}
	}
	var missing *PersonIDKeyMissing
	if errors.As(err, &missing) {
		return outcome{errType: "PersonIdKeyMissing", message: missing.Error()}
	}
	var encode *UnicodeEncodeError
	if errors.As(err, &encode) {
		return outcome{errType: "UnicodeEncodeError", message: encode.Error()}
	}
	return outcome{errType: fmt.Sprintf("go:%T", err), message: err.Error()}
}

// pythonOutcome decodes the result the way fn's Go counterpart renders it:
// str for ids and canonical numbers, a hex string for bytes.
func pythonOutcome(c goldenCase) (outcome, error) {
	if raised, ok := c.Result["raised"].(map[string]any); ok {
		kind, _ := raised["type"].(string)
		message, err := text(raised["message"])
		if err != nil || raised["code"] != nil {
			return outcome{}, fmt.Errorf("raised %v: %v", raised, err)
		}
		return outcome{errType: kind, message: message}, nil
	}
	body, ok := c.Result["ok"]
	if !ok {
		return outcome{}, errors.New("result has neither ok nor raised")
	}
	switch c.Fn {
	case "canonical_mobile":
		if body == nil {
			return outcome{value: nil}, nil
		}
		s, err := text(body)
		return outcome{value: s}, err
	case "derive_person_id":
		s, err := text(body)
		return outcome{value: s}, err
	default:
		b, err := octets(body)
		return outcome{value: hex.EncodeToString(b)}, err
	}
}

func textArg(c goldenCase, key string) (string, error) {
	raw, ok := c.Args[key]
	if !ok {
		return "", fmt.Errorf("missing argument %q", key)
	}
	return text(raw)
}

func replay(c goldenCase) (outcome, error) {
	switch c.Fn {
	case "canonical_mobile":
		raw, err := textArg(c, "raw")
		if err != nil {
			return outcome{}, err
		}
		canonical, ok := CanonicalMobile(raw)
		if !ok {
			if canonical != "" {
				return outcome{}, errors.New("a refusal returned text")
			}
			return outcome{value: nil}, nil
		}
		return outcome{value: canonical}, nil
	case "derive_person_id", "derive_phone_digest":
		canonical, err := textArg(c, "canonical")
		if err != nil {
			return outcome{}, err
		}
		key, err := octets(c.Args["key"])
		if err != nil {
			return outcome{}, err
		}
		if c.Fn == "derive_person_id" {
			id, err := DerivePersonID(canonical, key)
			return goOutcome(id, err), nil
		}
		digest, err := DerivePhoneDigest(canonical, key)
		return goOutcome(hex.EncodeToString(digest), err), nil
	case "derive_code_digest":
		challenge, err := textArg(c, "challenge_id")
		if err != nil {
			return outcome{}, err
		}
		id, err := hex.DecodeString(strings.ReplaceAll(challenge, "-", ""))
		if err != nil || len(id) != 16 {
			return outcome{}, fmt.Errorf("challenge_id %q: %v", challenge, err)
		}
		code, err := textArg(c, "code")
		if err != nil {
			return outcome{}, err
		}
		key, err := octets(c.Args["key"])
		if err != nil {
			return outcome{}, err
		}
		digest, err := DeriveCodeDigest([16]byte(id), code, key)
		return goOutcome(hex.EncodeToString(digest), err), nil
	case "read_key":
		env, present := c.Args["env"]
		if !present {
			return outcome{}, errors.New("missing env")
		}
		raw := []byte{}
		if env != nil {
			decoded, err := octets(env)
			if err != nil {
				return outcome{}, err
			}
			raw = decoded
		}
		key, err := ReadKey(string(raw))
		return goOutcome(hex.EncodeToString(key), err), nil
	}
	return outcome{}, fmt.Errorf("unknown function %q", c.Fn)
}

func TestIdentityMatchesPython(t *testing.T) {
	perFn := map[string]int{}
	returned := map[string]int{}
	mismatches, total := 0, 0
	for _, file := range loadGoldens(t) {
		for _, c := range file.Cases {
			want, err := pythonOutcome(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			got, err := replay(c)
			if err != nil {
				t.Fatalf("%s %s %s: %v", file.Mode, c.Fn, c.Name, err)
			}
			total++
			perFn[c.Fn]++
			if want.errType == "" && want.value != nil {
				returned[c.Fn]++
			}
			if !reflect.DeepEqual(want, got) {
				mismatches++
				if mismatches <= 20 {
					t.Errorf("%s %s %s(%v):\n  Python %s\n  Go     %s", file.Mode, c.Name, c.Fn, c.Args, want, got)
				}
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("%d mismatches over %d Python cases", mismatches, total)
	}
	minimum := map[string]int{
		"canonical_mobile": 3000, "derive_person_id": 500, "derive_phone_digest": 500,
		"derive_code_digest": 500, "read_key": 800,
	}
	for fn, least := range minimum {
		if perFn[fn] < least || returned[fn] < least/10 || perFn[fn]-returned[fn] < least/10 {
			t.Errorf("%s: %d cases, %d returning a value: the corpus lost its spread", fn, perFn[fn], returned[fn])
		}
	}
	t.Logf("%d Python cases agree, 0 mismatches; per function %v, returning a value %v", total, perFn, returned)
}

func TestFuzzShardsAreComplete(t *testing.T) {
	seen := map[int]bool{}
	declared, shards, rendered := -1, -1, 0
	for _, file := range loadGoldens(t) {
		if file.Fuzz == nil {
			continue
		}
		if declared >= 0 && (declared != file.Fuzz.Total || shards != file.Fuzz.Shards) {
			t.Fatal("fuzz shards disagree on their totals")
		}
		declared, shards = file.Fuzz.Total, file.Fuzz.Shards
		if seen[file.Fuzz.Shard] {
			t.Fatalf("shard %d appears twice", file.Fuzz.Shard)
		}
		seen[file.Fuzz.Shard] = true
		rendered += len(file.Cases)
	}
	if shards < 1 || len(seen) != shards || rendered != declared {
		t.Fatalf("%d of %d shards holding %d of %d fuzz cases", len(seen), shards, rendered, declared)
	}
}

var goNames = map[string]string{
	"DOMAIN":              "Domain",
	"KEY_ENV_VAR":         "KeyEnvVar",
	"MIN_KEY_LENGTH":      "MinKeyLength",
	"OTP_CODE_DOMAIN":     "OTPCodeDomain",
	"OTP_PHONE_DOMAIN":    "OTPPhoneDomain",
	"PersonIdKeyMissing":  "PersonIDKeyMissing",
	"canonical_mobile":    "CanonicalMobile",
	"derive_code_digest":  "DeriveCodeDigest",
	"derive_person_id":    "DerivePersonID",
	"derive_phone_digest": "DerivePhoneDigest",
	"read_key":            "ReadKey",
}

func inRanges(ranges [][2]rune, cursor *int, r rune) bool {
	for *cursor < len(ranges) && ranges[*cursor][1] < r {
		*cursor++
	}
	return *cursor < len(ranges) && ranges[*cursor][0] <= r
}

func TestConstantsMatchPython(t *testing.T) {
	var constants struct {
		KeyEnvVar      string    `json:"key_env_var"`
		MinKeyLength   int       `json:"min_key_length"`
		Domain         string    `json:"domain"`
		OTPPhoneDomain string    `json:"otp_phone_domain"`
		OTPCodeDomain  string    `json:"otp_code_domain"`
		IsSpace        [][2]rune `json:"isspace"`
		Decimal        [][3]int  `json:"decimal"`
		ReDigit        [][2]rune `json:"re_digit"`
		ReSpace        [][2]rune `json:"re_space"`
		Names          []string  `json:"names"`
	}
	found := 0
	for _, file := range loadGoldens(t) {
		if file.Constants == nil {
			continue
		}
		found++
		if err := json.Unmarshal(file.Constants, &constants); err != nil {
			t.Fatal(err)
		}
	}
	if found != 1 {
		t.Fatalf("%d files carry constants, want 1", found)
	}
	if KeyEnvVar != constants.KeyEnvVar || MinKeyLength != constants.MinKeyLength {
		t.Errorf("KEY_ENV_VAR/MIN_KEY_LENGTH: Python %q %d", constants.KeyEnvVar, constants.MinKeyLength)
	}
	for name, pair := range map[string][2]any{
		"DOMAIN":           {constants.Domain, Domain()},
		"OTP_PHONE_DOMAIN": {constants.OTPPhoneDomain, OTPPhoneDomain()},
		"OTP_CODE_DOMAIN":  {constants.OTPCodeDomain, OTPCodeDomain()},
	} {
		want, err := octets(pair[0])
		if err != nil || !bytes.Equal(want, pair[1].([]byte)) {
			t.Errorf("%s: Python %q, Go %q (%v)", name, want, pair[1], err)
		}
	}
	if len(constants.Names) != len(goNames) {
		t.Errorf("Python has %d public names %v, goNames lists %d", len(constants.Names), constants.Names, len(goNames))
	}
	for _, name := range constants.Names {
		if goNames[name] == "" {
			t.Errorf("Python exports %s and the port lists no counterpart", name)
		}
	}
	if len(constants.IsSpace) == 0 || len(constants.ReDigit) == 0 || len(constants.ReSpace) == 0 || len(constants.Decimal) == 0 {
		t.Fatal("character tables missing")
	}
	space, reSpace, reDigit, decimal := 0, 0, 0, 0
	for r := rune(0); r <= utf8.MaxRune; r++ {
		pySpace := inRanges(constants.IsSpace, &space, r)
		if isPySpace(r) != pySpace || inRanges(constants.ReSpace, &reSpace, r) != pySpace {
			t.Errorf("U+%04X: str.isspace() %v, re \\s %v, Go %v", r, pySpace, !pySpace, isPySpace(r))
		}
		for decimal < len(constants.Decimal) && rune(constants.Decimal[decimal][1]) < r {
			decimal++
		}
		isDecimal := decimal < len(constants.Decimal) && rune(constants.Decimal[decimal][0]) <= r
		if unicode.Is(unicode.Nd, r) != isDecimal || inRanges(constants.ReDigit, &reDigit, r) != isDecimal {
			t.Errorf("U+%04X: unicodedata.decimal says %v, Go Nd %v", r, isDecimal, unicode.Is(unicode.Nd, r))
		}
	}
}
