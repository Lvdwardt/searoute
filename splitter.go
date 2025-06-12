package main

import (
	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
)

// Splitter is a function that fc with more than 2 pairs of coordinates into fc with 2 pairs of coordinates
func splitter(fc gdj.FeatureCollection) gdj.FeatureCollection {

	var newFc gdj.FeatureCollection
	newFc.Type = "FeatureCollection"

	//	Loop through all features in the feature collection
	for _, feature := range fc.Features {
		// Loop through all coordinates in the feature
		lengthOfCoords := len(feature.Geometry.Coordinates)
		for i := 0; i < lengthOfCoords-1; i++ {
			coord1 := feature.Geometry.Coordinates[i]
			coord2 := feature.Geometry.Coordinates[i+1]

			newFeature := gdj.Feature{
				Type: "Feature",
				Geometry: gdj.Geometry{
					Type:        "LineString",
					Coordinates: []gdj.Position{coord1, coord2},
				},
			}
			newFc.Features = append(newFc.Features, newFeature)

		}
	}
	return newFc
}
