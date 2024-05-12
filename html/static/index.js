import { polyMap, colorMap, drawShape, clipPositions } from './draw.js';
import { getTileBounds, inBoundingBox, fromLatLngToTilePixel } from './mercator.js';
await google.maps.importLibrary("maps");

const tileSize = 256;
const axiosInstance = axios.create({
    baseURL: window.location.origin,
    timeout: 1000,
});

class SeaSpyOverlay extends google.maps.OverlayView {
    bounds;
    canvas;
    constructor(bounds, canvas) {
      super();
      this.bounds = bounds;
      this.canvas = canvas;
    }

    onAdd() {
        this.div = document.createElement("div");
        this.div.id = "overlayMouseTarget";
        this.div.style.borderStyle = "none";
        this.div.style.borderWidth = "0px";
        this.div.style.position = "absolute";
        this.div.appendChild(this.canvas);

        const panes = this.getPanes();
        panes.overlayMouseTarget.appendChild(this.div);
    }

    draw() {
        const overlayProjection = this.getProjection();
        const sw = overlayProjection.fromLatLngToDivPixel(this.bounds.sw);
        const ne = overlayProjection.fromLatLngToDivPixel(this.bounds.ne);

        if (this.div) {
            this.div.style.left = sw.x + "px";
            this.div.style.top = ne.y + "px";
            this.div.style.width = ne.x - sw.x + "px";
            this.div.style.height = sw.y - ne.y + "px";
        }
    }

    onRemove() {
        if (this.div) {
            this.div.parentNode.removeChild(this.div);
            delete this.div;
        }
    }
}

class SeaSpyMapType {
    constructor(gmap, state, ships) {
        this.tileSize = new google.maps.Size(tileSize, tileSize);
        this.gmap = gmap;
        this.state = state;
        this.ships = ships;
    }

    getTile(coord, zoom) {
        let tileId = `${coord.x},${coord.y}`;

        const canvas = document.createElement('canvas');
        canvas.id = `canvas_${tileId}`;
        canvas.width = tileSize;
        canvas.height = tileSize;
        canvas.addEventListener("click", async (e) => {
            let shapes = this.state.shapes.get(tileId);
            let ctx = canvas.getContext("2d");
            canvasClick(shapes, this.ships, ctx, this.gmap, e);
        });

        const bounds = getTileBounds(coord, tileSize, zoom);

        var canvasOverlay = new SeaSpyOverlay(bounds, canvas);
        canvasOverlay.setMap(this.gmap);

        this.state.shapes.set(tileId, []);

        this.state.tileIdToDetails.set(tileId, { bounds: bounds, coord: coord, canvas: canvas, zoom: zoom, size: tileSize });
        this.state.overlays.set(canvas, canvasOverlay);

        return canvas;
    }
}

async function seaspy() {
    var gmap = new google.maps.Map(document.getElementById('map'), {
        zoom: 3,
        minZoom: 3,
        center: {lat: 20, lng: 0},
        draggableCursor: "default",
        draggingCursor: "default",
        clickableIcons: false,
        disableDefaultUI: true,
        fullscreenControl: true,
    });

    const shipTypes = await getShipTypes();
    const shipMap = await getShips();
    const infoWindow = new google.maps.InfoWindow;
    const polyline = new google.maps.Polyline;

    var ships = {
        types: shipTypes.types,
        groups: shipTypes.groups,
        map: shipMap,
        infowindow: infoWindow,
        route: polyline,
    };

    var state = {
        shapes: new Map(),
        overlays: new Map(),
        tileIdToDetails: new Map(),
        clipBuffer: new Map(),
    };

    const seaspyMap = new SeaSpyMapType(gmap, state, ships);
    gmap.overlayMapTypes.insertAt(0, seaspyMap);

    const search = document.getElementById("search");
    search.addEventListener("input", debounce((e) => {
        searchHandler(e.target.value, gmap, ships)
    }, 500));
    
    google.maps.event.addListener(gmap, 'zoom_changed', function() {
        state.clipBuffer = new Map();

        for (let [k, overlay] of state.overlays) {
            overlay.setMap(null);
            state.overlays.delete(k);
        }
    });

    google.maps.event.addListener(gmap, 'tilesloaded', function() {

        for (let mmsi in ships.map) {
            let ship = ships.map[mmsi];
            let latlng = {lat: ship.latlon[0], lng: ship.latlon[1]};

            // AIS without names clutter the map, consider adding frontend filter option
            if (!ship.name) {
                continue;
            }

            // AIS sending undefined latlng, consider moving to backend to filter
            if (latlng.lat.toFixed(4) == 0.0000 && latlng.lng.toFixed(4) == 0.0000) {
                continue;
            }

            for (let [tileId, tileDetails] of state.tileIdToDetails) {
                if (!inBoundingBox(tileDetails.bounds, latlng)) {
                    continue;
                }

                addShipMarker(state, ships, mmsi, ship, tileId);
            }
        }

        for (let [tileId] of state.tileIdToDetails) {
            drawClipBuffer(state, tileId);
        }

        // Empty the tile map to eliminate overlapping/duplicate bounding boxes as new tiles load.
        state.tileIdToDetails = new Map();
    });
}

function searchHandler(q, gmap, ships) {
    let searchResults = document.getElementById("search-results");
    searchResults.innerHTML = '';
    searchResults.style.display = 'none';

    if (!q || q.length < 3) {
        return;
    }

    let search = q.trim().toLowerCase();

    let resultCounter = 0;
    let resultLimit = 20;

    for (let mmsi in ships.map) {
        let ship = ships.map[mmsi];

        let mmsiMatch = mmsi === search;
        let shipMatch = false;
        if (ship.name) {
            shipMatch = ship.name.trim().toLowerCase().includes(search);
        }

        if (shipMatch || mmsiMatch) {
            if (resultCounter === resultLimit) {
                break;
            }
            resultCounter++;

            const resultDiv = document.createElement('div');
            resultDiv.classList.add('search-result');

            const nameElement = document.createElement('span');
            nameElement.classList.add('search-result-name');
            nameElement.innerHTML = ship.name;
            resultDiv.appendChild(nameElement);

            const mmsiElement = document.createElement('span');
            mmsiElement.classList.add('search-result-mmsi');
            mmsiElement.innerHTML = mmsi;
            resultDiv.appendChild(mmsiElement);

            resultDiv.addEventListener("click", async (e) => {
                gmap.setCenter({lat: ship.latlon[0], lng: ship.latlon[1]});
                gmap.setZoom(15);
                openInfoWindow(gmap, ships, mmsi);
                openShipHistory(gmap, ships, mmsi);
            });

            searchResults.appendChild(resultDiv);
        }
    }

    if (resultCounter > 0) {
        searchResults.style.display = 'block';
    }
}

async function openInfoWindow(gmap, ships, mmsi) {
    const ship = ships.map[mmsi];
    const shipInfo = await getShipInfo(mmsi);
    const category = getShipGroup(ships, ship.shipType).category;
    const content = formatContent(mmsi, ship, shipInfo, category);

    ships.infowindow.setOptions({
        content: content,
        position: { lat: ship.latlon[0], lng: ship.latlon[1] },
        pixelOffset: new google.maps.Size(0, -5),
    });
    ships.infowindow.open(gmap);
}

async function openShipHistory(gmap, ships, mmsi) {
    const shipHist = await getShipHistory(mmsi);
    const historyIcon = {
        path: "M 0,-1 0,1",
        strokeColor: "#03ad25",
        strokeOpacity: 1,
        scale: 1,
    };

    if (ships.route) ships.route.setMap(null);
    ships.route = new google.maps.Polyline({
        path: shipHist,
        strokeOpacity: 0,
        strokeWeight: 0,
        icons: [
            {
                icon: historyIcon,
                "offset": 0,
                "repeat": "5px",
            },
        ],
    });

    ships.route.setMap(gmap);
}

async function addShipMarker(state, ships, mmsi, ship, tileId) {
    let shipGroup = getShipGroup(ships, ship.shipType);
    let latlng = {lat: ship.latlon[0], lng: ship.latlon[1]};

    let tile = state.tileIdToDetails.get(tileId);
    let centerPoint = fromLatLngToTilePixel(latlng, tile.size, tile.zoom);

    // Fade boolean to pass to colorMap if ship has not been observed for more than 1 day
    let fade = Math.floor(Date.now() / 1000) - ship.lastUpdate > 86400;

    // Get functions from draw.js based on ship's marker type and color.
    let polys = polyMap(ship.marker);
    let draw = drawShape(ship.marker);
    let colors = colorMap(shipGroup['color'], fade);

    for (let [drawType, polyShape] of polys) {
        let ctx = tile.canvas.getContext("2d");
        let shape = polyShape(centerPoint, ship.rotation);
        var color = colors.get(drawType);

        let path = draw(ctx, shape, color);
        state.shapes.get(tileId).push({path: path, color: color, mmsi: mmsi});

        var clipping = clipPositions(centerPoint, shape, tileSize);
        for (let i in clipping) {
            let clippedTileCoord = {x: tile.coord.x + clipping[i].x, y: tile.coord.y + clipping[i].y};
            let clippedTileId = `${clippedTileCoord.x},${clippedTileCoord.y}`;
            let clippedShape = polyShape(clipping[i].center, ship.rotation);

            // The tile where a shape clips may not have been loaded yet.
            // In this case, store the clipped shape in the clip buffer instead of drawing it.
            if (state.tileIdToDetails.has(clippedTileId)) {
                let clippedTile = state.tileIdToDetails.get(clippedTileId); 
                let clippedCtx = clippedTile.canvas.getContext("2d");
                let path = draw(clippedCtx, clippedShape, color);
                state.shapes.get(clippedTileId).push({path: path, color: color, mmsi: mmsi});
            } else {
                if (!state.clipBuffer.has(clippedTileId)) {
                    state.clipBuffer.set(clippedTileId, []);
                }
                state.clipBuffer.get(clippedTileId).push({drawFunc: draw, shape: clippedShape, color: color, mmsi: mmsi});
            }
        }
    }
}

async function drawClipBuffer(state, tileId) {
    if (state.clipBuffer.has(tileId)) {
        let tile = state.tileIdToDetails.get(tileId);
        let ctx = tile.canvas.getContext("2d");
        let clips = state.clipBuffer.get(tileId);
        for (let i in clips) {
            let path = clips[i].drawFunc(ctx, clips[i].shape, clips[i].color);
            state.shapes.get(tileId).push({path: path, color: clips[i].color, mmsi: clips[i].mmsi});
        }
        state.clipBuffer.delete(tileId);
    }    
}

async function canvasClick(shapes, ships, ctx, gmap, e) {
    var mmsi;
    for (let i = 0; i < shapes.length; i++) {
        if (ctx.isPointInPath(shapes[i].path, e.offsetX, e.offsetY)) {
            mmsi = shapes[i].mmsi;
            break;
        }
    }

    if (mmsi) {
        openInfoWindow(gmap, ships, mmsi);
        openShipHistory(gmap, ships, mmsi);
    } else {
        if (ships.route) ships.route.setMap(null);
        if (ships.infowindow) ships.infowindow.close();
    }
}

function getShipGroup(ships, type) {
    let shipGroup = {};
    if (ships.types.hasOwnProperty(type)) {
        const shipType = ships.types[type];
        shipGroup = ships.groups[shipType.groupId];
    } else {
        shipGroup['category'] = "Other";
        shipGroup['color'] = "#949494";
        console.warn("unknown ship type: " + type);
    }
    return shipGroup;
}

function formatContent(mmsi, ship, shipInfo, category) {
    let name = ship.name;
    if (shipInfo.imoNumber) {
        name = `<a href="https://www.shipspotting.com/photos/gallery?imo=${shipInfo.imoNumber}" target="_blank" rel="noopener noreferrer">${ship.name}</a>`;
    }

    const content = 
    `<div id="infoWindow">` +
    `<p><b>${name}</b></p>` +
    `<p>MMSI: ${mmsi}\n` + 
    `Position: ${ship.latlon[0].toFixed(4)}, ${ship.latlon[1].toFixed(4)}\n` +
    `Heading: ${ship.heading}\n` +
    `Speed (kt): ${ship.sog}\n` +
    `Dest: ${shipInfo.destination}\n` +
    `ShipType: ${category} (${ship.shipType})\n` +
    `NavStat: ${ship.navStat}\n` +
    `Last Seen: ${friendlyTime(ship.lastUpdate)}` +
    `</div>`;

    return content;
}

function friendlyTime(lastUpdate) {
    let time = Math.floor(Date.now() / 1000) - lastUpdate;

    let days = Math.floor(time / 86400);
    let hours = Math.floor((time % 86400) / 3600);
    let minutes = Math.floor(((time % 86400) % 3600) / 60);
    let seconds = Math.floor(((time % 86400) % 3600) % 60);

    let s = "";
    if (days > 0) {
        s += `${days}d `;
    }

    if (hours > 0) {
        s += `${hours}h `;
    }

    if (minutes > 0) {
        s += `${minutes}m `;
    }

    s += `${seconds}s ago`;

    return s;
}

async function getShips() {
    const { data } = await axiosInstance.get("/ships");
    return data;
}

async function getShipInfo(mmsi) {
    const { data } = await axiosInstance.get('/shipInfo/' + mmsi);
    return data;
}

async function getShipTypes() {
    let [shipTypesRsp, shipGroupsRsp] = await Promise.all([
        axiosInstance.get('/shipTypes'),
        axiosInstance.get('/shipGroups'),
    ]);

    return { types: shipTypesRsp.data, groups: shipGroupsRsp.data };
}

async function getShipHistory(mmsi) {
    const { data } = await axiosInstance.get('/shipHistory/' + mmsi);

    let hist = [];
    for (let n of data) {
        hist.push({
            "lat": n.latlon[0],
            "lng": n.latlon[1]
        });
    }
    return hist;
}

// debounce function from https://www.joshwcomeau.com/snippets/javascript/debounce/
function debounce(callback, wait) {
    let timeoutId = null;
    return (...args) => {
        window.clearTimeout(timeoutId);
        timeoutId = window.setTimeout(() => {
            callback.apply(null, args);
        }, wait);
    };
}

seaspy();