// ============================================================================
// shared/filter.js
// Generic table-row filter logic. Pure functions over (root, selectors) so
// it can be reused by index (unified table) and detail (per-section subfilter)
// without modification. The DOM contract is:
//   - rows expose dataset.category, dataset.npm, dataset.semester
//   - filter sources are: activeTab (string|attribute value), npm,
//     semester (all string|null|'' = no constraint)
// ============================================================================

/**
 * Returns the value of the active tab button (data-filter attribute).
 * `root` should contain a `[data-filter].active` element; returns 'all' if absent.
 */
function readActiveTab(root) {
    const active = root.querySelector('#categoryTabs [data-filter].active');
    return active ? active.getAttribute('data-filter') : 'all';
}

/**
 * Index page: filters rows in #tableBody by tab + npm + semester.
 * Idempotent: re-running just re-applies the current state.
 */
export function applyIndexFilter() {
    const root = document;
    const activeTab = readActiveTab(root);
    const npm = root.querySelector('#filterNPM')?.value || '';
    const semester = root.querySelector('#filterSemester')?.value || '';

    root.querySelectorAll('#tableBody tr').forEach((row) => {
        const rowCat = (row.getAttribute('data-category') || '').toLowerCase();
        const rowNpm = row.getAttribute('data-npm') || '';
        const rowSem = row.getAttribute('data-semester') || '';

        let show = true;
        if (activeTab !== 'all' && rowCat !== activeTab) show = false;
        if (show && npm && rowNpm !== npm) show = false;
        if (show && semester && rowSem !== semester) show = false;

        row.style.display = show ? '' : 'none';
    });
}

/**
 * Updates the semester dropdown label and options based on the active tab.
 * KRS → "Semester:" with numbers, KHS → "Istilah:" with tahun ajaran,
 * Semua → "Semester/Tahun:" with all values.
 */
export function updateSemesterDropdown() {
    const root = document;
    const activeTab = readActiveTab(root);
    const label = root.querySelector('#subFilterLabel');
    const select = root.querySelector('#filterSemester');

    // Update label
    if (activeTab === 'krs') {
        label.innerHTML = '<i class="bi bi-funnel"></i> Semester:';
    } else if (activeTab === 'khs') {
        label.innerHTML = '<i class="bi bi-funnel"></i> Istilah:';
    } else {
        label.innerHTML = '<i class="bi bi-funnel"></i> Semester/Tahun:';
    }

    // Collect values from visible rows based on active tab
    const values = new Set();
    root.querySelectorAll('#tableBody tr').forEach((row) => {
        const rowCat = (row.getAttribute('data-category') || '').toLowerCase();
        const rowSem = row.getAttribute('data-semester') || '';
        
        if (activeTab === 'all') {
            if (rowSem) values.add(rowSem);
        } else if (rowCat === activeTab && rowSem) {
            values.add(rowSem);
        }
    });

    // Rebuild options
    select.innerHTML = '<option value="">Semua</option>';
    Array.from(values)
        .sort((a, b) => {
            // Numeric sort for semester, alphabetical for tahun
            const numA = parseInt(a), numB = parseInt(b);
            if (!isNaN(numA) && !isNaN(numB)) return numA - numB;
            return a.localeCompare(b);
        })
        .forEach((v) => {
            const opt = document.createElement('option');
            opt.value = v;
            opt.textContent = v;
            select.appendChild(opt);
        });
}

/**
 * Populates the index page's #filterNPM select by scanning the rendered #tableBody.
 */
export function populateIndexFilters() {
    const root = document;
    const npmSet = new Set();

    root.querySelectorAll('#tableBody tr').forEach((row) => {
        const npm = row.getAttribute('data-npm');
        if (npm) npmSet.add(npm);
    });

    const npmSel = root.querySelector('#filterNPM');
    if (!npmSel) return;
    
    npmSet.forEach((npm) => {
        const opt = document.createElement('option');
        opt.value = npm;
        opt.textContent = npm;
        npmSel.appendChild(opt);
    });

    // Populate semester dropdown based on initial active tab
    updateSemesterDropdown();
}

/**
 * Wires tab buttons to re-apply the index filter on click.
 * Idempotent within a page load — safe to call from the page bootstrap.
 */
export function bindIndexTabs() {
    document.querySelectorAll('#categoryTabs [data-filter]').forEach((btn) => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('#categoryTabs [data-filter]').forEach((b) => b.classList.remove('active'));
            btn.classList.add('active');
            updateSemesterDropdown();
            applyIndexFilter();
        });
    });
}

// ---------- Detail-page filter helpers ----------

/**
 * Detail page: toggles the krsSection/khsSection visibility based on the
 * primary docFilter value, and shows/hides the subFilter controls.
 */
export function applyDetailFilter() {
    const filter = document.getElementById('docFilter').value;
    const krsSection = document.getElementById('krsSection');
    const khsSection = document.getElementById('khsSection');
    const subFilterLabel = document.getElementById('subFilterLabel');
    const subFilter = document.getElementById('subFilter');

    if (filter === 'krs') {
        krsSection.style.display = 'block';
        khsSection.style.display = 'none';
        subFilterLabel.style.display = 'inline';
        subFilter.style.display = 'inline';
        populateSubFilter('krs');
    } else if (filter === 'khs') {
        krsSection.style.display = 'none';
        khsSection.style.display = 'block';
        subFilterLabel.style.display = 'inline';
        subFilter.style.display = 'inline';
        populateSubFilter('khs');
    } else {
        krsSection.style.display = 'block';
        khsSection.style.display = 'block';
        subFilterLabel.style.display = 'none';
        subFilter.style.display = 'none';
    }
}

/**
 * Populates the subFilter <select> with semester (KRS) or tahun (KHS) values
 * harvested from the matching section's .doc-section elements.
 */
export function populateSubFilter(docType) {
    const subFilter = document.getElementById('subFilter');
    subFilter.innerHTML = '<option value="all">Semua</option>';

    const sections = document.querySelectorAll('#' + docType + 'Section .doc-section');
    const values = new Set();
    sections.forEach((s) => {
        const val = docType === 'krs' ? s.dataset.semester : s.dataset.tahun;
        if (val) values.add(val);
    });

    Array.from(values)
        .sort()
        .forEach((v) => {
            const opt = document.createElement('option');
            opt.value = v;
            opt.textContent = v;
            subFilter.appendChild(opt);
        });
}

/**
 * Detail page: narrows visible .doc-section elements inside the active
 * primary filter by their semester/tahun data attribute.
 */
export function applyDetailSubFilter() {
    const docFilter = document.getElementById('docFilter').value;
    const subFilter = document.getElementById('subFilter').value;

    const sections = document.querySelectorAll('#' + docFilter + 'Section .doc-section');
    sections.forEach((s) => {
        const val = docFilter === 'krs' ? s.dataset.semester : s.dataset.tahun;
        if (subFilter === 'all' || val === subFilter) {
            s.style.display = 'block';
        } else {
            s.style.display = 'none';
        }
    });
}
