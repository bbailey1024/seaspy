let polySquareFuncMap = new Map();
polySquareFuncMap.set("outline", polySquareOutline);
polySquareFuncMap.set("fill", polySquareFill);

let polyShipFuncMap = new Map();
polyShipFuncMap.set("outline", polyShipOutline);
polyShipFuncMap.set("fill", polyShipFill);

let polyCircleFuncMap = new Map();
polyCircleFuncMap.set("outline", polyCircleOutline);
polyCircleFuncMap.set("fill", polyCircleFill);

// colorMap returns a map of draw types to their respective colors.
// Includes transparency values if fade boolean is true.
export function colorMap(color, fade) {
    let colorMap = new Map();

    if (fade) {
        colorMap.set("outline", "#00000033");
        colorMap.set("fill", color + "80");
    } else {
        colorMap.set("outline", "#000000");
        colorMap.set("fill", color);
    }
    return colorMap;
}

// drawShape returns the draw function associated with a ship marker type.
// This is defined by the server where 0 = anchored, 1 = moving, 2 = stopped.
export function drawShape(marker) {
    var drawShape;
    switch (marker) {
        case 0: 
            drawShape = drawCircle2D;
            break;
        case 1:
            drawShape = drawPolygon2D;
            break;
        case 2:
            drawShape = drawPolygon2D;
            break;
        default:
            console.warn("marker value is undefined");
    }
    return drawShape;
}

// polyMap returns the polygon function map associated with a ship marker type.
// This is defined by the server where 0 = anchored, 1 = moving, 2 = stopped.
export function polyMap(marker) {
    var polyMap;
    switch (marker) {
        case 0: 
            polyMap = polyCircleFuncMap;
            break;
        case 1:
            polyMap = polyShipFuncMap;
            break;
        case 2:
            polyMap = polySquareFuncMap;
            break;
        default:
            console.warn("marker value is undefined");
    }
    return polyMap;
}

// clipPositions returns an array of tile coordinate offsets and a center position for each tile.
export function clipPositions(center, shape, tileSize) {
    let start = shape[0];
    let shapeDimensions = polyDimensions(shape);

    let clipping = {x: 0, y: 0};
    if (start.x + shapeDimensions.x > tileSize) clipping.x = 1;
    if (start.x - shapeDimensions.x < shapeDimensions.x) clipping.x = -1;
    if (start.y + shapeDimensions.y > tileSize) clipping.y = 1;
    if (start.y - shapeDimensions.y < shapeDimensions.y) clipping.y = -1;
    
    let results = [];

    if (clipping.x == 1) {
        results.push({ x: 1, y: 0, center: {x: center.x - tileSize, y: center.y}});
    }

    if (clipping.y == 1) {
        results.push({ x: 0, y: 1, center: {x: center.x, y: center.y - tileSize}});
    }            

    if (clipping.x == -1) {
        results.push({ x: -1, y: 0, center: {x: center.x + tileSize, y: center.y}});
    }

    if (clipping.y == -1) {
        results.push({ x: 0, y: -1, center: {x: center.x, y: center.y + tileSize}});
    }

    if (clipping.x == 1 && clipping.y == 1) {
        results.push({ x: 1, y: 1, center: {x: center.x - tileSize, y: center.y - tileSize}});
    }

    if (clipping.x == -1 && clipping.y == -1) {
        results.push({ x: -1, y: -1, center: {x: center.x + tileSize, y: center.y + tileSize}});
    }

    if (clipping.x == 1 && clipping.y == -1) {
        results.push({ x: 1, y: -1, center: {x: center.x - tileSize, y: center.y + tileSize}});
    }

    if (clipping.x == -1 && clipping.y == 1) {
        results.push({ x: -1, y: 1, center: {x: center.x + tileSize, y: center.y - tileSize}});
    }

    return results;
}

function polyDimensions(shape) {
    let max = {x: shape[0].x, y: shape[0].y};
    let min = {x: shape[0].x, y: shape[0].y};
    for (let i = 1; i < shape.length; i++) {
        if (shape[i].x < min.x) min.x = shape[i].x;
        if (shape[i].x > max.x) max.x = shape[i].x;
        if (shape[i].y < min.y) min.y = shape[i].y;
        if (shape[i].y > max.y) max.y = shape[i].y;
    }
    return {x: max.x - min.x, y: max.y - min.y};
}

function polyShipFill(centerPoint, rotation) {
    const startPoint = {x: centerPoint.x - 4, y: centerPoint.y - 11};
    const poly = [
        {x: startPoint.x + 1, y: startPoint.y + 17},
        {x: startPoint.x + 1, y: startPoint.y + 7},
        {x: startPoint.x + 4, y: startPoint.y + 2},
        {x: startPoint.x + 7, y: startPoint.y + 7},
        {x: startPoint.x + 7, y: startPoint.y + 17},
        {x: startPoint.x + 1, y: startPoint.y + 17},
    ];
    return rotatePoly(poly, centerPoint, rotation);
}

function polyShipOutline(centerPoint, rotation) {
    const startPoint = {x: centerPoint.x - 4, y: centerPoint.y - 11};
    const poly = [
        {x: startPoint.x, y: startPoint.y},
        {x: startPoint.x, y: startPoint.y + 8},
        {x: startPoint.x + 4, y: startPoint.y},
        {x: startPoint.x + 8, y: startPoint.y + 8},
        {x: startPoint.x + 8, y: startPoint.y + 18},
        {x: startPoint.x, y: startPoint.y + 18},
    ];
    return rotatePoly(poly, centerPoint, rotation);
}

// polySquareFill plots the fill pattern of stopped ship polygon (square) based on a center point.
// Starting point is offset from center point at -5, -5.
function polySquareFill(centerPoint) {
    const startPoint = {x: centerPoint.x - 5, y: centerPoint.y - 5};
    return [
        {x: startPoint.x + 1, y: startPoint.y + 1},
        {x: startPoint.x + 1, y: startPoint.y + 9},
        {x: startPoint.x + 9, y: startPoint.y + 9},
        {x: startPoint.x + 9, y: startPoint.y + 1},
    ];
}

// polySquareOutline plots the outline pattern of stopped ship polygon (square) based on a center point.
// Starting point is offset from center point at -5, -5.
function polySquareOutline(centerPoint) {
    const startPoint = {x: centerPoint.x - 5, y: centerPoint.y - 5};
    return [
        {x: startPoint.x, y: startPoint.y},
        {x: startPoint.x, y: startPoint.y + 10},
        {x: startPoint.x + 10, y: startPoint.y + 10},
        {x: startPoint.x + 10, y: startPoint.y},
    ];
}

// polyCircleFill returns a square fill polygon that will be used to identify clipping.
// Calling functions requiring a circle will provide this polygon to a drawCircle function.
// This was done to support easier workflow for the tile clipping operations.
function polyCircleFill(centerPoint) {
    return polySquareFill(centerPoint);
}

// polyCircleOutline returns a square outline polygon that will be used to identify clipping.
// Calling functions requiring a circle will provide this polygon to a drawCircle function.
// This was done to support easier workflow for the tile clipping operations.
function polyCircleOutline(centerPoint) {
    return polySquareOutline(centerPoint);
}

// drawCircle derives a center point and radius from the dimensions of the provided polygon.
// In practice, for this application, it should always be a square.
// Non-square polygons will behave in an unexpected manner and will produce a console warning.
// This was done to support easier workflow for the tile clipping operations.
function drawCircle(ctx, polygon, color) {
    let dimensions = polyDimensions(polygon);
    if (dimensions.x != dimensions.y) {
        console.warn("polygon provided to drawCirle is not a square");
    }

    // The polygon center can be derived from its dimensions and starting coordinates.
    // A bit hacky here, but we know the square offsets are both negative, so it saves some complexity.
    // For anything general purpose this would need to be refined.
    let centerPoint = {x: polygon[0].x + dimensions.x/2, y: polygon[0].y + dimensions.y/2};

    let radius = dimensions.x/2;

    ctx.beginPath();
    ctx.arc(centerPoint.x, centerPoint.y, radius, 0, 2 * Math.PI, false);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.closePath();
}

function drawPolygon(ctx, polygon, color) {
    ctx.fillStyle = color;
    
    ctx.beginPath();
    ctx.moveTo(polygon[0].x, polygon[0].y);
    for (let i = 0; i < polygon.length; i++) {
        ctx.lineTo(polygon[i].x, polygon[i].y);
    }
    ctx.lineTo(polygon[0].x, polygon[0].y);
    ctx.fill();
    ctx.closePath();
}

function drawPolygon2D(ctx, polygon, color) {
    ctx.fillStyle = color;

    let t = new Path2D;    
    t.moveTo(polygon[0].x, polygon[0].y);
    for (let i = 0; i < polygon.length; i++) {
        t.lineTo(polygon[i].x, polygon[i].y);
    }
    t.lineTo(polygon[0].x, polygon[0].y);

    ctx.fill(t);

    return t;
}

// drawCircle derives a center point and radius from the dimensions of the provided polygon.
// In practice, for this application, it should always be a square.
// Non-square polygons will behave in an unexpected manner and will produce a console warning.
// This was done to support easier workflow for the tile clipping operations.
function drawCircle2D(ctx, polygon, color) {
    let dimensions = polyDimensions(polygon);
    if (dimensions.x != dimensions.y) {
        console.warn("polygon provided to drawCirle is not a square");
    }

    // The polygon center can be derived from its dimensions and starting coordinates.
    // A bit hacky here, but we know the square offsets are both negative, so it saves some complexity.
    // For anything general purpose this would need to be refined.
    let centerPoint = {x: polygon[0].x + dimensions.x/2, y: polygon[0].y + dimensions.y/2};

    let radius = dimensions.x/2;

    let t = new Path2D;
    t.arc(centerPoint.x, centerPoint.y, radius, 0, 2 * Math.PI, false);

    ctx.fillStyle = color;
    ctx.fill(t);

    return t;
}

function rotatePoly(poly, center, rotation) {
    let result = [];
    for (let i = 0; i < poly.length; i++) {
        result.push(rotatePoint(poly[i], center, rotation));
    }
    return result;
}

function rotatePoint(point, center, rotation) {
    var radians = rotation * Math.PI / 180.0;
    return {
        x: Math.cos(radians) * (point.x - center.x) - Math.sin(radians) * (point.y - center.y) + center.x,
        y: Math.sin(radians) * (point.x - center.x) + Math.cos(radians) * (point.y - center.y) + center.y
    };
}