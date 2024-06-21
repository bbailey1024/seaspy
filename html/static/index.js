import { polyMap, colorMap, drawShape, clipPositions } from './draw.js';
import { getTileBounds, fromLatLngToTilePixel } from './mercator.js';
await google.maps.importLibrary("maps");

const tileSize = 256;
const axiosInstance = axios.create({
    baseURL: window.location.origin,
    timeout: 1000,
});

class SeaSpyOverlay extends google.maps.OverlayView {
    constructor(bounds, canvas) {
      super();
      this.bounds = bounds;
      this.canvas = canvas;
    }

    onAdd() {
        this.div = document.createElement("div");
        this.div.style.borderStyle = "none";
        this.div.style.borderWidth = "0px";
        this.div.style.position = "absolute";
        this.div.classList.add("overlayMouseTarget");
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
    constructor(gmap, state, shipmeta) {
        this.tileSize = new google.maps.Size(tileSize, tileSize);
        this.gmap = gmap;
        this.state = state;
        this.shipmeta = shipmeta;
    }

    getTile(coord, zoom) {
        let tileId = `${coord.x},${coord.y}`;

        const canvas = document.createElement('canvas');
        canvas.id = `${tileId}`;
        canvas.width = tileSize;
        canvas.height = tileSize;
        canvas.addEventListener("click", async (e) => {
            let shapes = this.state.shapes.get(tileId);
            let ctx = canvas.getContext("2d");
            canvasClick(shapes, this.shipmeta, ctx, this.gmap, e);
        });

        const bounds = getTileBounds(coord, tileSize, zoom);

        var canvasOverlay = new SeaSpyOverlay(bounds, canvas);
        canvasOverlay.setMap(this.gmap);

        this.state.shapes.set(tileId, []);

        this.state.tileIdToDetails.set(tileId, { bounds: bounds, coord: coord, canvas: canvas, zoom: zoom, size: tileSize });
        this.state.overlays.set(tileId, canvasOverlay);

        this.state.active.set(tileId, 0);

        return canvas;
    }

    releaseTile(tile) {
        // There is an issue where tiles are released immediately after loading.
        // This is inconsistent, but likely occurs due to marker draw time.
        // To address this, tiles are only released if they are active.
        // This value is set to 0 during getTile and 1 in the drawTile function.
        // This issue also requires a manual cleanup of orphaned overlayMouseTarget divs.
        // This is implemented in the zoom_changed event by destroying all overlayMouseTarget divs.
        if (this.state.active.get(tile.id) === 1) {
            this.state.shapes.delete(tile.id);
            this.state.tileIdToDetails.delete(tile.id);
            this.state.clipBuffer.delete(tile.id);

            if (this.state.overlays.has(tile.id)) {
                this.state.overlays.get(tile.id).setMap(null);
                this.state.overlays.delete(tile.id);
            }
        }
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
    const infoWindow = new google.maps.InfoWindow;
    const polyline = new google.maps.Polyline;

    var shipmeta = {
        types: shipTypes.types,
        groups: shipTypes.groups,
        infowindow: infoWindow,
        route: polyline,
        search: {},
    };

    var state = {
        shapes: new Map(),
        overlays: new Map(),
        tileIdToDetails: new Map(),
        clipBuffer: new Map(),
        active: new Map(),
    };

    const seaspyMap = new SeaSpyMapType(gmap, state, shipmeta);
    gmap.overlayMapTypes.insertAt(0, seaspyMap);

    const search = document.getElementById("search");
    search.addEventListener("input", debounce((e) => {
        searchHandler(e.target.value, gmap, shipmeta)
    }, 500));

    setInterval(async function(){
        drawTiles(state, shipmeta);
        shipmeta.search = await getSearchCache();
    }, 10000);

    google.maps.event.addListener(gmap, 'zoom_changed', function() {
        stateCleanup(state);
    });

    google.maps.event.addListener(gmap, 'tilesloaded', async function() {
        drawTiles(state, shipmeta);
        shipmeta.search = await getSearchCache();
    });
}

// stateCleanup performs cleanup of state on zoom change as all prior tiles have been destroyed.
// See note in releaseTile for why manual cleanup of overlayMouseTarget divs is required.
function stateCleanup(state) {
    for (let [tileId, overlay] of state.overlays) {
        overlay.setMap(null);
        state.overlays.delete(tileId);
    }

    let overlayDivs = document.getElementsByClassName("overlayMouseTarget");
    while (overlayDivs.length > 0) {
        overlayDivs[0].parentNode.removeChild(overlayDivs[0]);
    }

    state.shapes = new Map();
    state.tileIdToDetails = new Map();
    state.clipBuffer = new Map();
    state.active = new Map();
}

// drawTiles draws ship markers based on latlng bounding box of the tile.
// Tiles are drawn in reverse then forward order to ensure ship clips are drawn reliably.
async function drawTiles(state, shipmeta) {
    const tileData = new Map();
    await Promise.all(
        state.tileIdToDetails.entries().map(async ([tileId, { bounds }]) => {
            const data = await getShipsBbox(bounds);
            tileData.set(tileId, data);
        })
    );

    for (let [tileId, tileDetails] of state.tileIdToDetails) {
        state.active.set(tileId) == 1;

        if (!tileData.has(tileId)) {
            continue;
        }

        let tileShips = tileData.get(tileId);

        let ctx = tileDetails.canvas.getContext("2d");
        ctx.clearRect(0, 0, tileDetails.canvas.width, tileDetails.canvas.height);
        state.shapes.set(tileId, []);

        for (let ship of tileShips) {
            let shipGroup = getShipGroup(shipmeta, ship.shipType);
            addShipMarker(state, shipGroup, ship, tileId);
        }
        drawClipBuffer(state, tileId);
    }

    for (let [tileId] of [...state.tileIdToDetails].reverse()) {
        if (!tileData.has(tileId)) {
            return;
        }

        let tileShips = tileData.get(tileId);

        for (let ship of tileShips) {
            let shipGroup = getShipGroup(shipmeta, ship.shipType);
            addShipMarker(state, shipGroup, ship, tileId);
        }
        drawClipBuffer(state, tileId); 
    }
}

function searchHandler(q, gmap, shipmeta) {
    if (!q || q.length < 3) {
        return;
    }

    let search = q.trim().toLowerCase();

    let resultLimit = 20;
    let results = [];

    for (let ship of shipmeta.search) {
        let mmsiMatch = ship.mmsi === search;
        let shipMatch = false;
        if (ship.name) {
            shipMatch = ship.name.trim().toLowerCase().includes(search);
        }

        if (shipMatch || mmsiMatch) {
            if (results.length >= resultLimit) {
                break;
            }
            results.push(ship);
        }
    }

    results.sort((a, b) => {
        if (a.name < b.name) return -1;
        if (a.name > b.name) return 1;
        return 0;
    });

    let searchResults = document.getElementById("search-results");
    searchResults.innerHTML = '';
    searchResults.style.display = 'none';

    for (let r of results) {
        const resultDiv = document.createElement('div');
        resultDiv.classList.add('search-result');

        const nameElement = document.createElement('span');
        nameElement.classList.add('search-result-name');
        nameElement.innerHTML = r.name;
        resultDiv.appendChild(nameElement);

        const mmsiElement = document.createElement('span');
        mmsiElement.classList.add('search-result-mmsi');
        mmsiElement.innerHTML = r.mmsi;
        resultDiv.appendChild(mmsiElement);

        resultDiv.addEventListener("click", async (e) => {
            gmap.setCenter({lat: r.latlon[0], lng: r.latlon[1]});
            gmap.setZoom(15);
            openInfoWindow(gmap, shipmeta, r.mmsi);
            openShipHistory(gmap, shipmeta, r.mmsi);
        });

        searchResults.appendChild(resultDiv);
    }

    if (results.length > 0) {
        searchResults.style.display = 'block';
    }
}

async function openInfoWindow(gmap, shipmeta, mmsi) {
    const shipInfo = await getShipInfoWindow(mmsi);
    shipInfo.category = getShipGroup(shipmeta, shipInfo.shipType).category;
    const content = formatContent(shipInfo);

    shipmeta.infowindow.setOptions({
        content: content,
        position: { lat: shipInfo.latlon[0], lng: shipInfo.latlon[1] },
        pixelOffset: new google.maps.Size(0, -5),
    });
    shipmeta.infowindow.open(gmap);
}

async function openShipHistory(gmap, shipmeta, mmsi) {
    const shipHist = await getShipHistory(mmsi);

    if (shipHist.length == 0) {
        return;
    }

    const historyIcon = {
        path: "M 0,-1 0,1",
        strokeColor: "#03ad25",
        strokeOpacity: 1,
        scale: 1,
    };

    if (shipmeta.route) shipmeta.route.setMap(null);
    shipmeta.route = new google.maps.Polyline({
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

    shipmeta.route.setMap(gmap);
}

async function addShipMarker(state, shipGroup, ship, tileId) {
    let latlng = {lat: ship.latlon[0], lng: ship.latlon[1]};

    if (!ship.name) {
        return;
    }

    if (latlng.lat.toFixed(4) == 0.0000 && latlng.lng.toFixed(4) == 0.0000) {
        return;
    }

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
        state.shapes.get(tileId).push({path: path, color: color, mmsi: ship.mmsi});

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
                state.shapes.get(clippedTileId).push({path: path, color: color, mmsi: ship.mmsi});
            } else {
                if (!state.clipBuffer.has(clippedTileId)) {
                    state.clipBuffer.set(clippedTileId, []);
                }
                state.clipBuffer.get(clippedTileId).push({drawFunc: draw, shape: clippedShape, color: color, mmsi: ship.mmsi});
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

async function canvasClick(shapes, shipmeta, ctx, gmap, e) {
    var mmsi;
    for (let i = shapes.length - 1; i >= 0; i--) {
        if (ctx.isPointInPath(shapes[i].path, e.offsetX, e.offsetY)) {
            mmsi = shapes[i].mmsi;
            break;
        }
    }

    if (mmsi) {
        openInfoWindow(gmap, shipmeta, mmsi);
        openShipHistory(gmap, shipmeta, mmsi);
    } else {
        if (shipmeta.route) shipmeta.route.setMap(null);
        if (shipmeta.infowindow) shipmeta.infowindow.close();
    }
}

function getShipGroup(shipmeta, type) {
    let shipGroup = {};
    if (shipmeta.types.hasOwnProperty(type)) {
        const shipType = shipmeta.types[type];
        shipGroup = shipmeta.groups[shipType.groupId];
    } else {
        shipGroup['category'] = "Other";
        shipGroup['color'] = "#949494";
        console.warn("unknown ship type: " + type);
    }
    return shipGroup;
}

function formatContent(shipInfo) {
    let name = shipInfo.name;
    if (shipInfo.imoNumber) {
        name = `<a href="https://www.shipspotting.com/photos/gallery?imo=${shipInfo.imoNumber}" target="_blank" rel="noopener noreferrer">${shipInfo.name}</a>`;
    }

    const content = 
    `<div id="infoWindow">` +
    `<p><b>${name}</b></p>` +
    `<p>MMSI: ${shipInfo.mmsi}\n` + 
    `Position: ${shipInfo.latlon[0].toFixed(4)}, ${shipInfo.latlon[1].toFixed(4)}\n` +
    `Heading: ${shipInfo.heading}\n` +
    `Speed (kt): ${shipInfo.sog}\n` +
    `Dest: ${shipInfo.destination}\n` +
    `ShipType: ${shipInfo.category} (${shipInfo.shipType})\n` +
    `NavStat: ${shipInfo.navStat}\n` +
    `Last Seen: ${friendlyTime(shipInfo.lastUpdate)}` +
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

// getShipsBbox returns an array of ships, sorted by geohash.
// Plotting shapes in this order will result shape overlap that is not aesthetically pleasing.
// Sorting by mmsi will plot shapes in a manner that lacks geospatial awareness.
async function getShipsBbox(bounds) {
    const uri = `/ships/${bounds.sw.lat},${bounds.sw.lng}/${bounds.ne.lat},${bounds.ne.lng}`
    const { data } = await axiosInstance.get(uri);
    data.sort((a,b) => a.mmsi - b.mmsi);
    return data;
}

async function getShipInfoWindow(mmsi) {
    const { data } = await axiosInstance.get('/shipInfoWindow/' + mmsi);
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

async function getSearchCache() {
    const { data } = await axiosInstance.get('/searchFields');
    return data;
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