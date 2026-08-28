// ============================================================================
// student.js — Student page: fetches and displays paginated student list.
// Implements Bootstrap 5 native pagination with configurable page size.
// Default: 25 per page, configurable: 25, 50, 100.
// ============================================================================

const container = document.getElementById('studentListContainer');
const searchInput = document.getElementById('searchInput');
const perPageSelect = document.getElementById('perPageSelect');
const paginationControls = document.getElementById('paginationControls');
const prevPageItem = document.getElementById('prevPageItem');
const nextPageItem = document.getElementById('nextPageItem');
const prevPageLink = document.getElementById('prevPageLink');
const nextPageLink = document.getElementById('nextPageLink');
const showingStart = document.getElementById('showingStart');
const showingEnd = document.getElementById('showingEnd');
const totalRecords = document.getElementById('totalRecords');

// Pagination state
let currentPage = 1;
let perPage = 25;
let totalPages = 1;
let totalCount = 0;
let allStudents = [];
let filteredStudents = [];

// ============================================================================
// Data Fetching
// ============================================================================

async function loadStudents() {
    try {
        container.innerHTML = `
            <div class="text-center py-5">
                <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Memuat...</span>
                </div>
                <p class="text-body-secondary mt-3">Memuat data mahasiswa...</p>
            </div>`;

        const resp = await fetch('/api/v1/eval/students');
        const payload = await resp.json();

        if (payload.status === 'success' && Array.isArray(payload.data)) {
            allStudents = payload.data;
            filteredStudents = [...allStudents];
            totalCount = allStudents.length;
            totalPages = Math.ceil(totalCount / perPage);
            renderCurrentPage();
            renderPaginationControls();
        } else {
            container.innerHTML = `
                <div class="text-center py-5">
                    <i class="bi bi-exclamation-circle display-4 text-warning"></i>
                    <p class="text-body-secondary mt-3">Gagal memuat data mahasiswa.</p>
                </div>`;
        }
    } catch (err) {
        container.innerHTML = `
            <div class="text-center py-5">
                <i class="bi bi-wifi-off display-4 text-danger"></i>
                <p class="text-body-secondary mt-3">Tidak dapat terhubung ke server.</p>
                <p class="text-body-tertiary small">${err.message}</p>
            </div>`;
    }
}

// ============================================================================
// Pagination Logic
// ============================================================================

function renderCurrentPage() {
    const search = (searchInput.value || '').toLowerCase();

    // Filter students based on search
    filteredStudents = allStudents.filter((s) => {
        const matchSearch = !search ||
            (s.npm || '').toLowerCase().includes(search) ||
            (s.name || '').toLowerCase().includes(search);
        return matchSearch;
    });

    totalCount = filteredStudents.length;
    totalPages = Math.ceil(totalCount / perPage);

    // Ensure current page is valid
    if (currentPage > totalPages) {
        currentPage = Math.max(1, totalPages);
    }

    // Calculate slice indices
    const startIndex = (currentPage - 1) * perPage;
    const endIndex = Math.min(startIndex + perPage, totalCount);
    const pageStudents = filteredStudents.slice(startIndex, endIndex);

    // Update page info display
    if (totalCount === 0) {
        showingStart.textContent = '0';
        showingEnd.textContent = '0';
    } else {
        showingStart.textContent = String(startIndex + 1);
        showingEnd.textContent = String(endIndex);
    }
    totalRecords.textContent = String(totalCount);

    // Render the student rows
    renderStudentRows(pageStudents);

    // Update pagination controls
    renderPaginationControls();
    updatePaginationButtons();
}

function renderStudentRows(students) {
    if (students.length === 0) {
        container.innerHTML = `
            <div class="text-center py-5">
                <i class="bi bi-search display-4 text-body-secondary"></i>
                <p class="lead text-body-secondary mt-3">Tidak ada mahasiswa ditemukan.</p>
            </div>`;
        return;
    }

    const rows = students.map((s) => `
        <tr>
            <td><a href="/eval/${s.npm}" class="text-decoration-none fw-semibold">${s.npm}</a></td>
            <td>${s.name || '-'}</td>
            <td class="text-center">
                <a href="/eval/${s.npm}" class="btn btn-sm btn-outline-info"><i class="bi bi-eye"></i> Detail</a>
                <a href="/eval/krs?npm=${s.npm}" class="btn btn-sm btn-outline-primary"><i class="bi bi-journal-text"></i> KRS</a>
                <a href="/eval/khs?npm=${s.npm}" class="btn btn-sm btn-outline-success"><i class="bi bi-journal-check"></i> KHS</a>
            </td>
        </tr>
    `).join('');

    container.innerHTML = `
        <div class="card border-0 shadow-sm">
            <div class="card-body p-0">
                <div class="table-responsive">
                    <table class="table table-hover table-striped table-sm align-middle mb-0">
                        <thead class="table-dark">
                            <tr>
                                <th>NPM</th>
                                <th>Nama</th>
                                <th class="text-center">Aksi</th>
                            </tr>
                        </thead>
                        <tbody>${rows}</tbody>
                    </table>
                </div>
            </div>
        </div>
    `;
}

// ============================================================================
// Bootstrap 5 Pagination Component
// ============================================================================

function renderPaginationControls() {
    // Clear existing page number items (keep prev and next)
    const pageItems = paginationControls.querySelectorAll('.page-item:not(#prevPageItem):not(#nextPageItem)');
    pageItems.forEach(item => item.remove());

    if (totalPages <= 1) {
        paginationControls.style.display = 'none';
        return;
    }

    paginationControls.style.display = 'flex';

    // Generate page number items
    const pages = getVisiblePages();

    pages.forEach((pageNum, index) => {
        // Insert before the next button
        const nextItem = document.getElementById('nextPageItem');

        if (pageNum === '...') {
            const ellipsisItem = document.createElement('li');
            ellipsisItem.className = 'page-item disabled';
            ellipsisItem.innerHTML = '<span class="page-link">...</span>';
            paginationControls.insertBefore(ellipsisItem, nextItem);
        } else {
            const pageItem = document.createElement('li');
            pageItem.className = `page-item ${pageNum === currentPage ? 'active' : ''}`;
            if (pageNum === currentPage) {
                pageItem.setAttribute('aria-current', 'page');
            }
            pageItem.innerHTML = `<a class="page-link" href="#" data-page="${pageNum}">${pageNum}</a>`;
            paginationControls.insertBefore(pageItem, nextItem);
        }
    });

    // Wire click handlers for new page links
    paginationControls.querySelectorAll('.page-link[data-page]').forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const page = parseInt(link.dataset.page, 10);
            if (!isNaN(page) && page !== currentPage) {
                goToPage(page);
            }
        });
    });
}

function getVisiblePages() {
    const pages = [];
    const maxVisible = 5; // Maximum number of page buttons to show

    if (totalPages <= maxVisible + 2) {
        // Show all pages if total is small
        for (let i = 1; i <= totalPages; i++) {
            pages.push(i);
        }
    } else {
        // Always show first page
        pages.push(1);

        // Calculate range around current page
        let start = Math.max(2, currentPage - 1);
        let end = Math.min(totalPages - 1, currentPage + 1);

        // Adjust if near the beginning
        if (currentPage <= 3) {
            end = 4;
        }

        // Adjust if near the end
        if (currentPage >= totalPages - 2) {
            start = totalPages - 3;
        }

        // Add ellipsis after first page if needed
        if (start > 2) {
            pages.push('...');
        }

        // Add pages in range
        for (let i = start; i <= end; i++) {
            pages.push(i);
        }

        // Add ellipsis before last page if needed
        if (end < totalPages - 1) {
            pages.push('...');
        }

        // Always show last page
        pages.push(totalPages);
    }

    return pages;
}

function updatePaginationButtons() {
    // Update Previous button
    if (currentPage <= 1) {
        prevPageItem.classList.add('disabled');
        prevPageLink.setAttribute('aria-disabled', 'true');
        prevPageLink.setAttribute('tabindex', '-1');
    } else {
        prevPageItem.classList.remove('disabled');
        prevPageLink.removeAttribute('aria-disabled');
        prevPageLink.removeAttribute('tabindex');
    }

    // Update Next button
    if (currentPage >= totalPages) {
        nextPageItem.classList.add('disabled');
        nextPageLink.setAttribute('aria-disabled', 'true');
        nextPageLink.setAttribute('tabindex', '-1');
    } else {
        nextPageItem.classList.remove('disabled');
        nextPageLink.removeAttribute('aria-disabled');
        nextPageLink.removeAttribute('tabindex');
    }
}

function goToPage(page) {
    if (page < 1 || page > totalPages) return;
    currentPage = page;
    renderCurrentPage();
    // Scroll to top of container
    container.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

// ============================================================================
// Event Listeners
// ============================================================================

// Search input
searchInput.addEventListener('input', () => {
    currentPage = 1; // Reset to first page on search
    renderCurrentPage();
});

// Per page select
perPageSelect.addEventListener('change', () => {
    perPage = parseInt(perPageSelect.value, 10);
    currentPage = 1; // Reset to first page on page size change
    totalPages = Math.ceil(totalCount / perPage);
    renderCurrentPage();
});

// Previous page button
prevPageLink.addEventListener('click', (e) => {
    e.preventDefault();
    if (currentPage > 1) {
        goToPage(currentPage - 1);
    }
});

// Next page button
nextPageLink.addEventListener('click', (e) => {
    e.preventDefault();
    if (currentPage < totalPages) {
        goToPage(currentPage + 1);
    }
});

// Keyboard navigation (left/right arrows)
document.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowLeft' && currentPage > 1) {
        goToPage(currentPage - 1);
    } else if (e.key === 'ArrowRight' && currentPage < totalPages) {
        goToPage(currentPage + 1);
    }
});

// ============================================================================
// Initialize
// ============================================================================

loadStudents();
