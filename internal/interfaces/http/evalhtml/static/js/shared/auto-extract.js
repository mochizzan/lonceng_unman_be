// ============================================================================
// shared/auto-extract.js
// Shared helpers for auto-extracting PDF data into the GT edit forms.
// Used by edit-krs.js and edit-khs.js to populate forms from parser output.
// ============================================================================

import { postJSON } from './api-client.js';
import { appendRow } from './row-ops.js';

/**
 * Sets the value of a form element by name.
 * @param {HTMLFormElement} form
 * @param {string} name
 * @param {string|number} value
 */
function setField(form, name, value) {
    const el = form.querySelector(`[name="${name}"]`);
    if (el) el.value = value ?? '';
}

/**
 * Clears all data rows from a table's tbody.
 * @param {string} tableSelector
 */
function clearRows(tableSelector) {
    const tbody = document.querySelector(tableSelector + ' tbody');
    if (tbody) tbody.innerHTML = '';
}

/**
 * Populates the MK table with course data.
 * @param {string} tableSelector - CSS selector for the table.
 * @param {Array<{name: string, type: string}>} columns - Column schema.
 * @param {Array<object>} courses - Course data from extraction.
 * @param {(course: object) => Record<string, string|number>} mapper - Maps course to row values.
 */
function populateTable(tableSelector, columns, courses, mapper) {
    clearRows(tableSelector);
    const tbody = document.querySelector(tableSelector + ' tbody');
    if (!tbody) return;
    courses.forEach((course) => {
        const values = mapper(course);
        appendRow(tableSelector, columns);
        const lastRow = tbody.lastElementChild;
        if (!lastRow) return;
        const inputs = lastRow.querySelectorAll('input');
        columns.forEach((col, i) => {
            if (inputs[i]) inputs[i].value = values[col.name] ?? '';
        });
    });
}

/**
 * Wires the auto-extract button to fetch parsed PDF data and populate the form.
 *
 * @param {object} options
 * @param {string} options.npm - Student NPM.
 * @param {string} options.file - GT filename (e.g. "semester_8.json").
 * @param {string} options.docType - "krs" or "khs".
 * @param {string} options.tableSelector - CSS selector for the MK table.
 * @param {Array<{name: string, type: string}>} options.columns - Column schema.
 * @param {(data: object) => object} options.extractHeader - Extracts header fields from parser output.
 * @param {(data: object) => Array<object>} options.extractCourses - Extracts course list from parser output.
 * @param {(course: object) => Record<string, string|number>} options.mapCourseRow - Maps course to row values.
 */
export function wireAutoExtract(options) {
    const { npm, file, docType, tableSelector, columns, extractHeader, extractCourses, mapCourseRow } = options;

    const btn = document.getElementById('autoExtractBtn');
    const status = document.getElementById('autoExtractStatus');
    if (!btn) return;

    btn.addEventListener('click', async () => {
        btn.disabled = true;
        const originalText = btn.innerHTML;
        btn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Mengekstrak...';
        if (status) status.textContent = '';

        try {
            const result = await postJSON(`/api/v1/eval/${npm}/${docType}/${file}/auto-extract`, {});

            if (!result.ok) {
                if (status) {
                    status.textContent = result.message || 'Gagal mengekstrak';
                    status.className = 'ms-2 align-self-center small text-danger';
                }
                btn.disabled = false;
                btn.innerHTML = originalText;
                return;
            }

            const data = result.data;
            if (!data) {
                if (status) {
                    status.textContent = 'Tidak ada data dikembalikan';
                    status.className = 'ms-2 align-self-center small text-warning';
                }
                btn.disabled = false;
                btn.innerHTML = originalText;
                return;
            }

            const form = document.getElementById('gtForm');
            if (!form) return;

            // Populate header fields
            const header = extractHeader(data);
            Object.entries(header).forEach(([name, value]) => {
                setField(form, name, value);
            });

            // Populate MK table
            const courses = extractCourses(data);
            if (courses && courses.length > 0) {
                populateTable(tableSelector, columns, courses, mapCourseRow);
            }

            if (status) {
                status.textContent = `Berhasil! ${courses?.length || 0} mata kuliah diekstrak.`;
                status.className = 'ms-2 align-self-center small text-success';
            }
        } catch (err) {
            if (status) {
                status.textContent = 'Gagal menghubungi server: ' + (err.message || 'error');
                status.className = 'ms-2 align-self-center small text-danger';
            }
        } finally {
            btn.disabled = false;
            btn.innerHTML = originalText;
        }
    });
}
