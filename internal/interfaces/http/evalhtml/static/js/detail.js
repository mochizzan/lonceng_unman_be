// ============================================================================
// detail.js — Entry point for /eval/:npm (per-student detail page).
// Wires the primary doc filter, the secondary semester/tahun subfilter,
// and the Extract Data button with extraction verification.
// ============================================================================

import {
    applyDetailFilter,
    applyDetailSubFilter,
} from './shared/filter.js';

document.getElementById('docFilter')?.addEventListener('change', applyDetailFilter);
document.getElementById('subFilter')?.addEventListener('change', applyDetailSubFilter);

// ============================================================================
// Extraction Verification & Button Logic
// ============================================================================

/**
 * ExtractionStatus represents the result of verifying whether a document
 * can be extracted. It serves as the comparator for determining button states.
 */
class ExtractionStatus {
    /**
     * @param {boolean} pdfExists - Whether the raw PDF file exists on disk
     * @param {boolean} alreadyExtracted - Whether extraction JSON already exists
     * @param {boolean} canExtract - Whether extraction can be performed
     */
    constructor(pdfExists, alreadyExtracted, canExtract) {
        this.pdfExists = pdfExists;
        this.alreadyExtracted = alreadyExtracted;
        this.canExtract = canExtract;
    }

    /**
     * Returns true if the Extract Data button should be visible.
     * The button appears when extraction hasn't been done yet.
     */
    shouldShowExtractButton() {
        return !this.alreadyExtracted;
    }

    /**
     * Returns true if the Extract Data button should be enabled.
     * The button is enabled only when the PDF exists and extraction is possible.
     */
    isExtractEnabled() {
        return this.pdfExists && this.canExtract && !this.alreadyExtracted;
    }

    /**
     * Returns true if the Create GT button should be enabled.
     * GT creation requires extraction to exist as a prerequisite.
     */
    isGTEnabled() {
        return this.alreadyExtracted;
    }

    /**
     * Returns a human-readable status message for the extraction state.
     */
    getStatusMessage() {
        if (this.alreadyExtracted) {
            return "Data sudah diekstraksi";
        }
        if (!this.pdfExists) {
            return "PDF belum diunduh — unduh terlebih dahulu";
        }
        return "Siap diekstraksi";
    }
}

/**
 * Verifies extraction status for a document by checking server state.
 * This function queries the server to determine if the PDF exists
 * and whether extraction has already been performed.
 * 
 * @param {string} npm - Student NPM
 * @param {string} docType - Document type ("krs" or "khs")
 * @param {string} file - Document filename
 * @returns {Promise<ExtractionStatus>} The extraction verification result
 */
async function verifyExtractionStatus(npm, docType, file) {
    try {
        const response = await fetch(`/api/v1/eval/${npm}/${docType}/${file}/extraction-status`);
        if (!response.ok) {
            // If endpoint doesn't exist, fall back to checking via auto-extract probe
            return await probeExtractionStatus(npm, docType, file);
        }
        const data = await response.json();
        return new ExtractionStatus(
            data.data?.pdf_exists ?? false,
            data.data?.already_extracted ?? false,
            data.data?.can_extract ?? false
        );
    } catch (error) {
        console.warn("Extraction status verification failed:", error);
        // Fallback: assume PDF exists if not already extracted
        return new ExtractionStatus(true, false, true);
    }
}

/**
 * Probes extraction status by attempting a lightweight check.
 * Used as fallback when the dedicated status endpoint is unavailable.
 * 
 * @param {string} npm - Student NPM
 * @param {string} docType - Document type ("krs" or "khs")
 * @param {string} file - Document filename
 * @returns {Promise<ExtractionStatus>} The extraction verification result
 */
async function probeExtractionStatus(npm, docType, file) {
    try {
        // Check if extraction already exists by probing the cache
        const cacheResponse = await fetch(`/api/v1/eval/${npm}/${docType}/${file}/auto-extract`, {
            method: 'HEAD',
            headers: { 'X-Probe': 'true' }
        });
        const alreadyExtracted = cacheResponse.status === 200;
        return new ExtractionStatus(true, alreadyExtracted, true);
    } catch {
        return new ExtractionStatus(true, false, true);
    }
}

/**
 * Performs extraction for a document by calling the extract-and-save API.
 * This endpoint persists the extraction to disk so it survives page refresh.
 * 
 * @param {string} npm - Student NPM
 * @param {string} docType - Document type ("krs" or "khs")
 * @param {string} file - Document filename
 * @returns {Promise<object>} The extraction result
 */
async function performExtraction(npm, docType, file) {
    // BUG FIX: Use extract-and-save endpoint instead of auto-extract
    // auto-extract only returns JSON but does NOT persist to disk
    // extract-and-save parses the PDF AND writes to extractDir
    const response = await fetch(`/api/v1/eval/${npm}/${docType}/${file}/extract-and-save`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json'
        }
    });

    if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || `Extraction failed with status ${response.status}`);
    }

    return response.json();
}

/**
 * Updates the UI state of extract and GT buttons based on extraction status.
 * 
 * @param {HTMLElement} button - The extract button element
 * @param {HTMLElement|null} gtButton - The GT button element (if exists)
 * @param {ExtractionStatus} status - The extraction verification result
 */
function updateButtonStates(button, gtButton, status) {
    // Update Extract Data button
    if (status.alreadyExtracted) {
        button.disabled = true;
        button.innerHTML = '<i class="bi bi-check-lg"></i> Sudah Diekstraksi';
        button.classList.remove('btn-primary');
        button.classList.add('btn-outline-success');
    } else if (!status.pdfExists) {
        button.disabled = true;
        button.innerHTML = '<i class="bi bi-magic"></i> Extract Data';
        button.title = "PDF belum diunduh — unduh terlebih dahulu";
    } else {
        button.disabled = false;
        button.innerHTML = '<i class="bi bi-magic"></i> Extract Data';
    }

    // Update Create GT button based on extraction verification result
    if (gtButton) {
        if (status.isGTEnabled()) {
            gtButton.classList.remove('disabled');
            gtLinkRemoveDisabled(gtButton);
        } else {
            gtButton.classList.add('disabled');
            gtLinkAddDisabled(gtButton);
        }
    }
}

/**
 * Removes disabled state from a link-based GT button.
 */
function gtLinkRemoveDisabled(link) {
    link.classList.remove('disabled');
    link.removeAttribute('aria-disabled');
    link.removeAttribute('tabindex');
    const href = link.getAttribute('data-href');
    if (href) {
        link.setAttribute('href', href);
        link.removeAttribute('data-href');
    }
}

/**
 * Adds disabled state to a link-based GT button.
 */
function gtLinkAddDisabled(link) {
    link.classList.add('disabled');
    link.setAttribute('aria-disabled', 'true');
    link.setAttribute('tabindex', '-1');
    const href = link.getAttribute('href');
    if (href) {
        link.setAttribute('data-href', href);
        link.removeAttribute('href');
    }
}

/**
 * Shows a toast notification to the user.
 * 
 * @param {string} message - The message to display
 * @param {string} type - The notification type: "success", "error", "warning", "info"
 */
function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toastContainer') || createToastContainer();
    const toastId = `toast-${Date.now()}`;
    const bgClass = {
        success: 'bg-success',
        error: 'bg-danger',
        warning: 'bg-warning',
        info: 'bg-info'
    }[type] || 'bg-info';

    const toastHTML = `
        <div id="${toastId}" class="toast align-items-center ${bgClass} text-white border-0" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="d-flex">
                <div class="toast-body">${message}</div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
            </div>
        </div>
    `;

    toastContainer.insertAdjacentHTML('beforeend', toastHTML);
    const toastElement = document.getElementById(toastId);
    const bsToast = new bootstrap.Toast(toastElement, { delay: 5000 });
    bsToast.show();

    toastElement.addEventListener('hidden.bs.toast', () => {
        toastElement.remove();
    });
}

/**
 * Creates the toast container if it doesn't exist.
 */
function createToastContainer() {
    const container = document.createElement('div');
    container.id = 'toastContainer';
    container.className = 'toast-container position-fixed top-0 end-0 p-3';
    container.style.zIndexIndex = '9999';
    document.body.appendChild(container);
    return container;
}

/**
 * Initializes all Extract Data buttons on the detail page.
 * Wires click handlers and performs initial extraction verification.
 */
function initializeExtractButtons() {
    const extractButtons = document.querySelectorAll('.extract-btn');

    extractButtons.forEach(button => {
        const npm = button.dataset.npm;
        const docType = button.dataset.doctype;
        const file = button.dataset.file;

        // Find the corresponding GT button in the same card
        const cardBody = button.closest('.card-body');
        const gtButton = cardBody?.querySelector('a.btn-success');

        // Perform initial extraction verification
        verifyExtractionStatus(npm, docType, file).then(status => {
            updateButtonStates(button, gtButton, status);
        }).catch(error => {
            console.error("Failed to verify extraction status:", error);
            // Enable button as fallback
            button.disabled = false;
        });

        // Wire click handler for extraction
        button.addEventListener('click', async () => {
            if (button.disabled) return;

            const originalHTML = button.innerHTML;
            button.disabled = true;
            button.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Mengekstrak...';

            try {
                const result = await performExtraction(npm, docType, file);
                showToast(`Berhasil mengekstrak data ${docType.toUpperCase()} untuk ${npm}`, 'success');

                // Update button state after successful extraction
                const newStatus = new ExtractionStatus(true, true, true);
                updateButtonStates(button, gtButton, newStatus);

                // Optionally reload to show updated comparison
                setTimeout(() => location.reload(), 1500);
            } catch (error) {
                console.error("Extraction failed:", error);
                showToast(`Gagal mengekstrak: ${error.message}`, 'error');
                button.innerHTML = originalHTML;
                button.disabled = false;
            }
        });
    });
}

// Initialize extract buttons when DOM is ready
document.addEventListener('DOMContentLoaded', initializeExtractButtons);
