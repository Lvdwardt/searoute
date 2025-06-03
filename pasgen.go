package main

import (
	"encoding/json"
	"os"

	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
)

// Feature struct to hold the GeoJSON feature data
type Feature struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string   `json:"type"`
		Coordinates gdj.Path `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		OCoords   gdj.Position `json:"origin_port_coords"`
		DCoords   gdj.Position `json:"destination_port_coords"`
		OToWpDist float64      `json:"origin_port_to_nearest_common_wp_dist_in_km"`
		WpToDDist float64      `json:"common_wp_to_destination_port_dist_in_km"`
		WpDist    float64      `json:"waypoint_to_waypoint_dist_in_km"`
		TotalDist float64      `json:"total_dist_from_origin_port_to_destination_port_in_km"`
		RouteName string       `json:"route_name"`
	} `json:"properties"`
	ID string `json:"id"`
}

// Output struct to hold the final output in GeoJSON format
type Output struct {
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Features []Feature `json:"features"`
}

func generateOutput(path gdj.Path, oCoords gdj.Position, dCoords gdj.Position, totalDistance float64, oToWpDist float64, wpToDDist float64, Wp2WpDist float64, routeName string) Output {
	//	Add values to the output struct
	var output Output
	newFeature := Feature{
		Type: "Feature",
		Geometry: struct {
			Type        string   `json:"type"`
			Coordinates gdj.Path `json:"coordinates"`
		}{
			Type:        "LineString",
			Coordinates: path,
		},
		Properties: struct {
			OCoords   gdj.Position `json:"origin_port_coords"`
			DCoords   gdj.Position `json:"destination_port_coords"`
			OToWpDist float64      `json:"origin_port_to_nearest_common_wp_dist_in_km"`
			WpToDDist float64      `json:"common_wp_to_destination_port_dist_in_km"`
			WpDist    float64      `json:"waypoint_to_waypoint_dist_in_km"`
			TotalDist float64      `json:"total_dist_from_origin_port_to_destination_port_in_km"`
			RouteName string       `json:"route_name"`
		}{
			OCoords:   oCoords,
			DCoords:   dCoords,
			OToWpDist: oToWpDist,
			WpToDDist: wpToDDist,
			WpDist:    Wp2WpDist,
			TotalDist: totalDistance,
			RouteName: routeName,
		},
		ID: "1",
	}
	output.Features = append(output.Features, newFeature)
	output.Type = "FeatureCollection"
	output.Name = "Short Sea Route"

	writeOutput(output, "temp/output.geojson")
	return output
}

// Generic Function to write the output to a file
func writeOutput(output interface{}, filename string) {
	//	Convert the output struct to json
	jsonOutput, err := json.Marshal(output)
	if err != nil {
		panic(err)
	}

	//	Create a file and write the json to it
	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	_, err = file.Write(jsonOutput)
	if err != nil {
		panic(err)
	}
}
