package main

import (
	"encoding/json"
	"fmt"
	"context"
	"github.com/gin-gonic/gin"
	geo "github.com/kellydunn/golang-geo"
	"github.com/mark3labs/mcp-go/mcp"
	mcpServer "github.com/mark3labs/mcp-go/server"
	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
	"io"
	"log"
	"os"
	"strings"
)

var PortData []Port

// seaRouteTool defines the MCP tool for calculating sea routes.
var seaRouteTool = mcp.NewTool("calculateSeaRoute",
	mcp.WithDescription("Calculates the shortest sea route between two ports given their names or coordinates."),
	mcp.WithString("origin_port_name", mcp.Description("Name of the origin port (optional if coordinates provided)."), mcp.Optional()),
	mcp.WithString("destination_port_name", mcp.Description("Name of the destination port (optional if coordinates provided)."), mcp.Optional()),
	mcp.WithNumber("origin_latitude", mcp.Description("Latitude of the origin port (optional if port name provided)."), mcp.Optional()),
	mcp.WithNumber("origin_longitude", mcp.Description("Longitude of the origin port (optional if port name provided)."), mcp.Optional()),
	mcp.WithNumber("destination_latitude", mcp.Description("Latitude of the destination port (optional if port name provided)."), mcp.Optional()),
	mcp.WithNumber("destination_longitude", mcp.Description("Longitude of the destination port (optional if port name provided)."), mcp.Optional()),
)

// seaRouteToolHandler implements the logic for the calculateSeaRoute tool.
func seaRouteToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var originCoords gdj.Position
	var destinationCoords gdj.Position
	var routeName string

	originPortName, oPortNameOk := request.GetString("origin_port_name")
	destPortName, dPortNameOk := request.GetString("destination_port_name")

	originLat, oLatOk := request.GetFloat("origin_latitude")
	originLon, oLonOk := request.GetFloat("origin_longitude")
	destLat, dLatOk := request.GetFloat("destination_latitude")
	destLon, dLonOk := request.GetFloat("destination_longitude")

	if oPortNameOk && dPortNameOk && originPortName != "" && destPortName != "" {
		log.Printf("Processing MCP request by port names: %s to %s", originPortName, destPortName)
		oLon, oLat := getPortCoordinates(originPortName)
		dLon, dLat := getPortCoordinates(destPortName)

		if oLon == 0 && oLat == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Origin port not found: %s", originPortName)), nil
		}
		if dLon == 0 && dLat == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Destination port not found: %s", destPortName)), nil
		}
		originCoords = gdj.Position{oLon, oLat}
		destinationCoords = gdj.Position{dLon, dLat}
		routeName = originPortName + " to " + destPortName
	} else if oLatOk && oLonOk && dLatOk && dLonOk {
		log.Printf("Processing MCP request by coordinates: [%f,%f] to [%f,%f]", originLon, originLat, destLon, destLat)
		originCoords = gdj.Position{originLon, originLat}
		destinationCoords = gdj.Position{destLon, destLat}
		routeName = fmt.Sprintf("[%f,%f] to [%f,%f]", originLon, originLat, destLon, destLat)
	} else {
		return mcp.NewToolResultError("Insufficient parameters. Provide origin/destination port names or full coordinates (latitude and longitude for both)."), nil
	}

	outputData, err := calculatePassageInfo(originCoords, destinationCoords, routeName)
	if err != nil {
		log.Printf("Error calculating passage info: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Error during route calculation: %v", err.Error())), nil
	}

	jsonBytes, err := json.Marshal(outputData)
	if err != nil {
		log.Printf("Error marshalling result: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize route result: %v", err.Error())), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

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

	// Initialize MCP Server
	mcpSrv := mcpServer.NewMCPServer(
		"SeaRouteCalculator",
		"1.0.0",
		// mcpServer.WithToolCapabilities(false), // Example of an option
	)
	mcpSrv.AddTool(seaRouteTool, seaRouteToolHandler)

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
			// Handle error
			log.Println(err)
		}

		log.Println(form)

		//originLongi := form["originLongitude"]
		//originLati := form["originLatitude"]
		//destinationLongi := form["destinationLongitude"]
		//destinationLati := form["destinationLatitude"]

		fromPort := form["fromPort"]
		toPort := form["toPort"]

		// Get the origin coordinates
		originLong, originLat := getPortCoordinates(fromPort)
		// Get the destination coordinates
		destinationLong, destinationLat := getPortCoordinates(toPort)

		// Convert all the waypoints to float64
		//originLong, _ := strconv.ParseFloat(originLong, 64)
		//originLat, _ := strconv.ParseFloat(originLati, 64)
		//destinationLong, _ := strconv.ParseFloat(destinationLongi, 64)
		//destinationLat, _ := strconv.ParseFloat(destinationLati, 64)

		// Print the coordinates received from form
		log.Printf("Origin: %f, %f", originLong, originLat)
		log.Printf("Destination: %f, %f", destinationLong, destinationLat)

		// Get the origin coordinates
		originCoords := gdj.Position{originLong, originLat}
		// Get the destination coordinates
		destinationCoords := gdj.Position{destinationLong, destinationLat}
		// Set route name
		routeName := fmt.Sprintf("%s -> %s", fromPort, toPort)

		// Call the function to calculate the passage
		data := calculatePassageInfo(originCoords, destinationCoords, routeName)

		// Send the data to the client
		c.JSON(200, data)

	})

	// Handle MCP request
	// router.POST("/mcp", mcpHandler) // Old handler
	router.POST("/mcp", gin.WrapH(mcpSrv.HTTPHandler())) // New MCP server handler

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
func calculatePassageInfo(originCoords, destinationCoords gdj.Position, routeName string) (Output, error) {
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
			return Output{}, fmt.Errorf("error reading marnet_densified_v2.geojson: %w", err)
		}
	} else {
		// If it does, read the splitCoords.geojson file
		splitAvailable = true
		log.Println("splitCoords.geojson exists. Reading splitCoords.geojson file.")
		data, err = os.ReadFile("dataset/splitCoords.geojson")
		if err != nil {
			return Output{}, fmt.Errorf("error reading splitCoords.geojson: %w", err)
		}
	}

	//Unmarshall feature collection from geojson
	var err error // Declare err here so it's in the correct scope for json.Unmarshal
	err = json.Unmarshal(data, &fc)
	if err != nil {
		return Output{}, fmt.Errorf("error unmarshalling geojson data: %w", err)
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
	if err != nil {
		return Output{}, fmt.Errorf("error finding path with golangGeojsonDijkstra: %w", err)
	}

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

	return output, nil
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

// Old mcpHandler is removed.
