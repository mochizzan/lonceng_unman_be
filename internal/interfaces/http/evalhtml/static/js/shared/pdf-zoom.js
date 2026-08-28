// ============================================================================
// shared/pdf-zoom.js
// PDF iframe zoom controls for the edit pages. Reads zoom level from the
// #zoomLevel span and rewrites the #zoom fragment on the #pdfFrame iframe.
// The zoom state is module-scoped; re-binding on the same page resets it.
// ============================================================================

const MIN_ZOOM = 50;
const MAX_ZOOM = 300;
const ZOOM_STEP = 25;

let zoom = 100;

function getZoomLevelEl() {
    return document.getElementById('zoomLevel');
}

function getPdfFrame() {
    return document.getElementById('pdfFrame');
}

function updateZoom() {
    const frame = getPdfFrame();
    const levelEl = getZoomLevelEl();
    if (frame) {
        frame.src = frame.src.replace(/#zoom=\d+/, '#zoom=' + zoom);
    }
    if (levelEl) {
        levelEl.textContent = zoom + '%';
    }
}

export function zoomIn() {
    zoom = Math.min(zoom + ZOOM_STEP, MAX_ZOOM);
    updateZoom();
}

export function zoomOut() {
    zoom = Math.max(zoom - ZOOM_STEP, MIN_ZOOM);
    updateZoom();
}

export function resetZoom() {
    zoom = 100;
    updateZoom();
}

/**
 * Wires toolbar buttons (.zoom-in, .zoom-out, .zoom-reset) to the zoom
 * controls and initializes the displayed level.
 */
export function bindPdfZoom() {
    document.querySelector('.zoom-in')?.addEventListener('click', zoomIn);
    document.querySelector('.zoom-out')?.addEventListener('click', zoomOut);
    document.querySelector('.zoom-reset')?.addEventListener('click', resetZoom);
    updateZoom();
}
