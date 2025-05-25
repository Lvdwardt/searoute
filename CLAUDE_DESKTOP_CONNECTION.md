# Connecting Claude Desktop to the SeaRoute MCP Server

This document outlines how to connect Claude Desktop to the SeaRoute MCP server.

## Server Implementation Details

The SeaRoute MCP server is implemented in Go using the `github.com/mark3labs/mcp-go` library. It utilizes the `SSEServer` transport mechanism, which is based on Server-Sent Events (SSE).

The server exposes two primary endpoints for MCP communication:

1.  **SSE Connection Endpoint (`GET /mcp`)**:
    *   **Method**: `GET`
    *   **URL**: `http://<your_server_address_and_port>/mcp`
    *   **Purpose**: Claude Desktop should first connect to this endpoint to establish an SSE connection. Upon successful connection, the server will include an `MCP-Session-ID` in the response headers. This session ID is crucial for subsequent requests.

2.  **Tool Call Endpoint (`POST /messages`)**:
    *   **Method**: `POST`
    *   **URL**: `http://<your_server_address_and_port>/messages?sessionId=<session_id>`
    *   **Purpose**: After obtaining a session ID, Claude Desktop uses this endpoint to make tool calls (e.g., to the `calculateSeaRoute` tool). The `sessionId` obtained from the `/mcp` endpoint must be included as a URL query parameter.
    *   **Request Body**: The body of the POST request should be a standard MCP JSON-RPC message, for example:
        ```json
        {
          "jsonrpc": "2.0",
          "method": "callTool",
          "params": {
            "name": "calculateSeaRoute",
            "arguments": {
              "origin_port_name": "Shanghai",
              "destination_port_name": "New York"
            }
          },
          "id": "1"
        }
        ```

## Authentication

The SeaRoute MCP server **does not currently implement any authentication mechanisms**. The endpoints are open and do not require OAuth 2.1 or any other form of authentication token.

## Configuration in Claude Desktop

When configuring Claude Desktop to connect to this MCP server, you would typically need to provide the server's base URL. Based on the server's endpoint structure:

*   Claude Desktop will likely initiate the connection by making a `GET` request to `/mcp` at the provided base URL.
*   It should then use the returned `MCP-Session-ID` for `POST` requests to the `/messages` endpoint at the same base URL.

**Example Base URL**: If your server is running at `http://localhost:8080`, this would be the base URL to configure in Claude Desktop.

**Note on Local Configuration (if applicable, based on the blog post for local/bridged servers):**

If Claude Desktop requires a local configuration file (e.g., `claude_desktop_config.json`) for servers that don't support OAuth or are not discoverable through a central directory, the configuration would need to instruct Claude how to run or connect to this server.

However, the blog post implies that remote servers are generally connected to via their HTTPS URL and might involve OAuth. Since this server is remote but without OAuth, the exact connection method from Claude Desktop would depend on its capabilities for connecting to unauthenticated remote MCP servers. If Claude Desktop *strictly* requires OAuth for all remote servers, it might not be able to connect to this server without modifications to either the server (to add OAuth) or potentially Claude Desktop's connection mechanisms (if it allows unauthenticated remote connections).

Assuming Claude Desktop can connect to a remote, unauthenticated MCP server using the SSE flow:

1.  Provide the server's base URL (e.g., `http://<your_server_address_and_port>`).
2.  Claude should automatically handle the `GET /mcp` and subsequent `POST /messages?sessionId=` flow.

Due to limitations in the execution environment, the server functionality with `SSEServer` could not be fully tested. The above instructions are based on the intended implementation and the `mcp-go` library's behavior for SSE-based MCP servers.
