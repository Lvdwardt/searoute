package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	gdj "github.com/pitchinnate/golangGeojsonDijkstra"
)

const (
	// MCPPort is the port on which the MCP server will run
	MCPPort = "8081"
	// MCPEndpointPath is the path for the MCP endpoint
	MCPEndpointPath = "/mcp"
)

// StartMCPServer Runs MCP HTTP Stream server on port 8081
func StartMCPServer() {
	// Initialize the MCP server with a name and version
	mcpServer := server.NewMCPServer(
		"SeaRoute",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Set HTTP Streaming server endpoint
	streamServer := server.NewStreamableHTTPServer(mcpServer, server.WithEndpointPath(MCPEndpointPath))

	// Define tools
	shortSeaRoute := mcp.NewTool("calculate_short_sea_route",
		mcp.WithDescription("Calculate the shortest sea route between two ports available in the system"),
		mcp.WithTitleAnnotation("Shortest Sea Route Calculation"),
		mcp.WithString("origin_port_name",
			mcp.Required(),
			mcp.Description("Origin port name"),
		),
		mcp.WithString("destination_port_name",
			mcp.Required(),
			mcp.Description("Destination port name"),
		))

	getPortNames := mcp.NewTool("get_port_names",
		mcp.WithDescription("Get the names of all ports available in the system starting with the substring of the port or country name"),
		mcp.WithTitleAnnotation("Port Names Search"),
		mcp.WithString("search",
			mcp.Required(),
			mcp.Description("Search query to filter port names by the substring of the port or country name"),
		))

	// Register the tool with the server
	mcpServer.AddTool(shortSeaRoute, HandleShortSeaRoute)
	mcpServer.AddTool(getPortNames, HandleGetPortNames)

	// Start the MCP server
	// Print MCP endpoint information
	log.Printf("MCP Server is starting...")
	log.Printf("MCP endpoint URL: http://localhost:%s%s", MCPPort, MCPEndpointPath)
	log.Printf("No authentication required for this server")

	// Start the streamable HTTP server
	if err := streamServer.Start(":" + MCPPort); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
	log.Printf("MCP Server started successfully on port %s", MCPPort)
}

// HandleShortSeaRoute is the handler for the shortSeaRoute tool of mcp server
func HandleShortSeaRoute(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fromPort, err := request.RequireString("origin_port_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	toPort, err := request.RequireString("destination_port_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get the origin coordinates
	originLong, originLat := getPortCoordinates(fromPort)
	// Get the destination coordinates
	destinationLong, destinationLat := getPortCoordinates(toPort)

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

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// HandleGetPortNames is the handler for the getPortNames tool of mcp server
func HandleGetPortNames(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	searchTerm, err := request.RequireString("search")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get the list of port names matching the search query
	portNames := filterPorts(searchTerm)

	jsonBytes, err := json.MarshalIndent(portNames, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}
