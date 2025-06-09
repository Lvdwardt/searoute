package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	geo "github.com/kellydunn/golang-geo"
	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
)

var PortData []Port

func main() {
	skipNowTemp := false
	if skipNowTemp {
		navWarn := GetNavWarnings()

		SaveNavWarnings(navWarn)
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

	// Load the port data
	PortData, err = LoadPorts()
	if err != nil {
		log.Printf("Error loading port data: %v", err)
	}

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

	// Setup route group for the API
	// Handle the index route
	router.GET("/", func(c *gin.Context) {

		c.HTML(200, "home.gohtml", gin.H{})
	})

	// Handle the about page
	router.GET("/about", func(c *gin.Context) {
		c.HTML(200, "about.gohtml", gin.H{})
	})

	// Handle the search for ports
	router.GET("/ports", func(c *gin.Context) {
		// Get the search query
		searchQuery := c.Query("search")
		// Get the filtered port data
		filteredPorts := filterPorts(searchQuery)
		// Send the filtered port data to the client
		c.JSON(200, filteredPorts)
	})

	// Handle request to calculate the passage
	router.POST("/waypoints", func(c *gin.Context) {
		var form map[string]string
		if err := c.Bind(&form); err != nil {
			log.Println(err)
		}

		log.Println("Received form data:", form)

		// Try to get coordinates directly from the form
		originLongStr := form["originLongitude"]
		originLatStr := form["originLatitude"]
		destinationLongStr := form["destinationLongitude"]
		destinationLatStr := form["destinationLatitude"]

		log.Printf("Debug - originLongStr: '%s', originLatStr: '%s'", originLongStr, originLatStr)
		log.Printf("Debug - destinationLongStr: '%s', destinationLatStr: '%s'", destinationLongStr, destinationLatStr)

		var originLong, originLat, destinationLong, destinationLat float64
		var hasOriginCoords, hasDestCoords bool

		log.Printf("About to handle origin coordinates...")
		// Handle origin coordinates
		if originLongStr != "" && originLatStr != "" {
			log.Printf("Found origin coordinate strings, attempting to parse...")
			var err1, err2 error
			originLong, err1 = strconv.ParseFloat(originLongStr, 64)
			originLat, err2 = strconv.ParseFloat(originLatStr, 64)
			if err1 != nil || err2 != nil {
				log.Printf("Error parsing origin coordinates: %v, %v", err1, err2)
				// Try to parse from fromPort as coordinates
				originLong, originLat = parseCoordinatesFromString(form["fromPort"])
				hasOriginCoords = (originLong != 0 || originLat != 0)
				if !hasOriginCoords {
					// Final fallback to port lookup
					originLong, originLat = getPortCoordinates(form["fromPort"])
					hasOriginCoords = (originLong != 0 || originLat != 0)
				}
			} else {
				hasOriginCoords = true
				log.Printf("Successfully parsed origin coordinates: %f, %f", originLong, originLat)
			}
		} else {
			log.Printf("No origin coordinate strings found, trying to parse fromPort as coordinates...")
			// Try to parse fromPort as coordinates first
			originLong, originLat = parseCoordinatesFromString(form["fromPort"])
			hasOriginCoords = (originLong != 0 || originLat != 0)
			if !hasOriginCoords {
				// Fallback to port lookup for origin
				originLong, originLat = getPortCoordinates(form["fromPort"])
				hasOriginCoords = (originLong != 0 || originLat != 0)
			}
		}

		// Handle destination coordinates
		if destinationLongStr != "" && destinationLatStr != "" {
			var err3, err4 error
			destinationLong, err3 = strconv.ParseFloat(destinationLongStr, 64)
			destinationLat, err4 = strconv.ParseFloat(destinationLatStr, 64)
			if err3 != nil || err4 != nil {
				log.Printf("Error parsing destination coordinates: %v, %v", err3, err4)
				// Try to parse from toPort as coordinates
				destinationLong, destinationLat = parseCoordinatesFromString(form["toPort"])
				hasDestCoords = (destinationLong != 0 || destinationLat != 0)
				if !hasDestCoords {
					// Final fallback to port lookup
					destinationLong, destinationLat = getPortCoordinates(form["toPort"])
					hasDestCoords = (destinationLong != 0 || destinationLat != 0)
				}
			} else {
				hasDestCoords = true
				log.Printf("Successfully parsed destination coordinates: %f, %f", destinationLong, destinationLat)
			}
		} else {
			// Try to parse toPort as coordinates first
			destinationLong, destinationLat = parseCoordinatesFromString(form["toPort"])
			hasDestCoords = (destinationLong != 0 || destinationLat != 0)
			if !hasDestCoords {
				// Fallback to port lookup for destination
				destinationLong, destinationLat = getPortCoordinates(form["toPort"])
				hasDestCoords = (destinationLong != 0 || destinationLat != 0)
			}
		}

		fromPort := form["fromPort"]
		toPort := form["toPort"]

		log.Printf("Final Origin: %f, %f (hasCoords: %v)", originLong, originLat, hasOriginCoords)
		log.Printf("Final Destination: %f, %f (hasCoords: %v)", destinationLong, destinationLat, hasDestCoords)

		// Validate coordinates - check if we have valid coordinates
		if !hasOriginCoords {
			c.JSON(400, gin.H{"error": "Invalid origin location"})
			return
		}
		if !hasDestCoords {
			c.JSON(400, gin.H{"error": "Invalid destination location"})
			return
		}

		originCoords := gdj.Position{originLong, originLat}
		destinationCoords := gdj.Position{destinationLong, destinationLat}
		var routeName string
		if fromPort != "" && toPort != "" {
			routeName = fmt.Sprintf("%s -> %s", fromPort, toPort)
		} else {
			routeName = "Custom Coordinates"
		}

		data := calculatePassageInfo(originCoords, destinationCoords, routeName)
		c.JSON(200, data)
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

	// Sample data for test Shanghai-New-York
	//var originCoords = gdj.Position{72.9301, 19.0519}
	//var destinationCoords = gdj.Position{-9.0905, 38.7062}
	//var routeName = "Mumbai-Lisbon"

	// Calculate the passage info
	//calculatePassageInfo(originCoords, destinationCoords, routeName)

}

// CalculatePassageInfo calculates the ocean waypoints and distance between two coordinates and generates a GeoJSON output
func calculatePassageInfo(originCoords, destinationCoords gdj.Position, routeName string) Output {
	// FC variable is the GeoJSON FeatureCollection
	var fc gdj.FeatureCollection
	var newFc gdj.FeatureCollection
	var data []byte
	var splitAvailable bool
	// Check if splitCoords.geojson exists
	if _, err := os.Stat("dataset/splitCoords.geojson"); os.IsNotExist(err) {
		// If not, read the original file
		log.Println("splitCoords.geojson does not exist. Reading original file.")
		data, err = os.ReadFile("dataset/marnet_densified_v2.geojson")
		if err != nil {
			log.Fatal(err)
		}
	} else {
		// If it does, read the splitCoords.geojson file
		splitAvailable = true
		log.Println("splitCoords.geojson exists. Reading splitCoords.geojson file.")
		data, err = os.ReadFile("dataset/splitCoords.geojson")
		if err != nil {
			log.Fatal(err)
		}
	}

	//Unmarshall feature collection from geojson
	err := json.Unmarshal(data, &fc)
	if err != nil {
		log.Fatal(err)
	}

	//log.Println("Split file exists: ", splitAvailable)
	// Do not split if splitCoords.geojson exists
	if !splitAvailable {
		newFc = splitter(fc)
	} else {
		newFc = fc
	}
	//// Print the number of features in the collection
	//log.Printf("Number of features: %d", len(fc.Features))

	// Calculate the shortest path between two points
	path, distance, err := newFc.FindPath(originCoords, destinationCoords, 0.00001)

	distanceInKm := distance / 1000

	// Get the first and last coordinates of the path
	lastWp := path[0]
	firstWp := path[len(path)-1]

	log.Println("First waypoint: ", firstWp)
	log.Println("Last waypoint: ", lastWp)

	// Calculate the gc distance between the origin coordinates and the first waypoint
	distToFirstWp := CalcDistance(originCoords, firstWp)
	// Calculate gc distance between the destination coordinates and the last waypoint
	distFromLastWp := CalcDistance(destinationCoords, lastWp)
	// Total distance of the path
	totalDistance := distToFirstWp + distanceInKm + distFromLastWp

	// Print the path and distance
	log.Printf("Waypoints: %v", path)
	log.Printf("Origin to First Waypoint: %f Km", distToFirstWp)
	log.Printf("Waypoint Distance: %f Km", distanceInKm)
	log.Printf("Last Waypoint to Destination: %f Km", distFromLastWp)
	log.Printf("Total Distance: %f Km", totalDistance)

	//	Generate output geojson
	output := generateOutput(path, originCoords, destinationCoords, totalDistance, distToFirstWp, distFromLastWp, distanceInKm, routeName)

	return output
}

// CalcDistance calculates the distance between two points in meters
func CalcDistance(p1, p2 gdj.Position) float64 {
	wp1 := geo.NewPoint(p1[1], p1[0])
	wp2 := geo.NewPoint(p2[1], p2[0])

	return wp1.GreatCircleDistance(wp2)
}

// filterPorts filters the ports from the []Port struct
func filterPorts(query string) []Port {
	filteredPorts := make([]Port, 0)
	for _, port := range PortData {
		// Check if the search query is a substring of the port name or location
		if strings.Contains(strings.ToLower(port.Port), strings.ToLower(query)) || strings.Contains(strings.ToLower(port.Country), strings.ToLower(query)) {
			filteredPorts = append(filteredPorts, port)
		}
	}
	return filteredPorts
}

// getPortCoordinates returns the coordinates of a port
func getPortCoordinates(portName string) (float64, float64) {
	for _, port := range PortData {
		if port.Port == portName {
			return port.Longitude, port.Latitude
		}
	}
	return 0, 0
}

// parseCoordinatesFromString attempts to parse coordinates from a string like "lat, lng"
func parseCoordinatesFromString(input string) (float64, float64) {
	if input == "" {
		return 0, 0
	}

	// Remove extra whitespace
	input = strings.TrimSpace(input)

	// Try to match format: "lat, lng" or "lat,lng"
	parts := strings.Split(input, ",")
	if len(parts) == 2 {
		latStr := strings.TrimSpace(parts[0])
		lngStr := strings.TrimSpace(parts[1])

		lat, err1 := strconv.ParseFloat(latStr, 64)
		lng, err2 := strconv.ParseFloat(lngStr, 64)

		if err1 == nil && err2 == nil {
			// Validate coordinate ranges
			if lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180 {
				log.Printf("Successfully parsed coordinates from string '%s': %f, %f", input, lat, lng)
				return lng, lat // Return longitude first, latitude second to match function signature
			}
		}
		log.Printf("Failed to parse coordinates from string '%s': lat_err=%v, lng_err=%v", input, err1, err2)
	}

	return 0, 0
}
