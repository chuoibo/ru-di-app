package pil

// Opened is what a plugin's factory returned inside Image.open: the size is
// known, the pixels are not decoded yet.
type Opened interface {
	// Size is im.size right after Image.open.
	Size() (width, height int)
	// Load is im.load() followed by whatever the sanitizer reads before
	// exif_transpose: the decoded image with its info keys, or the
	// exception load raised (any error ends in not_an_image, except a
	// *BombError).
	Load() (*Image, error)
}

// Plugin is one entry of Image.OPEN, in registration order.
//
// Open returns a *NextError when Image.open's _open_core would catch the
// exception and try the next plugin, a *BombError for a decompression bomb
// check inside the plugin, and any other error when the exception would
// escape Image.open.
type Plugin struct {
	Name string
	// Accept is the plugin's _accept on the first 16 bytes (fewer when the
	// input is shorter); nil when the plugin registered without one. A
	// string result (a warning) counts as false.
	Accept func(prefix []byte) bool
	Open   func(data []byte) (Opened, error)
}
