package googleid

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

func fetchGoogleKeys() (map[string]*rsa.PublicKey, time.Duration, error) {
	resp, err := http.Get(googleCertsURL)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("google certs answered %d", resp.StatusCode)
	}
	var doc jwks
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, 0, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		n, err := jwkInt(k.N)
		if err != nil {
			return nil, 0, err
		}
		e, err := jwkInt(k.E)
		if err != nil {
			return nil, 0, err
		}
		if !e.IsInt64() {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: n, E: int(e.Int64())}
	}
	if len(keys) == 0 {
		return nil, 0, fmt.Errorf("google certs carried no RSA keys")
	}
	maxAge := time.Hour
	if cc := resp.Header.Get("Cache-Control"); cc != "" {
		for _, part := range strings.Split(cc, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "max-age=") {
				if n, err := strconv.Atoi(strings.TrimPrefix(part, "max-age=")); err == nil && n > 0 {
					maxAge = time.Duration(n) * time.Second
				}
			}
		}
	}
	return keys, maxAge, nil
}
