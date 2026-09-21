package jpegdec

// This file holds the decompressor state: a port of the libjpeg-turbo
// 3.1.4.1 j_decompress_ptr fields that Pillow's decoder reaches with its
// settings (8-bit samples, islow IDCT, fancy upsampling, no scaling, no
// colour quantization).

// J_COLOR_SPACE values used here.
const (
	csUnknown = iota
	csGrayscale
	csRGB
	csYCbCr
	csCMYK
	csYCCK
	csExtRGBX
)

const (
	reachedSOS = 1
	reachedEOI = 2

	maxCompsInScan  = 4
	maxComponents   = 10
	maxSampFactor   = 4
	dMaxBlocksInMCU = 10
	maxDimension    = 65500
	numQuantTbls    = 4
	numHuffTbls     = 4
	numArithTbls    = 16
)

// naturalOrder is jpeg_natural_order with its 16 guard entries, which
// corrupt run lengths index.
var naturalOrder = [80]int{
	0, 1, 8, 16, 9, 2, 3, 10,
	17, 24, 32, 25, 18, 11, 4, 5,
	12, 19, 26, 33, 40, 48, 41, 34,
	27, 20, 13, 6, 7, 14, 21, 28,
	35, 42, 49, 56, 57, 50, 43, 36,
	29, 22, 15, 23, 30, 37, 44, 51,
	58, 59, 52, 45, 38, 31, 39, 46,
	53, 60, 61, 54, 47, 55, 62, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
	63, 63, 63, 63, 63, 63, 63, 63,
}

// huffTable is JHUFF_TBL.
type huffTable struct {
	bits    [17]uint8
	huffval [256]uint8
}

// component is jpeg_component_info plus the sample and coefficient storage
// the controllers keep for it.
type component struct {
	id         int
	index      int
	h, v       int
	quantTblNo int
	dcTblNo    int
	acTblNo    int

	widthInBlocks     int
	heightInBlocks    int
	downsampledWidth  int
	downsampledHeight int

	// quant is compptr->quant_table, latched when a scan first includes
	// the component; nil until then.
	quant *[64]uint16
	// dctTable is the islow multiplier table (ISLOW_MULT_TYPE is short in
	// a SIMD build).
	dctTable [64]int16

	mcuWidth      int
	mcuHeight     int
	mcuBlocks     int
	lastColWidth  int
	lastRowHeight int

	// coef is the whole-image coefficient buffer of a multi-scan file:
	// coefCols x coefRows blocks, zero-filled like request_virt_barray.
	coef     []int16
	coefCols int
	coefRows int

	// plane holds the IDCT output, widthInBlocks*8 x heightInBlocks*8.
	plane       []byte
	planeStride int
}

// decoder is the decompression state.
type decoder struct {
	src *source

	// Marker reader.
	unreadMarker   int
	sawSOI         bool
	sawSOF         bool
	nextRestartNum int

	// Input controller.
	eoiReached       bool
	inHeaders        bool
	hasMultipleScans bool
	// scanActive is true while consume_input points at the coefficient
	// controller rather than consume_markers.
	scanActive       bool
	inputScanNumber  int
	outputScanNumber int

	// Datastream parameters.
	restartInterval int
	jpegColorSpace  int
	outColorSpace   int
	sawJFIF         bool
	sawAdobe        bool
	adobeTransform  int
	progressive     bool
	lossless        bool
	arith           bool
	dataPrecision   int
	imageWidth      int
	imageHeight     int
	numComponents   int
	comps           []component
	quantTbls       [numQuantTbls]*[64]uint16
	dcHuffTbls      [numHuffTbls]*huffTable
	acHuffTbls      [numHuffTbls]*huffTable
	arithDcL        [numArithTbls]uint8
	arithDcU        [numArithTbls]uint8
	arithAcK        [numArithTbls]uint8

	// Frame geometry.
	maxH          int
	maxV          int
	totalIMCURows int

	// Current scan.
	compsInScan       int
	curComps          [maxCompsInScan]*component
	ss, se, ah, al    int
	mcusPerRow        int
	mcuRowsInScan     int
	blocksInMCU       int
	mcuMembership     [dMaxBlocksInMCU]int
	inputIMCURow      int
	mcuRowsPerIMCURow int
	lastGoodIMCURow   int

	// coefBits is cinfo->coef_bits of a progressive file: numComponents
	// rows of current bit positions, then numComponents rows of the
	// previous ones.
	coefBits [][64]int

	ent entropy
	// arithS is the arithmetic decoder, allocated at its first scan.
	arithS *arithState
}

// unsupportedSignal unwinds a decode that reached a libjpeg path this port
// does not reproduce.
type unsupportedSignal struct{ reason string }
