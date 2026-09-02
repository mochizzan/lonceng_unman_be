// ============================================================================
// confirm-modal.js — Reusable Bootstrap modal confirmation dialog
// Used by edit-krs.js and edit-khs.js for overwrite confirmation
// ============================================================================

/**
 * Show a confirmation modal for overwriting Ground Truth data.
 * Returns a Promise that resolves to true if user confirms, false otherwise.
 * The modal is created dynamically and removed after dismissal.
 * 
 * @returns {Promise<boolean>} true if user clicked "Ya", false otherwise
 */
export function confirmOverwrite() {
    return new Promise((resolve) => {
        // Remove any existing modal instance to prevent stacking
        const existingModal = document.getElementById('overwriteModal');
        if (existingModal) {
            const existingInstance = bootstrap.Modal.getInstance(existingModal);
            if (existingInstance) {
                existingInstance.dispose();
            }
            existingModal.remove();
        }

        // Create modal markup
        const modalHtml = `
            <div class="modal fade" id="overwriteModal" tabindex="-1" 
                 aria-labelledby="overwriteModalLabel" aria-hidden="true">
                <div class="modal-dialog modal-dialog-centered">
                    <div class="modal-content">
                        <div class="modal-header">
                            <h5 class="modal-title" id="overwriteModalLabel">
                                <i class="bi bi-exclamation-triangle-fill text-warning me-2"></i>
                                Timpa Ground Truth?
                            </h5>
                            <button type="button" class="btn-close" 
                                    data-bs-dismiss="modal" aria-label="Tutup"></button>
                        </div>
                        <div class="modal-body">
                            <p>Data Ground Truth sudah ada. Apakah Anda yakin ingin menimpa data tersebut?</p>
                            <p class="text-body-secondary small mb-0">
                                Perubahan yang disimpa tidak dapat dikembalikan.
                            </p>
                        </div>
                        <div class="modal-footer">
                            <button type="button" class="btn btn-outline-secondary" 
                                    data-bs-dismiss="modal" id="overwriteNo">Tidak</button>
                            <button type="button" class="btn btn-primary" 
                                    id="overwriteYes">Ya</button>
                        </div>
                    </div>
                </div>
            </div>
        `;

        document.body.insertAdjacentHTML('beforeend', modalHtml);

        const modalEl = document.getElementById('overwriteModal');
        const modal = new bootstrap.Modal(modalEl, {
            keyboard: true,
            focus: true,
            backdrop: true
        });

        // Handle "Ya" button click
        document.getElementById('overwriteYes').addEventListener('click', () => {
            modal.hide();
            resolve(true);
        });

        // Handle "Tidak" button click
        document.getElementById('overwriteNo').addEventListener('click', () => {
            modal.hide();
            resolve(false);
        });

        // Handle modal hidden event (backdrop click, ESC, close button)
        modalEl.addEventListener('hidden.bs.modal', () => {
            modalEl.remove();
            resolve(false);
        });

        modal.show();
    });
}
