package zlib131

// compressFunc names the strategy function a configuration_table row selects.
type compressFunc int

const (
	funcStored compressFunc = iota
	funcFast
	funcSlow
)

// config is one row of zlib 1.3.1's configuration_table (deflate.c,
// without FASTEST).
type config struct {
	goodLength uint16 // reduce lazy search above this match length
	maxLazy    uint16 // do not perform lazy search above this match length
	niceLength uint16 // quit search above this match length
	maxChain   uint16
	fn         compressFunc
}

var configurationTable = [10]config{
	{0, 0, 0, 0, funcStored},
	{4, 4, 8, 4, funcFast},
	{4, 5, 16, 8, funcFast},
	{4, 6, 32, 32, funcFast},
	{4, 4, 16, 16, funcSlow},
	{8, 16, 32, 32, funcSlow},
	{8, 16, 128, 128, funcSlow},
	{8, 32, 128, 256, funcSlow},
	{32, 128, 258, 1024, funcSlow},
	{32, 258, 258, 4096, funcSlow},
}
