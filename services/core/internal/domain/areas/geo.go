package areas

import (
	"math"
	"slices"
)

// MaxAreaRadiusKm is MAX_AREA_RADIUS_KM: beyond it a coordinate belongs to
// no area.
const MaxAreaRadiusKm = 25.0

// earthRadiusKm is _EARTH_RADIUS_KM.
const earthRadiusKm = 6371.0088

// MathError is the exception CPython's math module or float arithmetic
// raises: Type is the exception class name and Message is str(exc).
type MathError struct {
	Type    string
	Message string
}

func (e *MathError) Error() string { return e.Type + ": " + e.Message }

var errMathDomain = &MathError{Type: "ValueError", Message: "math domain error"}

// piDouble and degToRad are computed in float64, as C folds
// `Py_MATH_PI / 180.0`. The exact Go constant math.Pi / 180 happens to round
// to the same bits; the float64 spelling is kept because it is the one that
// is right by construction rather than by coincidence.
var (
	piDouble = math.Pi
	degToRad = piDouble / 180.0
)

// radians is math.radians.
func radians(x float64) float64 { return float64(x * degToRad) }

// mathUnary is CPython's math_1 for a function that cannot overflow.
func mathUnary(x float64, f func(float64) float64) (float64, error) {
	r := f(x)
	if math.IsNaN(r) && !math.IsNaN(x) {
		return 0, errMathDomain
	}
	if math.IsInf(r, 0) && !math.IsInf(x, 0) && !math.IsNaN(x) {
		return 0, errMathDomain
	}
	return r, nil
}

// pyPow2 is CPython's float_pow(v, 2.0) over glibc pow.
func pyPow2(iv float64) (float64, error) {
	switch {
	case math.IsNaN(iv):
		return iv, nil
	case math.IsInf(iv, 0):
		return math.Abs(iv), nil
	case iv == 0:
		return 0, nil
	}
	iv = math.Abs(iv)
	if iv == 1.0 {
		return 1.0, nil
	}
	ix, errno := libmPow(iv, 2.0)
	if errno == errnoNone {
		if math.IsInf(ix, 0) {
			errno = errnoERANGE
		}
	} else if errno == errnoERANGE && ix == 0 {
		errno = errnoNone
	}
	switch errno {
	case errnoNone:
		return ix, nil
	case errnoERANGE:
		return 0, &MathError{Type: "OverflowError", Message: "(34, 'Numerical result out of range')"}
	default:
		return 0, &MathError{Type: "ValueError", Message: "(33, 'Numerical argument out of domain')"}
	}
}

// HaversineKm is haversine_km: the great-circle distance in kilometres, with
// the exact bits CPython computes and the ValueError it raises when a
// rounded intermediate leaves the domain of sqrt or asin.
func HaversineKm(lat1, lng1, lat2, lng2 float64) (float64, error) {
	phi1, phi2 := radians(lat1), radians(lat2)
	dPhi := radians(lat2 - lat1)
	dLambda := radians(lng2 - lng1)
	s1, err := mathUnary(dPhi/2, libmSin)
	if err != nil {
		return 0, err
	}
	p1, err := pyPow2(s1)
	if err != nil {
		return 0, err
	}
	c1, err := mathUnary(phi1, libmCos)
	if err != nil {
		return 0, err
	}
	c2, err := mathUnary(phi2, libmCos)
	if err != nil {
		return 0, err
	}
	s2, err := mathUnary(dLambda/2, libmSin)
	if err != nil {
		return 0, err
	}
	p2, err := pyPow2(s2)
	if err != nil {
		return 0, err
	}
	a := p1 + float64(float64(c1*c2)*p2)
	root, err := mathUnary(a, math.Sqrt)
	if err != nil {
		return 0, err
	}
	angle, err := mathUnary(root, libmAsin)
	if err != nil {
		return 0, err
	}
	return float64(float64(2*earthRadiusKm) * angle), nil
}

// byID is AREAS sorted by id, the order nearest_area walks.
var byID = func() []Area {
	sorted := All()
	slices.SortStableFunc(sorted, func(a, b Area) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return sorted
}()

// NearestArea is nearest_area: the closest area strictly inside the radius,
// ties going to the smaller id, or false when none is. The error is the
// ValueError haversine_km raised, which Python lets escape.
func NearestArea(lat, lng float64) (Area, bool, error) {
	best := -1
	bestKm := MaxAreaRadiusKm
	for i, area := range byID {
		km, err := HaversineKm(lat, lng, area.Lat, area.Lng)
		if err != nil {
			return Area{}, false, err
		}
		if km < bestKm {
			best, bestKm = i, km
		}
	}
	if best < 0 {
		return Area{}, false, nil
	}
	return byID[best], true, nil
}
