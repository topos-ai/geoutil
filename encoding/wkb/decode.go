package wkb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/golang/geo/s2"
)

func decodeOrder(r io.ByteReader) (binary.ByteOrder, error) {
	orderByte, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	var order binary.ByteOrder
	switch orderByte {
	case wkbXDR:
		order = binary.BigEndian
	case wkbNDR:
		order = binary.LittleEndian
	default:
		return nil, fmt.Errorf("wkb: unknown byte order %d", orderByte)
	}

	return order, nil
}

func decodeGeometryType(r io.Reader, order binary.ByteOrder) (uint32, error) {
	var geometryType uint32
	if err := binary.Read(r, order, &geometryType); err != nil {
		return 0, err
	}

	return geometryType, nil
}

func verifyGeometryType(r io.Reader, order binary.ByteOrder, expectedGeometryType uint32) error {
	geometryType, err := decodeGeometryType(r, order)
	if err != nil {
		return err
	}

	if geometryType != expectedGeometryType {
		return fmt.Errorf("wkb: invalid geometry type %d, expected %d", geometryType, expectedGeometryType)
	}

	return nil
}

func decodePoint(r io.Reader, order binary.ByteOrder) (s2.LatLng, error) {
	var lng float64
	if err := binary.Read(r, order, &lng); err != nil {
		return s2.LatLng{}, err
	}

	var lat float64
	if err := binary.Read(r, order, &lat); err != nil {
		return s2.LatLng{}, err
	}

	return s2.LatLngFromDegrees(lat, lng), nil
}

func decodeLinearRing(r io.Reader, order binary.ByteOrder) (*s2.Loop, error) {
	var n uint32
	if err := binary.Read(r, order, &n); err != nil {
		return nil, err
	}

	points := make([]s2.Point, n)
	for k := range points {
		latLng, err := decodePoint(r, order)
		if err != nil {
			return nil, err
		}

		points[k] = s2.PointFromLatLng(latLng)
	}

	// S2 does not require that the last two points of a linear ring be equal.
	if l := len(points) - 1; points[0] == points[l] {
		points = points[:l]
	}

	return s2.LoopFromPoints(points), nil
}

type reader interface {
	io.ByteReader
	io.Reader
}

func decodeWKBPoint(r reader) (s2.LatLng, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return s2.LatLng{}, err
	}

	if err := verifyGeometryType(r, order, wkbPoint); err != nil {
		return s2.LatLng{}, err
	}

	return decodePoint(r, order)
}

func decodeWKBLineString(r reader) (*s2.Polyline, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return nil, err
	}

	if err := verifyGeometryType(r, order, wkbLineString); err != nil {
		return nil, err
	}

	var n uint32
	if err := binary.Read(r, order, &n); err != nil {
		return nil, err
	}

	latLngs := make([]s2.LatLng, n)
	for i := range latLngs {
		latLng, err := decodePoint(r, order)
		if err != nil {
			return nil, err
		}

		latLngs[i] = latLng
	}

	return s2.PolylineFromLatLngs(latLngs), nil
}

func decodePolygonLoopsPartial(r io.Reader, order binary.ByteOrder) ([]*s2.Loop, error) {
	var nlr uint32
	if err := binary.Read(r, order, &nlr); err != nil {
		return nil, err
	}

	polygonLoops := make([]*s2.Loop, nlr)
	for j := range polygonLoops {

		// Build the loop and verify the winding order.
		loop, err := decodeLinearRing(r, order)
		if err != nil {
			return nil, err
		}

		switch {
		case j == 0:
			loop.Normalize()
		case loop.ContainsPoint(polygonLoops[0].Vertex(1)):
			loop.Invert()
		}

		polygonLoops[j] = loop
	}

	return polygonLoops, nil
}

func decodeWKBPolygonLoops(r reader) ([]*s2.Loop, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return nil, err
	}

	if err := verifyGeometryType(r, order, wkbPolygon); err != nil {
		return nil, err
	}

	return decodePolygonLoopsPartial(r, order)
}

func decodeWKBPolygonPartial(r reader, order binary.ByteOrder) (*s2.Polygon, error) {
	polygonLoops, err := decodePolygonLoopsPartial(r, order)
	if err != nil {
		return nil, err
	}

	return s2.PolygonFromLoops(polygonLoops), nil
}

func decodeWKBMultiPoint(r reader) ([]s2.Point, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return nil, err
	}

	if err := verifyGeometryType(r, order, wkbMultiPoint); err != nil {
		return nil, err
	}

	var n uint32
	if err := binary.Read(r, order, &n); err != nil {
		return nil, err
	}

	points := make([]s2.Point, n)
	for i := range points {
		point, err := decodeWKBPoint(r)
		if err != nil {
			return nil, err
		}

		points[i] = s2.PointFromLatLng(point)
	}

	return points, nil
}

func decodeWKBMultiLineString(r reader) ([]*s2.Polyline, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return nil, err
	}

	if err := verifyGeometryType(r, order, wkbMultiLineString); err != nil {
		return nil, err
	}

	var n uint32
	if err := binary.Read(r, order, &n); err != nil {
		return nil, err
	}

	polylines := make([]*s2.Polyline, n)
	for i := range polylines {
		polyline, err := decodeWKBLineString(r)
		if err != nil {
			return nil, err
		}

		polylines[i] = polyline
	}

	return polylines, nil
}

func decodeWKBMultiPolygonPartial(r reader, order binary.ByteOrder) (*s2.Polygon, error) {
	var n uint32
	if err := binary.Read(r, order, &n); err != nil {
		return nil, err
	}

	multiPolygonLoops := make([]*s2.Loop, 0, n)
	for i := uint32(0); i < n; i++ {
		polygonLoops, err := decodeWKBPolygonLoops(r)
		if err != nil {
			return nil, err
		}

		multiPolygonLoops = append(multiPolygonLoops, polygonLoops...)
	}

	return s2.PolygonFromLoops(multiPolygonLoops), nil
}

func decodeWKBPolygonOrMultiPolygon(r reader) (*s2.Polygon, error) {
	order, err := decodeOrder(r)
	if err != nil {
		return nil, err
	}

	geometryType, err := decodeGeometryType(r, order)
	if err != nil {
		return nil, err
	}

	switch geometryType {
	case wkbPolygon:
		return decodeWKBPolygonPartial(r, order)
	case wkbMultiPolygon:
		return decodeWKBMultiPolygonPartial(r, order)
	default:
		return nil, fmt.Errorf("wkb: invalid geometry type %d, expected %d or %d", geometryType, wkbPolygon, wkbMultiPolygon)
	}
}

func Unmarshal(data []byte, v interface{}) error {
	r := bytes.NewReader(data)
	switch geometry := v.(type) {
	case *s2.LatLng:
		latLng, err := decodeWKBPoint(r)
		if err != nil {
			return err
		}

		*geometry = latLng

	case *s2.Point:
		latLng, err := decodeWKBPoint(r)
		if err != nil {
			return err
		}

		*geometry = s2.PointFromLatLng(latLng)

	case *s2.Polyline:
		lineString, err := decodeWKBLineString(r)
		if err != nil {
			return err
		}

		*geometry = *lineString

	case *s2.Polygon:
		polygon, err := decodeWKBPolygonOrMultiPolygon(r)
		if err != nil {
			return err
		}

		*geometry = *polygon

	case *[]s2.Point:
		multiPoint, err := decodeWKBMultiPoint(r)
		if err != nil {
			return err
		}

		*geometry = multiPoint

	case *[]*s2.Polyline:
		multiLineString, err := decodeWKBMultiLineString(r)
		if err != nil {
			return err
		}

		*geometry = multiLineString
	}

	return nil
}
