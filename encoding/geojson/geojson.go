package geojson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/golang/geo/s2"

	"github.com/topos-ai/geoutil"
)

var errUndefinedFeatureGeometry = errors.New("geojson: Feature does not define a Geometry")

func marshalPolygon(polygon *s2.Polygon, precision int) ([]byte, error) {
	polygonCoordinates, err := geoutil.PolygonCoordinates(polygon, precision)
	if err != nil {
		return nil, err
	}

	var rg *rawGeometry
	if len(polygonCoordinates) == 1 {
		coordinates, err := json.Marshal(polygonCoordinates[0])
		if err != nil {
			return nil, err
		}

		rg = &rawGeometry{
			Type:        "Polygon",
			Coordinates: coordinates,
		}
	} else {
		coordinates, err := json.Marshal(polygonCoordinates)
		if err != nil {
			return nil, err
		}

		rg = &rawGeometry{
			Type:        "MultiPolygon",
			Coordinates: coordinates,
		}
	}

	return json.Marshal(rg)
}

// Feature represents a GeoJSON Feature object.
type Feature struct {
	ID         interface{}
	Properties map[string]interface{}
	Geometry   interface{}
	Precision  int
}

type rawGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type rawFeature struct {
	ID         interface{}            `json:"id,omitempty"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Geometry   json.RawMessage        `json:"geometry"`
}

func (f *Feature) UnmarshalJSON(data []byte) error {
	rf := &rawFeature{}
	if err := json.Unmarshal(data, rf); err != nil {
		return err
	}

	if rf.Type != "Feature" {
		return fmt.Errorf("geojson: invalid Feature Type value %s", rf.Type)
	}

	if rf.Geometry == nil {
		return errUndefinedFeatureGeometry
	}

	switch rf.ID.(type) {
	case string, float64, nil:
	default:
		return fmt.Errorf("geojson: invalid Feature ID type %T", rf.ID)
	}

	if !bytes.Equal(rf.Geometry, []byte("null")) {
		rg := &rawGeometry{}
		if err := json.Unmarshal(rf.Geometry, rg); err != nil {
			return err
		}

		switch rg.Type {
		case "Point":
			pointCoords := []float64{}
			if err := json.Unmarshal(rg.Coordinates, &pointCoords); err != nil {
				return err
			}

			point, err := geoutil.PointFromPointCoordinates(pointCoords)
			if err != nil {
				return err
			}

			f.Geometry = point

		case "LineString":
			lineStringCoords := [][]float64{}
			if err := json.Unmarshal(rg.Coordinates, &lineStringCoords); err != nil {
				return err
			}

			polyline, err := geoutil.PolylineFromLineStringCoordinates(lineStringCoords)
			if err != nil {
				return err
			}

			f.Geometry = polyline

		case "Polygon":
			polygonCoords := [][][]float64{}
			if err := json.Unmarshal(rg.Coordinates, &polygonCoords); err != nil {
				return err
			}

			polygon, err := geoutil.PolygonFromPolygonCoordinates(polygonCoords)
			if err != nil {
				return err
			}

			f.Geometry = polygon

		case "MultiPoint":
			multipointCoords := [][]float64{}
			if err := json.Unmarshal(rg.Coordinates, &multipointCoords); err != nil {
				return err
			}

			points, err := geoutil.PointsFromMultiPointCoordinates(multipointCoords)
			if err != nil {
				return err
			}

			f.Geometry = points

		case "MultiLineString":
			multiLineStringCoords := [][][]float64{}
			if err := json.Unmarshal(rg.Coordinates, &multiLineStringCoords); err != nil {
				return err
			}

			polylines, err := geoutil.PolylinesFromMultiLineStringCoordinates(multiLineStringCoords)
			if err != nil {
				return err
			}

			f.Geometry = polylines

		case "MultiPolygon":
			multipolygonCoords := [][][][]float64{}
			if err := json.Unmarshal(rg.Coordinates, &multipolygonCoords); err != nil {
				return err
			}

			polygon, err := geoutil.PolygonFromMultiPolygonCoordinates(multipolygonCoords)
			if err != nil {
				return err
			}

			f.Geometry = polygon

		default:
			return fmt.Errorf("geojson: invalid Feature Geometry Type value %s", rf.Geometry)
		}
	}

	f.ID = rf.ID
	f.Properties = rf.Properties
	return nil
}

func (f *Feature) MarshalJSON() ([]byte, error) {
	switch f.ID.(type) {
	case string, float64, nil:
	default:
		return nil, fmt.Errorf("geojson: invalid Feature ID type %T", f.ID)
	}

	rf := rawFeature{
		ID:         f.ID,
		Type:       "Feature",
		Properties: f.Properties,
	}

	if f.Geometry == nil {
		return json.Marshal(rf)
	}

	switch geometry := f.Geometry.(type) {
	case *s2.Polygon:
		data, err := marshalPolygon(geometry, f.Precision)
		if err != nil {
			return nil, err
		}

		rf.Geometry = data

	default:
		return nil, fmt.Errorf("geojson: invalid Feature Geometry type %T", f.Geometry)
	}

	return json.Marshal(rf)
}

// FeatureCollection represents a GeoJSON FeatureCollection object.
type FeatureCollection struct {
	Features []*Feature
}

type rawFeatureCollection struct {
	Type     string          `json:"type"`
	Features json.RawMessage `json:"features,omitempty"`
}

func (fc *FeatureCollection) UnmarshalJSON(data []byte) error {
	rfc := &rawFeatureCollection{}
	if err := json.Unmarshal(data, rfc); err != nil {
		return err
	}

	if rfc.Type != "FeatureCollection" {
		return fmt.Errorf("geojson: invalid FeatureCollection Type value %s", rfc.Type)
	}

	if err := json.Unmarshal(rfc.Features, &fc.Features); err != nil {
		return err
	}

	return nil
}

func (fc *FeatureCollection) MarshalJSON() ([]byte, error) {
	rfc := &rawFeatureCollection{
		Type: "FeatureCollection",
	}

	data, err := json.Marshal(fc.Features)
	if err != nil {
		return nil, err
	}

	rfc.Features = data
	return json.Marshal(rfc)
}
