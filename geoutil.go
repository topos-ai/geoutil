package geoutil

import (
	"fmt"

	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
)

const (
	PrecisionMax = 0
	PrecisionE5  = iota
	PrecisionE6  = iota
	PrecisionE7  = iota
)

func precisionMax(a s1.Angle) float64 {
	return a.Degrees()
}

func precisionE5(a s1.Angle) float64 {
	return float64(a.E5()) / 1e5
}

func precisionE6(a s1.Angle) float64 {
	return float64(a.E6()) / 1e6
}

func precisionE7(a s1.Angle) float64 {
	return float64(a.E7()) / 1e7
}

func selectPrecisionFunc(precision int) (func(s1.Angle) float64, error) {
	switch precision {
	case PrecisionMax:
		return precisionMax, nil
	case PrecisionE5:
		return precisionE5, nil
	case PrecisionE6:
		return precisionE6, nil
	case PrecisionE7:
		return precisionE7, nil
	default:
		return nil, fmt.Errorf("geoutil: invalid precision level %d", precision)
	}
}

func pointCoordinates(point s2.Point, precisionFunc func(s1.Angle) float64) []float64 {
	latLng := s2.LatLngFromPoint(point)
	return []float64{
		precisionFunc(latLng.Lng),
		precisionFunc(latLng.Lat),
	}
}

func PointCoordinates(point s2.Point, precision int) ([]float64, error) {
	precisionFunc, err := selectPrecisionFunc(precision)
	if err != nil {
		return nil, err
	}

	return pointCoordinates(point, precisionFunc), nil
}

func loopCoordinates(loop *s2.Loop, precisionFunc func(s1.Angle) float64) [][]float64 {
	nv := loop.NumVertices()
	if nv == 0 {
		return [][]float64{}
	}

	lcs := make([][]float64, nv+1)
	for j := 0; j < nv; j++ {
		lcs[j] = pointCoordinates(loop.OrientedVertex(j), precisionFunc)
	}

	lcs[nv] = lcs[0]
	return lcs
}

func LoopCoordinates(loop *s2.Loop, precision int) ([][]float64, error) {
	precisionFunc, err := selectPrecisionFunc(precision)
	if err != nil {
		return nil, err
	}

	return loopCoordinates(loop, precisionFunc), nil
}

func PolygonCoordinates(polygon *s2.Polygon, precision int) ([][][][]float64, error) {
	precisionFunc, err := selectPrecisionFunc(precision)
	if err != nil {
		return nil, err
	}

	// Count the number of shells.
	ns := 0
	nl := polygon.NumLoops()
	for i := 0; i < nl; i++ {
		if !polygon.Loop(i).IsHole() {
			ns++
		}
	}

	mpcs := make([][][][]float64, 0, ns)
	for i := 0; i < nl; {
		pcs := make([][][]float64, 0, 1)

		hole := false
		for ; i < nl; i++ {
			loop := polygon.Loop(i)
			if hole && !loop.IsHole() {
				break
			}

			pcs = append(pcs, loopCoordinates(loop, precisionFunc))

			// The next loop, if there is one, should represent a hole.
			hole = true
		}

		mpcs = append(mpcs, pcs)
	}

	return mpcs, nil
}

func unmarshalLatLng(latLng *s2.LatLng, coords []float64) error {
	if d := len(coords); d != 2 {
		return fmt.Errorf("geoutil: cannot process coordinates with dimension %d", d)
	}

	*latLng = s2.LatLngFromDegrees(coords[1], coords[0])
	return nil
}

func unmarshalPoint(point *s2.Point, coords []float64) error {
	latLng := s2.LatLng{}
	if err := unmarshalLatLng(&latLng, coords); err != nil {
		return err
	}

	*point = s2.PointFromLatLng(latLng)
	return nil
}

func unmarshalLoops(loops *[]*s2.Loop, polygonCoords [][][]float64) error {
	shell := len(*loops)
	for _, linearRingCoords := range polygonCoords {
		points := make([]s2.Point, 0, len(linearRingCoords))
		for _, pointCoords := range linearRingCoords {
			point := s2.Point{}
			if err := unmarshalPoint(&point, pointCoords); err != nil {
				return err
			}

			points = append(points, point)
		}

		// S2 loops are not required to repeat the closing point.
		if j := len(points) - 1; points[0] == points[j] {
			points = points[:j]
		}

		// Build the loop and verify the winding order.
		loop := s2.LoopFromPoints(points)

		switch {
		case len(*loops) <= shell:
			loop.Normalize()
		case loop.ContainsPoint((*loops)[shell].Vertex(0)):
			loop.Invert()
		}

		*loops = append(*loops, loop)
	}

	return nil
}

func PointFromPointCoordinates(coords []float64) (s2.Point, error) {
	point := s2.Point{}
	if err := unmarshalPoint(&point, coords); err != nil {
		return s2.Point{}, err
	}

	return point, nil
}

func PolylineFromLineStringCoordinates(coords [][]float64) (*s2.Polyline, error) {
	latLngs := make([]s2.LatLng, len(coords))
	for i, pointCoords := range coords {
		if err := unmarshalLatLng(&latLngs[i], pointCoords); err != nil {
			return nil, err
		}
	}

	return s2.PolylineFromLatLngs(latLngs), nil
}

func PolygonFromPolygonCoordinates(coords [][][]float64) (*s2.Polygon, error) {
	loops := make([]*s2.Loop, 0, len(coords))
	if err := unmarshalLoops(&loops, coords); err != nil {
		return nil, err
	}

	return s2.PolygonFromLoops(loops), nil
}

func PointsFromMultiPointCoordinates(coords [][]float64) ([]s2.Point, error) {
	points := make([]s2.Point, len(coords))
	for i, pointCoords := range coords {
		if err := unmarshalPoint(&points[i], pointCoords); err != nil {
			return nil, err
		}
	}

	return points, nil
}

func PolylinesFromMultiLineStringCoordinates(coords [][][]float64) ([]*s2.Polyline, error) {
	polylines := make([]*s2.Polyline, len(coords))
	for i, lineStringCoords := range coords {
		polyline, err := PolylineFromLineStringCoordinates(lineStringCoords)
		if err != nil {
			return nil, err
		}

		polylines[i] = polyline
	}

	return polylines, nil
}

func PolygonFromMultiPolygonCoordinates(coords [][][][]float64) (*s2.Polygon, error) {
	loops := make([]*s2.Loop, 0, len(coords))
	for _, polygonCoords := range coords {
		if err := unmarshalLoops(&loops, polygonCoords); err != nil {
			return nil, err
		}
	}

	return s2.PolygonFromLoops(loops), nil
}
