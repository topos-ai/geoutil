package wkb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/golang/geo/s2"
)

func encodePointFromLatLng(w io.Writer, latLng s2.LatLng) error {
	if err := binary.Write(w, binary.BigEndian, latLng.Lng.Degrees()); err != nil {
		return err
	}

	if err := binary.Write(w, binary.BigEndian, latLng.Lat.Degrees()); err != nil {
		return err
	}

	return nil
}

func encodePoint(w io.Writer, point s2.Point) error {
	return encodePointFromLatLng(w, s2.LatLngFromPoint(point))
}

func encodeLinearRing(w io.Writer, loop *s2.Loop) error {
	np := loop.NumVertices() + 1

	// Number of points.
	if err := binary.Write(w, binary.BigEndian, uint32(np)); err != nil {
		return err
	}

	for i := 0; i < np; i++ {
		if err := encodePoint(w, loop.OrientedVertex(i)); err != nil {
			return err
		}
	}

	return nil
}

type writer interface {
	io.ByteWriter
	io.Writer
}

func encodeWKBPoint(w writer, point s2.Point) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbPoint); err != nil {
		return err
	}

	return encodePoint(w, point)
}

func encodeWKBPointFromLatLng(w writer, latLng s2.LatLng) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbPoint); err != nil {
		return err
	}

	return encodePointFromLatLng(w, latLng)
}

func encodeWKBLineString(w writer, polyline *s2.Polyline) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbLineString); err != nil {
		return err
	}

	// Number of points.
	if err := binary.Write(w, binary.BigEndian, uint32(len(*polyline))); err != nil {
		return err
	}

	for _, point := range *polyline {
		if err := encodePoint(w, point); err != nil {
			return err
		}
	}

	return nil
}

func encodeWKBPolygon(w writer, loops []*s2.Loop) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbPolygon); err != nil {
		return err
	}

	// Number of linear rings.
	if err := binary.Write(w, binary.BigEndian, uint32(len(loops))); err != nil {
		return err
	}

	for _, loop := range loops {
		if err := encodeLinearRing(w, loop); err != nil {
			return nil
		}
	}

	return nil
}

func encodeWKBMultiPoint(w writer, points []s2.Point) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbMultiPoint); err != nil {
		return err
	}

	// Number of points.
	if err := binary.Write(w, binary.BigEndian, uint32(len(points))); err != nil {
		return err
	}

	for _, point := range points {
		if err := encodeWKBPoint(w, point); err != nil {
			return err
		}
	}

	return nil
}

func encodeWKBMultiLineString(w writer, polylines []*s2.Polyline) error {

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbMultiLineString); err != nil {
		return err
	}

	// Number of line strings.
	if err := binary.Write(w, binary.BigEndian, uint32(len(polylines))); err != nil {
		return err
	}

	for _, polyline := range polylines {
		if err := encodeWKBLineString(w, polyline); err != nil {
			return err
		}
	}

	return nil
}

func encodeWKBMultiPolygon(w writer, polygon *s2.Polygon) error {

	// Count the number of shells. The number of shells is the number of polygons
	// required in the WKB representation of the geometry.
	ns := 0
	nl := polygon.NumLoops()
	for i := 0; i < nl; i++ {
		if !polygon.Loop(i).IsHole() {
			ns++
		}
	}

	if ns <= 1 {
		return encodeWKBPolygon(w, polygon.Loops())
	}

	// Endianess.
	if err := w.WriteByte(wkbXDR); err != nil {
		return err
	}

	// Geometry type.
	if err := binary.Write(w, binary.BigEndian, wkbMultiPolygon); err != nil {
		return err
	}

	// Number of polygons.
	if err := binary.Write(w, binary.BigEndian, uint32(ns)); err != nil {
		return err
	}

	loops := polygon.Loops()
	for i := 0; i < nl; {
		j := i + 1
		for ; j < nl && polygon.Loop(j).IsHole(); j++ {
		}

		if err := encodeWKBPolygon(w, loops[i:j]); err != nil {
			return err
		}

		i = j
	}

	return nil
}

type byteWriter struct {
	w io.Writer
}

func newByteWriter(w io.Writer) *byteWriter {
	return &byteWriter{
		w: w,
	}
}

func (bw *byteWriter) Write(p []byte) (n int, err error) {
	return bw.w.Write(p)
}

func (bw *byteWriter) WriteByte(c byte) error {
	_, err := bw.w.Write([]byte{c})
	return err
}

type Encoder struct {
	w writer
}

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{}
	if bw, ok := w.(writer); ok {
		e.w = bw
	} else {
		e.w = newByteWriter(w)
	}

	return e
}

func (e *Encoder) Encode(v interface{}) error {
	switch geometry := v.(type) {
	case s2.LatLng:
		return encodeWKBPointFromLatLng(e.w, geometry)
	case s2.Point:
		return encodeWKBPoint(e.w, geometry)
	case *s2.Polyline:
		return encodeWKBLineString(e.w, geometry)
	case []s2.Point:
		return encodeWKBMultiPoint(e.w, geometry)
	case []*s2.Polyline:
		return encodeWKBMultiLineString(e.w, geometry)
	case *s2.Polygon:
		return encodeWKBMultiPolygon(e.w, geometry)
	default:
		return fmt.Errorf("wkb: unknown geometry type %T", v)
	}
}

func Marshal(v interface{}) ([]byte, error) {
	w := bytes.NewBuffer([]byte{})
	if err := NewEncoder(w).Encode(v); err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}
