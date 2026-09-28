package nhung

import "strconv"

func appendFloat(b []byte, x float32) []byte {
	return strconv.AppendFloat(b, float64(x), 'g', -1, 32)
}
