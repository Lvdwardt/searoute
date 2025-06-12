package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"

	"github.com/gin-gonic/gin"
	geo "github.com/kellydunn/golang-geo"
	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
	"github.com/joho/godotenv"
)

type Coordinate struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

type MultiRouteRequest struct {
	Coordinates []Coordinate `json:"coordinates"`
}

type MultiRouteResponse struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates interface{}     `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		TotalDistance float64 `json:"total_distance"`
		RouteCount    int     `json:"route_count"`
	} `json:"properties"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}

	// Create a temp folder
	if _, err := os.Stat("temp"); os.IsNotExist(err) {
		err := os.MkdirAll("temp", 0777)
		if err != nil {
			return
		}
	}

	// Setting the logger
	f, err := os.OpenFile("temp/runtime.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(f)

	wrt := io.MultiWriter(os.Stdout, f)
	log.SetOutput(wrt)

	// Set the router as the default one provided by Gin
	router := gin.Default()

	err = router.SetTrustedProxies(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Serve html template files
	router.LoadHTMLGlob("web/templates/**/*.gohtml")
	// Load the static files
	router.Static("/static", "./web/static")

	// Simple debugging interface
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "home.gohtml", gin.H{})
	})

	// Handle request to calculate routes for multiple coordinates
	router.POST("/multi-routes", func(c *gin.Context) {
		var request MultiRouteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(400, gin.H{"error": "Invalid JSON format: " + err.Error()})
			return
		}

		if len(request.Coordinates) < 2 {
			c.JSON(400, gin.H{"error": "At least 2 coordinates are required"})
			return
		}

		// Calculate the complete route through all coordinates
		response, err := calculateMultiRoute(request.Coordinates)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to calculate route: " + err.Error()})
			return
		}

		c.JSON(200, response)
	})

	//Start and run the server if production environment
	if os.Getenv("APP_ENV") == "prod" {
		log.Println("Starting server in production environment")
		err = router.RunTLS(fmt.Sprintf(":%s", os.Getenv("PORT")), os.Getenv("CERT_PATH"), os.Getenv("KEY_PATH"))
	} else {
		log.Println("Starting server in development environment")
		err = router.Run(fmt.Sprintf(":%s", os.Getenv("PORT")))
		if err != nil {
			log.Fatal(err)
		}
	}
}

// normalizeLongitude normalizes longitude to handle dateline crossing
func normalizeLongitude(lon float64) float64 {
	for lon > 180 {
		lon -= 360
	}
	for lon < -180 {
		lon += 360
	}
	return lon
}

// adjustCoordinatesForDateline adjusts coordinates to minimize distance across dateline
func adjustCoordinatesForDateline(origin, destination gdj.Position) (gdj.Position, gdj.Position) {
	origLon := origin[0]
	destLon := destination[0]
	
	// Calculate the direct difference
	directDiff := destLon - origLon
	
	// If the difference is greater than 180, it's shorter to go the other way
	if directDiff > 180 {
		// Destination is too far east, adjust it west
		destLon -= 360
	} else if directDiff < -180 {
		// Destination is too far west, adjust it east
		destLon += 360
	}
	
	return gdj.Position{origLon, origin[1]}, gdj.Position{destLon, destination[1]}
}

// splitCoordinatesAtDateline splits coordinates into separate line segments at dateline crossings
func splitCoordinatesAtDateline(coordinates [][]float64) [][][]float64 {
	if len(coordinates) < 2 {
		return [][][]float64{coordinates}
	}
	
	var segments [][][]float64
	var currentSegment [][]float64
	
	// Add first coordinate
	currentSegment = append(currentSegment, coordinates[0])
	
	for i := 1; i < len(coordinates); i++ {
		prevLon := coordinates[i-1][0]
		currLon := coordinates[i][0]
		
		// Check if this segment crosses the dateline
		lonDiff := currLon - prevLon
		
		// Debug logging for suspicious longitude differences
		if lonDiff > 100 || lonDiff < -100 {
			log.Printf("Large longitude difference detected: %.3f -> %.3f (diff: %.3f)", prevLon, currLon, lonDiff)
		}
		
		// Check for exact dateline transitions
		isDatelineCrossing := (prevLon == 180 && currLon == -180) || (prevLon == -180 && currLon == 180)
		
		// Also check for the specific case of 180 to -180 transition
		if lonDiff > 180 || lonDiff < -180 || isDatelineCrossing {
			// This is a dateline crossing - end current segment and start new one
			log.Printf("Detected dateline crossing between points %d and %d (%.3f -> %.3f)", i-1, i, prevLon, currLon)
			
			// Finish current segment
			if len(currentSegment) > 1 {
				segments = append(segments, currentSegment)
			}
			
			// Start new segment with current coordinate
			currentSegment = [][]float64{coordinates[i]}
		} else {
			// Normal segment - add coordinate
			currentSegment = append(currentSegment, coordinates[i])
		}
	}
	
	// Add final segment if it has coordinates
	if len(currentSegment) > 1 {
		segments = append(segments, currentSegment)
	}
	
	log.Printf("Split route into %d segments", len(segments))
	return segments
}

// calculateMultiRoute calculates routes between all consecutive coordinate pairs and returns a single linestring
func calculateMultiRoute(coordinates []Coordinate) (MultiRouteResponse, error) {
	var response MultiRouteResponse
	var allCoordinates [][]float64
	var totalDistance float64

	log.Println("Loading GeoJSON data...")

	// Define split file path
	splitFilePath := "dataset/splitCoords.geojson"
	
	// Check if split file exists, if not create it
	if _, err := os.Stat(splitFilePath); os.IsNotExist(err) {
		log.Println("Split file does not exist. Creating split coordinates file...")
		
		// Read original file
		originalData, err := os.ReadFile("dataset/searoutes-v1.geojson")
		if err != nil {
			return response, fmt.Errorf("failed to read original geojson file: %v", err)
		}

		// Unmarshal original feature collection
		var originalFc gdj.FeatureCollection
		err = json.Unmarshal(originalData, &originalFc)
		if err != nil {
			return response, fmt.Errorf("failed to unmarshal original geojson: %v", err)
		}

		// Split the coordinates
		splitFc := splitter(originalFc)

		// Save split coordinates to file
		splitData, err := json.Marshal(splitFc)
		if err != nil {
			return response, fmt.Errorf("failed to marshal split coordinates: %v", err)
		}

		err = os.WriteFile(splitFilePath, splitData, 0644)
		if err != nil {
			return response, fmt.Errorf("failed to write split coordinates file: %v", err)
		}

		log.Println("Split coordinates file created successfully.")
	} else {
		log.Println("Split coordinates file exists. Using existing file.")
	}

	// Always load from split file
	data, err := os.ReadFile(splitFilePath)
	if err != nil {
		return response, fmt.Errorf("failed to read split coordinates file: %v", err)
	}

	// Unmarshal feature collection from split geojson
	var fc gdj.FeatureCollection
	err = json.Unmarshal(data, &fc)
	if err != nil {
		return response, fmt.Errorf("failed to unmarshal split geojson: %v", err)
	}

	log.Printf("Loaded feature collection with %d features", len(fc.Features))

	// Add first coordinate (normalized)
	allCoordinates = append(allCoordinates, []float64{normalizeLongitude(coordinates[0].Lon), coordinates[0].Lat})

	// Calculate route between each consecutive pair of coordinates
	for i := 0; i < len(coordinates)-1; i++ {
		origin := gdj.Position{normalizeLongitude(coordinates[i].Lon), coordinates[i].Lat}
		destination := gdj.Position{normalizeLongitude(coordinates[i+1].Lon), coordinates[i+1].Lat}

		log.Printf("Calculating route from %v to %v", origin, destination)
		log.Printf("Origin longitude: %f, Destination longitude: %f", origin[0], destination[0])
		
		// Validate coordinates before processing
		if !isValidCoordinate(origin) || !isValidCoordinate(destination) {
			return response, fmt.Errorf("invalid coordinates detected: origin %v, destination %v", origin, destination)
		}
		
		// Check if this crosses the dateline
		lonDiff := destination[0] - origin[0]
		crossesDateline := lonDiff > 180 || lonDiff < -180
		
		if crossesDateline {
			log.Printf("WARNING: Route crosses dateline. Longitude difference: %f", lonDiff)
			// For dateline crossings near the 180/-180 boundary, use direct connection
			if isNearDateline(origin[0]) || isNearDateline(destination[0]) {
				log.Printf("DATELINE HANDLING: Using direct connection for coordinates near dateline")
				// Add destination coordinate directly
				allCoordinates = append(allCoordinates, []float64{normalizeLongitude(destination[0]), destination[1]})
				
				distance := CalcDistance(origin, destination)
				log.Printf("Direct path created with distance %f meters", distance)
				
				distanceInKm := distance / 1000
				totalDistance += distanceInKm
				continue
			}
			
			// Adjust coordinates to handle dateline crossing
			origin, destination = adjustCoordinatesForDateline(origin, destination)
			log.Printf("Adjusted coordinates - Origin: %v, Destination: %v", origin, destination)
		}

		// Calculate the shortest path between two points
		path, distance, err := fc.FindPath(origin, destination, 0.0000000001)
		if err != nil {
			log.Printf("ERROR: FindPath failed with error: %v", err)
			log.Printf("ERROR: Origin coordinates: [%f, %f]", origin[0], origin[1])
			log.Printf("ERROR: Destination coordinates: [%f, %f]", destination[0], destination[1])
			
			// Enhanced fallback for various error scenarios
			log.Printf("FALLBACK: Using direct line connection")
			// Add destination coordinate directly for fallback
			allCoordinates = append(allCoordinates, []float64{normalizeLongitude(destination[0]), destination[1]})
			distance = CalcDistance(origin, destination)
			distanceInKm := distance / 1000
			totalDistance += distanceInKm
			continue
		}

		log.Printf("Found path with %d waypoints and distance %f meters", len(path), distance)

		distanceInKm := distance / 1000
		totalDistance += distanceInKm

		// Add connecting segments to ensure complete route
		if len(path) > 0 {
			firstWp := path[len(path)-1] // closest to origin
			lastWp := path[0]            // closest to destination
			
			// Calculate and add distances for connection segments
			distToFirstWp := CalcDistance(origin, firstWp)
			distFromLastWp := CalcDistance(lastWp, destination)
			totalDistance += (distToFirstWp + distFromLastWp) / 1000 // Convert to km
			
			// Add waypoints from pathfinding (in reverse order as they come destination to origin)
			for j := len(path) - 1; j >= 0; j-- {
				normalizedLon := normalizeLongitude(path[j][0])
				allCoordinates = append(allCoordinates, []float64{normalizedLon, path[j][1]})
			}
		}
		
		// Always add the destination port coordinate to ensure we reach the exact port location
		allCoordinates = append(allCoordinates, []float64{normalizeLongitude(destination[0]), destination[1]})
	}

	// Split coordinates at dateline crossings
	log.Printf("Before splitting: checking %d coordinates for dateline crossings", len(allCoordinates))
	segments := splitCoordinatesAtDateline(allCoordinates)
	
	// Build response
	response.Type = "Feature"
	if len(segments) == 1 {
		// Single segment - use LineString
		response.Geometry.Type = "LineString"
		response.Geometry.Coordinates = segments[0]
	} else {
		// Multiple segments - use MultiLineString
		response.Geometry.Type = "MultiLineString"
		response.Geometry.Coordinates = segments
	}
	response.Properties.TotalDistance = totalDistance
	response.Properties.RouteCount = len(coordinates) - 1

	log.Printf("Multi-route calculation complete. Total distance: %f km, %d route segments, %d geometry segments", totalDistance, len(coordinates)-1, len(segments))

	return response, nil
}

// CalcDistance calculates the distance between two points in meters
func CalcDistance(p1, p2 gdj.Position) float64 {
	wp1 := geo.NewPoint(p1[1], p1[0])
	wp2 := geo.NewPoint(p2[1], p2[0])

	return wp1.GreatCircleDistance(wp2)
}

// isValidCoordinate checks if a coordinate is valid
func isValidCoordinate(pos gdj.Position) bool {
	if len(pos) < 2 {
		return false
	}
	lon := pos[0]
	lat := pos[1]
	
	// Check for NaN or infinite values
	if math.IsNaN(lon) || math.IsInf(lon, 0) || math.IsNaN(lat) || math.IsInf(lat, 0) {
		return false
	}
	
	// Check coordinate bounds
	return lon >= -180 && lon <= 180 && lat >= -90 && lat <= 90
}

// isNearDateline checks if a longitude is very close to the International Date Line
func isNearDateline(lon float64) bool {
	// Consider coordinates within 5 degrees of the dateline as "near"
	return lon > 175 || lon < -175
}
