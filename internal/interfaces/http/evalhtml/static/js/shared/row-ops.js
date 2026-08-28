// ============================================================================
// shared/row-ops.js
// Generic add/remove row operations for the edit-page MK tables.
// The form schema is supplied as a column list so the same module drives
// KRS (9 data cols) and KHS (7 data cols) tables from a single declaration.
//
// Schema entry shape: { name: string, type: 'text'|'number' }
// ============================================================================

/**
 * Appends a new editable row to the given table's tbody. Each input mirrors
 * the field name attribute declared in `columns` so the form submit handler
 * can read values consistently.
 *
 * @param {string} tableSelector - CSS selector for the <table> element.
 * @param {Array<{name: string, type: string}>} columns - Column definition.
 */
export function appendRow(tableSelector, columns) {
    const tbody = document.querySelector(tableSelector + ' tbody');
    if (!tbody) return;
    const row = document.createElement('tr');
    row.innerHTML =
        columns
            .map(
                (col) =>
                    `<td><input type="${col.type}" name="${col.name}"></td>`,
            )
            .join('') +
        '<td><button type="button" class="row-remove">Hapus</button></td>';
    tbody.appendChild(row);
}

/**
 * Wires delegated click handler so any .row-remove button (existing or
 * future) removes its enclosing <tr>. Call once per page after the table
 * is in the DOM.
 */
export function bindRowRemovers() {
    document.addEventListener('click', (e) => {
        const target = e.target;
        if (target instanceof HTMLElement && target.classList.contains('row-remove')) {
            target.closest('tr')?.remove();
        }
    });
}

/**
 * Reads all data rows from the given table and converts them into an array
 * of plain objects keyed by column.name. Skips rows whose first column is
 * empty (treated as placeholder/blank rows).
 *
 * @param {string} tableSelector
 * @param {Array<{name: string, type: string}>} columns
 * @returns {Array<Record<string, string|number>>}
 */
export function readRows(tableSelector, columns) {
    const rows = document.querySelectorAll(tableSelector + ' tbody tr');
    const out = [];
    rows.forEach((row) => {
        const inputs = row.querySelectorAll('input');
        if (!inputs[0]?.value) return;
        const obj = {};
        columns.forEach((col, i) => {
            const raw = inputs[i]?.value ?? '';
            obj[col.name] = col.type === 'number' ? parseInt(raw || '0', 10) : raw;
        });
        out.push(obj);
    });
    return out;
}
