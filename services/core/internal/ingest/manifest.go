package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ManifestFile is one delivered file and what it claims about itself.
type ManifestFile struct {
	Name   string `json:"name"`
	Rows   int    `json:"rows"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// Manifest travels with a delivery and is the only thing that says what a
// complete delivery looks like.
//
// `DotSeq` increments per delivery so a gap is detectable, and `DotTruoc`
// chains them. Neither is enforced as a foreign key: a broken chain has to be
// reportable, and refusing it would block the recovery load that repairs it.
type Manifest struct {
	SchemaVersion string         `json:"schema_version"`
	Dot           string         `json:"dot"`
	DotSeq        int            `json:"dot_seq"`
	DotTruoc      *string        `json:"dot_truoc"`
	KieuDot       string         `json:"kieu_dot"`
	TuMoc         *string        `json:"tu_moc"`
	ExportedAt    string         `json:"exported_at"`
	Files         []ManifestFile `json:"files"`
	Rows          int            `json:"rows"`
	UpdatedAtMin  *time.Time     `json:"updated_at_min"`
	UpdatedAtMax  *time.Time     `json:"updated_at_max"`
	FramesRoot    string         `json:"frames_root"`

	// Dir is where the manifest was read from; file names resolve against it.
	Dir string `json:"-"`
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ReadManifest loads and checks a manifest, without opening the data files.
func ReadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("manifest is not JSON: %w", err)
	}
	manifest.Dir = filepath.Dir(path)
	if manifest.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("manifest declares %q, this reader is %s",
			manifest.SchemaVersion, SchemaVersion)
	}
	if strings.TrimSpace(manifest.Dot) == "" {
		return nil, fmt.Errorf("manifest has no delivery name")
	}
	if manifest.DotSeq <= 0 {
		return nil, fmt.Errorf("delivery %s has sequence %d; a delivery that "+
			"cannot be ordered cannot be found missing", manifest.Dot, manifest.DotSeq)
	}
	switch manifest.KieuDot {
	case "toan_bo", "tang_dan":
	default:
		return nil, fmt.Errorf("delivery %s is of unknown kind %q",
			manifest.Dot, manifest.KieuDot)
	}
	if len(manifest.Files) == 0 {
		return nil, fmt.Errorf("delivery %s lists no files", manifest.Dot)
	}
	for _, file := range manifest.Files {
		if !sha256Hex.MatchString(file.SHA256) {
			return nil, fmt.Errorf("%s: digest %q is not a sha256",
				file.Name, file.SHA256)
		}
		if strings.ContainsAny(file.Name, `/\`) {
			// The manifest names a file beside itself, never a path. Accepting
			// one would let a delivery reach anywhere the process can read.
			return nil, fmt.Errorf("%s: a file name, not a path", file.Name)
		}
	}
	return &manifest, nil
}

// VerifyFile reads one delivered file through and checks it against what the
// manifest claimed.
//
// Done before anything is written, and on the bytes rather than on the name:
// a delivery whose digest does not match is not a delivery that is slightly
// wrong, it is a different file than the one that was described.
func (m *Manifest) VerifyFile(file ManifestFile) error {
	path := filepath.Join(m.Dir, file.Name)
	handle, err := os.Open(path)
	if err != nil {
		return err
	}
	defer handle.Close()

	digest := sha256.New()
	written, err := io.Copy(digest, handle)
	if err != nil {
		return err
	}
	if written != file.Bytes {
		return fmt.Errorf("%s: %d bytes on disk, manifest says %d",
			file.Name, written, file.Bytes)
	}
	got := hex.EncodeToString(digest.Sum(nil))
	if got != file.SHA256 {
		return fmt.Errorf("%s: sha256 %s, manifest says %s",
			file.Name, got, file.SHA256)
	}
	return nil
}

// Path is where one delivered file sits.
func (m *Manifest) Path(file ManifestFile) string {
	return filepath.Join(m.Dir, file.Name)
}

// LineDigest is the content hash of one delivered line. Storing it lets a
// re-delivery of an identical line be recognised as the same fact rather than
// written again.
func LineDigest(line []byte) string {
	sum := sha256.Sum256(line)
	return hex.EncodeToString(sum[:])
}
