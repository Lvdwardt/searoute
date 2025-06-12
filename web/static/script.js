// Simple sea route calculator - Multi-route only

// Spinner configuration
const spinOps = {
    lines: 13,
    length: 38,
    width: 17,
    radius: 45,
    scale: 1,
    corners: 1,
    speed: 1,
    rotate: 0,
    animation: 'spinner-line-fade-quick',
    direction: 1,
    color: '#ffffff',
    fadeColor: 'transparent',
    top: '50%',
    left: '50%',
    shadow: '0 0 1px transparent',
    zIndex: 2000000000,
    className: 'spinner',
    position: 'absolute',
}

const spinElement = document.getElementById('map');
const spinner = new Spinner(spinOps).spin(spinElement);
spinner.stop();

// Initialize map
const map = L.map("map", {
    preferCanvas: true,
}).setView([0, 0], 2);

// Add base map tiles
L.tileLayer('https://{s}.basemaps.cartocdn.com/rastertiles/voyager_labels_under/{z}/{x}/{y}{r}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
    subdomains: 'abcd',
    maxZoom: 20
}).addTo(map);

// Add sea chart overlay
L.tileLayer('https://tiles.openseamap.org/seamark/{z}/{x}/{y}.png', {
    attribution: 'Map data: &copy; <a href="http://www.openseamap.org">OpenSeaMap</a> contributors'
}).addTo(map);

// Global variables for route management
let currentRouteLayer = null;
let currentMarkersLayer = null;

// Function to clear existing routes and markers
function clearMap() {
    if (currentRouteLayer) {
        map.removeLayer(currentRouteLayer);
        currentRouteLayer = null;
    }
    if (currentMarkersLayer) {
        map.removeLayer(currentMarkersLayer);
        currentMarkersLayer = null;
    }
    
    // Hide result info
    document.getElementById('result-info').style.display = 'none';
    
    // Clear stored route data
    window.currentRouteGeoJSON = null;
}

// Function to display multi-route on map
function displayMultiRoute(routeData) {
    clearMap();
    
    if (!routeData || !routeData.geometry || !routeData.geometry.coordinates) {
        console.error('Invalid route data received');
        return;
    }

    // Handle both LineString and MultiLineString geometries
    let allRouteCoords = [];
    let routeSegments = [];
    
    if (routeData.geometry.type === 'LineString') {
        // Single route segment
        const coords = routeData.geometry.coordinates.map(coord => [coord[1], coord[0]]); // Convert [lng, lat] to [lat, lng]
        allRouteCoords = coords;
        routeSegments = [coords];
    } else if (routeData.geometry.type === 'MultiLineString') {
        // Multiple route segments (e.g., when crossing dateline)
        routeSegments = routeData.geometry.coordinates.map(segment => 
            segment.map(coord => [coord[1], coord[0]]) // Convert [lng, lat] to [lat, lng]
        );
        // Flatten all coordinates for marker placement
        allRouteCoords = routeSegments.flat();
    } else {
        console.error('Unsupported geometry type:', routeData.geometry.type);
        return;
    }

    // Create route lines (one or multiple segments)
    currentRouteLayer = L.layerGroup();
    routeSegments.forEach(segmentCoords => {
        if (segmentCoords.length > 0) {
            const polyline = L.polyline(segmentCoords, {
                color: '#2196F3',
                weight: 4,
                opacity: 0.8
            });
            currentRouteLayer.addLayer(polyline);
        }
    });
    currentRouteLayer.addTo(map);

    // Create markers for start and end points
    currentMarkersLayer = L.layerGroup();
    
    if (allRouteCoords.length > 0) {
        // Start marker (green) - first coordinate
        L.marker(allRouteCoords[0], {
            icon: L.icon({
                iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-green.png',
                shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/0.7.7/images/marker-shadow.png',
                iconSize: [25, 41],
                iconAnchor: [12, 41],
                popupAnchor: [1, -34],
                shadowSize: [41, 41]
            })
        }).bindPopup('Start Point').addTo(currentMarkersLayer);

        // End marker (red) - last coordinate
        const endIndex = allRouteCoords.length - 1;
        L.marker(allRouteCoords[endIndex], {
            icon: L.icon({
                iconUrl: 'https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-red.png',
                shadowUrl: 'https://cdnjs.cloudflare.com/ajax/libs/leaflet/0.7.7/images/marker-shadow.png',
                iconSize: [25, 41],
                iconAnchor: [12, 41],
                popupAnchor: [1, -34],
                shadowSize: [41, 41]
            })
        }).bindPopup('End Point').addTo(currentMarkersLayer);
    }
    
    currentMarkersLayer.addTo(map);

    // Fit map to route bounds
    try {
        const bounds = L.latLngBounds();
        currentRouteLayer.eachLayer(layer => {
            if (layer.getBounds && layer.getBounds().isValid()) {
                bounds.extend(layer.getBounds());
            }
        });
        if (bounds.isValid()) {
            map.fitBounds(bounds, { padding: [20, 20] });
        }
    } catch (e) {
        console.warn('Could not fit map to route bounds:', e);
        // Fallback: fit to all route coordinates
        if (allRouteCoords.length > 0) {
            const bounds = L.latLngBounds(allRouteCoords);
            map.fitBounds(bounds, { padding: [20, 20] });
        }
    }
    
    // Store route data
    window.currentRouteGeoJSON = routeData;

    // Show route information
    const totalDistance = routeData.properties.total_distance;
    const routeCount = routeData.properties.route_count;
    
    document.getElementById('total-distance').textContent = totalDistance.toFixed(2);
    document.getElementById('route-count').textContent = routeCount;
    document.getElementById('result-info').style.display = 'block';
}

// Multi-route calculation
document.getElementById('calculate-multi-route').addEventListener('click', function() {
    const coordinatesInput = document.getElementById('coordinates-input').value.trim();
    
    if (!coordinatesInput) {
        alert('Please enter coordinates in JSON format');
        return;
    }

    let coordinates;
    try {
        coordinates = JSON.parse(coordinatesInput);
    } catch (e) {
        alert('Invalid JSON format. Please check your input.');
        return;
    }

    if (!Array.isArray(coordinates) || coordinates.length < 2) {
        alert('Please provide at least 2 coordinates');
        return;
    }

    // Validate coordinate format
    for (let i = 0; i < coordinates.length; i++) {
        const coord = coordinates[i];
        if (!coord.hasOwnProperty('lon') || !coord.hasOwnProperty('lat') ||
            typeof coord.lon !== 'number' || typeof coord.lat !== 'number') {
            alert(`Invalid coordinate format at position ${i + 1}. Use: {"lon": number, "lat": number}`);
            return;
        }
        
        // Validate coordinate ranges
        if (coord.lat < -90 || coord.lat > 90 || coord.lon < -180 || coord.lon > 180) {
            alert(`Invalid coordinate values at position ${i + 1}. Latitude must be between -90 and 90, longitude between -180 and 180.`);
            return;
        }
    }

    console.log('Calculating route for coordinates:', coordinates);

    // Show spinner
    spinner.spin(spinElement);

    fetch('/multi-routes', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({ coordinates: coordinates })
    })
    .then(response => response.json())
    .then(data => {
        spinner.stop();
        if (data.error) {
            alert('Error: ' + data.error);
        } else {
            console.log('Route calculated successfully:', data);
            displayMultiRoute(data);
        }
    })
    .catch(error => {
        spinner.stop();
        console.error('Error:', error);
        alert('Failed to calculate route. Please try again.');
    });
});

// Clear button functionality
document.getElementById('clear-button').addEventListener('click', function() {
    clearMap();
    
    // Clear form input
    document.getElementById('coordinates-input').value = '';
    
    // Reset map view
    map.setView([0, 0], 2);
});

// Download button functionality (if needed later)
if (document.getElementById('download-button')) {
    document.getElementById('download-button').addEventListener('click', function() {
        if (!window.currentRouteGeoJSON) {
            alert('No route data available for download');
            return;
        }
        
        const dataStr = JSON.stringify(window.currentRouteGeoJSON, null, 2);
        const dataBlob = new Blob([dataStr], {type: 'application/json'});
        
        const url = URL.createObjectURL(dataBlob);
        const link = document.createElement('a');
        link.href = url;
        link.download = 'sea-route.geojson';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    });
}
