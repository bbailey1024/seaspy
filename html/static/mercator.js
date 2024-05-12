// Web Mercator Functions
// Reference: https://developers.google.com/maps/documentation/javascript/examples/map-coordinates
// Reference: https://stackoverflow.com/questions/23457916/how-to-get-latitude-and-longitude-bounds-from-google-maps-x-y-and-zoom-parameter

// inBoundingBox returns a boolean depending on whether the WGS 84 latitude and longitude fall within the bounding box.
export function inBoundingBox(bounds, pos) {
    let isLngInRange = pos.lng >= bounds.sw.lng && pos.lng < bounds.ne.lng;
    let isLatInRange = pos.lat >= bounds.sw.lat && pos.lat < bounds.ne.lat;
    return ( isLngInRange && isLatInRange );
}

// normalizeTile is used by getTileBounds to process or validate tile coordinates before bounds calculations.
// In practice, I've not observed a requirement for this but the author in the SO post and Google's documention believe there is a requirement.
// See getNormalizedCoords() - https://developers.google.com/maps/documentation/javascript/maptypes#ImageMapTypes
function normalizeTile(tile, zoom) {
    let t = Math.pow(2, zoom);
    tile.x = ((tile.x % t) + t) % t;
    tile.y = ((tile.y % t) + t) % t;
    return tile;
}

// getTileBounds takes tile coordinates, tile size, and zoom level and returns a WGS48 latitude and longitude bounding box for the southwest and northeast corners of the tile.
export function getTileBounds(tile, size, zoom) {
    tile = normalizeTile(tile, zoom);
    
    let t = Math.pow(2, zoom);
    let s = 256 / t;
    let sw = {
        x: tile.x * s,
        y: (tile.y * s) + s,
    }
    let ne = {
        x: tile.x * s + s,
        y: (tile.y * s),
    };

    return {
        sw: fromPointToLatLng(sw, size),
        ne: fromPointToLatLng(ne, size)
    }
}

// fromLatLngToTilePixel is a convenience function to convert WGS84 latitude and longitude to pixel coordinates relative to tiles based on tile size.
export function fromLatLngToTilePixel(latlng, size, zoom) {
    let point = fromLatLngToPoint(latlng, size);
    let pixel = fromPointToPixel(point, zoom);
    return fromPixelToTilePixel(pixel, size);
}

// fromPixelToTilePixel converts screen pixel coordinates to pixel coordinates relative to tiles based on tile size.
export function fromPixelToTilePixel(pixel, tileSize) {
    return {
        x: pixel.x % tileSize,
        y: pixel.y % tileSize,
    }
}

// fromPointToPixel converts world coordinate points to screen pixel coordinates.
export function fromPointToPixel(point, zoom) {
    let scale = Math.pow(2, zoom);
    return {
        x: Math.floor(point.x * scale),
        y: Math.floor(point.y * scale),
    }
}

// fromLatLngToPoint converts WGS84 latitude and longitude values to world coordinates on the map projection.
// Google refers to these points as world coordinates.
export function fromLatLngToPoint(latlng, size) {
    let siny = Math.sin((latlng.lat * Math.PI) / 180);
    siny = Math.min(Math.max(siny, -0.9999), 0.9999);
    return {
        x: size * (0.5 + latlng.lng / 360),
        y: size * (0.5 - Math.log((1 + siny) / (1 - siny)) / (4 * Math.PI)),
    }
}

// fromPointToLatLng converts world coordinate points to WGS84 latitude and longitude.
export function fromPointToLatLng(point, size) {
    return {
        lat: (2 * Math.atan(Math.exp((point.y - (size/2)) / -(size / (2 * Math.PI)))) - Math.PI / 2)/ (Math.PI / 180),
        lng: (point.x - (size/2)) / (size / 360)
    };
}

// fromLatLngToTileCoord returns tile coordinates based on latlng, tile size, and zoome level.
export function fromLatLngToTileCoord(latlng, size, zoom) {
    let scale = Math.pow(2, zoom);
    let point = fromLatLngToPoint(latlng, size);
    return {
        x: Math.floor((point.x * scale) / size),
        y: Math.floor((point.y * scale) / size),
    }
}