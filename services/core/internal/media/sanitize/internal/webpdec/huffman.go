package webpdec

// Port of utils/huffman_utils.c.

type huffmanCode struct {
	bits  uint8
	value uint16
}

const (
	huffmanTableBits     = 8
	huffmanTableMask     = (1 << huffmanTableBits) - 1
	lengthsTableBits     = 7
	lengthsTableMask     = (1 << lengthsTableBits) - 1
	maxAllowedCodeLength = 15
)

func getNextKey(key uint32, length int) uint32 {
	step := uint32(1) << uint(length-1)
	for key&step != 0 {
		step >>= 1
	}
	if step != 0 {
		return (key & (step - 1)) + step
	}
	return key
}

func replicateValue(table []huffmanCode, step, end int, code huffmanCode) {
	for {
		end -= step
		table[end] = code
		if end <= 0 {
			break
		}
	}
}

func nextTableBitSize(count []int, length, rootBits int) int {
	left := 1 << uint(length-rootBits)
	for length < maxAllowedCodeLength {
		left -= count[length]
		if left <= 0 {
			break
		}
		length++
		left <<= 1
	}
	return length - rootBits
}

// buildTable is BuildHuffmanTable; root nil is the sizing pass.
func buildTable(root []huffmanCode, rootBits int, codeLengths []int) int {
	totalSize := 1 << uint(rootBits)
	var count [maxAllowedCodeLength + 1]int
	var offset [maxAllowedCodeLength + 1]int
	size := len(codeLengths)
	for _, l := range codeLengths {
		if l > maxAllowedCodeLength {
			return 0
		}
		count[l]++
	}
	if count[0] == size {
		return 0
	}
	offset[1] = 0
	for l := 1; l < maxAllowedCodeLength; l++ {
		if count[l] > (1 << uint(l)) {
			return 0
		}
		offset[l+1] = offset[l] + count[l]
	}
	var sorted []uint16
	if root != nil {
		sorted = make([]uint16, size)
	}
	for symbol, l := range codeLengths {
		if l > 0 {
			if root != nil {
				if offset[l] >= size {
					return 0
				}
				sorted[offset[l]] = uint16(symbol)
			}
			offset[l]++
		}
	}
	if offset[maxAllowedCodeLength] == 1 {
		if root != nil {
			replicateValue(root, 1, totalSize, huffmanCode{bits: 0, value: sorted[0]})
		}
		return totalSize
	}
	low := uint32(0xffffffff)
	mask := uint32(totalSize - 1)
	key := uint32(0)
	numNodes, numOpen := 1, 1
	tableSize := 1 << uint(rootBits)
	tableOff := 0
	symbol := 0
	step := 2
	for l := 1; l <= rootBits; l, step = l+1, step<<1 {
		numOpen <<= 1
		numNodes += numOpen
		numOpen -= count[l]
		if numOpen < 0 {
			return 0
		}
		if root == nil {
			continue
		}
		for ; count[l] > 0; count[l]-- {
			replicateValue(root[key:], step, tableSize, huffmanCode{bits: uint8(l), value: sorted[symbol]})
			symbol++
			key = getNextKey(key, l)
		}
	}
	step = 2
	for l := rootBits + 1; l <= maxAllowedCodeLength; l, step = l+1, step<<1 {
		numOpen <<= 1
		numNodes += numOpen
		numOpen -= count[l]
		if numOpen < 0 {
			return 0
		}
		for ; count[l] > 0; count[l]-- {
			if key&mask != low {
				if root != nil {
					tableOff += tableSize
				}
				tableBits := nextTableBitSize(count[:], l, rootBits)
				tableSize = 1 << uint(tableBits)
				totalSize += tableSize
				low = key & mask
				if root != nil {
					root[low].bits = uint8(tableBits + rootBits)
					root[low].value = uint16(tableOff - int(low))
				}
			}
			if root != nil {
				replicateValue(root[tableOff+int(key>>uint(rootBits)):], step, tableSize,
					huffmanCode{bits: uint8(l - rootBits), value: sorted[symbol]})
				symbol++
			}
			key = getNextKey(key, l)
		}
	}
	if numNodes != 2*offset[maxAllowedCodeLength]-1 {
		return 0
	}
	return totalSize
}

// buildHuffmanTable is VP8LBuildHuffmanTable; without build it only sizes.
func buildHuffmanTable(rootBits int, codeLengths []int, build bool) ([]huffmanCode, int) {
	total := buildTable(nil, rootBits, codeLengths)
	if total == 0 || !build {
		return nil, total
	}
	table := make([]huffmanCode, total)
	buildTable(table, rootBits, codeLengths)
	return table, total
}
