package main

import (
	"encoding/json"
	"log"
	"os"

	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
)

type Feature struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string   `json:"type"`
		Coordinates gdj.Path `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		OCoords   gdj.Position `json:"o_coords"`
		DCoords   gdj.Position `json:"d_coords"`
		OToWpDist float64      `json:"o_to_wp_dist"`
		WpToDDist float64      `json:"wp_to_d_dist"`
		WpDist    float64      `json:"wp_dist"`
		TotalDist float64      `json:"total_dist"`
		RouteName string       `json:"route_name"`
	} `json:"properties"`
	ID string `json:"id"`
}

type Output struct {
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Features []Feature `json:"features"`
}

func generateOutput(path gdj.Path, oCoords gdj.Position, dCoords gdj.Position, totalDistance float64, oToWpDist float64, wpToDDist float64, Wp2WpDist float64, routeName string) Output {

	if len(path) > 0 {
		log.Printf("First element of path (path[0]): %v", path[0])
		log.Printf("Last element of path (path[%d]): %v", len(path)-1, path[len(path)-1])

		// Print first few and last few waypoints to understand the structure
		if len(path) > 2 {
			log.Printf("Path[0] (supposed to be closest to destination): %v", path[0])
			log.Printf("Path[1]: %v", path[1])
			log.Printf("Path[%d]: %v", len(path)-2, path[len(path)-2])
			log.Printf("Path[%d] (supposed to be closest to origin): %v", len(path)-1, path[len(path)-1])
		}
	}

	// Based on your original code comments:
	// lastWp := path[0]           // closest to destination
	// firstWp := path[len(path)-1] // closest to origin

	var firstWaypoint, lastWaypoint gdj.Position
	if len(path) > 0 {
		firstWaypoint = path[len(path)-1] // Should be closest to origin
		lastWaypoint = path[0]            // Should be closest to destination

		log.Printf("First waypoint (closest to origin): %v", firstWaypoint)
		log.Printf("Last waypoint (closest to destination): %v", lastWaypoint)

		// Verify distances make sense
		distOriginToFirst := CalcDistance(oCoords, firstWaypoint)
		distLastToDest := CalcDistance(lastWaypoint, dCoords)

		log.Printf("Distance from origin to first waypoint: %f km", distOriginToFirst)
		log.Printf("Distance from last waypoint to destination: %f km", distLastToDest)
		log.Printf("Expected oToWpDist: %f km", oToWpDist)
		log.Printf("Expected wpToDDist: %f km", wpToDDist)
	}

	//	Add values to the output struct
	var output Output

	// Create connecting line from origin to first waypoint
	if len(path) > 0 {
		originConnectionFeature := Feature{
			Type: "Feature",
			Geometry: struct {
				Type        string   `json:"type"`
				Coordinates gdj.Path `json:"coordinates"`
			}{
				Type:        "LineString",
				Coordinates: gdj.Path{oCoords, firstWaypoint},
			},
			Properties: struct {
				OCoords   gdj.Position `json:"o_coords"`
				DCoords   gdj.Position `json:"d_coords"`
				OToWpDist float64      `json:"o_to_wp_dist"`
				WpToDDist float64      `json:"wp_to_d_dist"`
				WpDist    float64      `json:"wp_dist"`
				TotalDist float64      `json:"total_dist"`
				RouteName string       `json:"route_name"`
			}{
				OCoords:   oCoords,
				DCoords:   firstWaypoint,
				OToWpDist: oToWpDist,
				WpToDDist: 0,
				WpDist:    0,
				TotalDist: oToWpDist,
				RouteName: routeName + " - Origin Connection",
			},
			ID: "origin_connection",
		}

		log.Printf("Origin connection: %v -> %v", oCoords, firstWaypoint)
		output.Features = append(output.Features, originConnectionFeature)
	}

	// Main water route feature
	mainRouteFeature := Feature{
		Type: "Feature",
		Geometry: struct {
			Type        string   `json:"type"`
			Coordinates gdj.Path `json:"coordinates"`
		}{
			Type:        "LineString",
			Coordinates: path,
		},
		Properties: struct {
			OCoords   gdj.Position `json:"o_coords"`
			DCoords   gdj.Position `json:"d_coords"`
			OToWpDist float64      `json:"o_to_wp_dist"`
			WpToDDist float64      `json:"wp_to_d_dist"`
			WpDist    float64      `json:"wp_dist"`
			TotalDist float64      `json:"total_dist"`
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
		ID: "main_route",
	}

	log.Printf("Main route has %d waypoints", len(path))
	output.Features = append(output.Features, mainRouteFeature)

	// Create connecting line from last waypoint to destination
	if len(path) > 0 {
		destinationConnectionFeature := Feature{
			Type: "Feature",
			Geometry: struct {
				Type        string   `json:"type"`
				Coordinates gdj.Path `json:"coordinates"`
			}{
				Type:        "LineString",
				Coordinates: gdj.Path{lastWaypoint, dCoords},
			},
			Properties: struct {
				OCoords   gdj.Position `json:"o_coords"`
				DCoords   gdj.Position `json:"d_coords"`
				OToWpDist float64      `json:"o_to_wp_dist"`
				WpToDDist float64      `json:"wp_to_d_dist"`
				WpDist    float64      `json:"wp_dist"`
				TotalDist float64      `json:"total_dist"`
				RouteName string       `json:"route_name"`
			}{
				OCoords:   lastWaypoint,
				DCoords:   dCoords,
				OToWpDist: 0,
				WpToDDist: wpToDDist,
				WpDist:    0,
				TotalDist: wpToDDist,
				RouteName: routeName + " - Destination Connection",
			},
			ID: "destination_connection",
		}

		log.Printf("Destination connection: %v -> %v", lastWaypoint, dCoords)
		output.Features = append(output.Features, destinationConnectionFeature)
	}

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
