// ============================================================================
// khs.js — KHS page: fetches cached KHS data by NPM + tahun ajaran + semester
// via POST /api/v1/lms/khs/data. Uses Bootstrap 5 card layout.
// ============================================================================

import { postJSON } from '/eval/static/js/shared/api-client.js';

const container = document.getElementById('khsDataContainer');
const npmInput = document.getElementById('npmInput');
const tahunAjaranInput = document.getElementById('tahunAjaranInput');
const semesterSelect = document.getElementById('semesterSelect');
const loadBtn = document.getElementById('loadKHSBtn');

// Parse query params
const params = new URLSearchParams(window.location.search);
const initialNpm = params.get('npm');
const initialTahun = params.get('tahun');
const initialSemester = params.get('semester');
if (initialNpm) npmInput.value = initialNpm;
if (initialTahun) tahunAjaranInput.value = initialTahun;
if (initialSemester) semesterSelect.value = initialSemester;

// Auto-load if all params present
if (initialNpm && initialTahun && initialSemester) {
    loadKHS(initialNpm, initialTahun, initialSemester);
}

loadBtn.addEventListener('click', () => {
    const npm = npmInput.value.trim();
    const tahun = tahunAjaranInput.value.trim();
    const semester = semesterSelect.value;
    if (npm && tahun) loadKHS(npm, tahun, semester);
});

[npmInput, tahunAjaranInput].forEach((el) => {
    el.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            const npm = npmInput.value.trim();
            const tahun = tahunAjaranInput.value.trim();
            const semester = semesterSelect.value;
            if (npm && tahun) loadKHS(npm, tahun, semester);
        }
    });
});

semesterSelect.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        const npm = npmInput.value.trim();
        const tahun = tahunAjaranInput.value.trim();
        const semester = semesterSelect.value;
        if (npm && tahun) loadKHS(npm, tahun, semester);
    }
});

async function loadKHS(npm, tahun, semester) {
    container.innerHTML = `
        <div class="text-center py-5">
            <div class="spinner-border text-primary" role="status">
                <span class="visually-hidden">Memuat...</span>
            </div>
            <p class="text-body-secondary mt-3">Memuat data KHS untuk NPM ${npm}...</p>
        </div>`;

    try {
        const resp = await postJSON('/api/v1/lms/khs/data', {
            npm,
            tahun_ajaran: tahun,
            semester,
        });
        if (resp.ok && resp.data) {
            renderKHS(resp.data);
        } else {
            container.innerHTML = `
                <div class="alert alert-warning" role="alert">
                    <i class="bi bi-exclamation-triangle"></i> ${resp.message || 'Data KHS tidak ditemukan.'}
                    <hr>
                    <p class="mb-0 small">Pastikan PDF KHS telah diekstrak terlebih dahulu melalui POST /api/v1/lms/khs/extract.</p>
                </div>`;
        }
    } catch (err) {
        container.innerHTML = `
            <div class="alert alert-danger" role="alert">
                <i class="bi bi-x-circle"></i> Gagal memuat data: ${err.message}
            </div>`;
    }
}

function renderKHS(data) {
    const khs = data.khs || data.KHS || data;
    const mahasiswa = khs.mahasiswa || khs.Mahasiswa || {};
    const periode = khs.periode || khs.Periode || {};
    const mataKuliah = khs.mata_kuliah || khs.MataKuliah || [];
    const rekap = khs.rekapitulasi || khs.Rekapitulasi || {};
    const penerbitan = khs.penerbitan || khs.Penerbitan || {};
    const persetujuan = khs.persetujuan || khs.Persetujuan || {};

    const rows = (Array.isArray(mataKuliah) ? mataKuliah : []).map((mk) => `
        <tr>
            <td>${mk.no || mk.No || ''}</td>
            <td>${mk.kode || mk.Kode || ''}</td>
            <td>${mk.nama || mk.Nama || ''}</td>
            <td>${mk.dosen || mk.Dosen || ''}</td>
            <td class="text-center">${mk.sks || mk.SKS || ''}</td>
            <td>${mk.nilai || mk.Nilai || ''}</td>
            <td class="text-center">${mk.mutu || mk.Mutu || ''}</td>
        </tr>
    `).join('');

    container.innerHTML = `
        <!-- Identity Card -->
        <div class="card border-0 shadow-sm mb-4">
            <div class="card-header bg-white border-bottom">
                <h5 class="mb-0 fw-semibold"><i class="bi bi-person-vcard"></i> Identitas Mahasiswa</h5>
            </div>
            <div class="card-body">
                <dl class="row mb-0">
                    <dt class="col-sm-3 col-lg-2 text-body-secondary">NPM</dt>
                    <dd class="col-sm-9 col-lg-10 fw-bold">${mahasiswa.npm || mahasiswa.NPM || '-'}</dd>

                    <dt class="col-sm-3 col-lg-2 text-body-secondary">Nama</dt>
                    <dd class="col-sm-9 col-lg-10">${mahasiswa.nama || mahasiswa.Nama || '-'}</dd>

                    <dt class="col-sm-3 col-lg-2 text-body-secondary">Program Studi</dt>
                    <dd class="col-sm-9 col-lg-10">${mahasiswa.program_studi || mahasiswa.ProgramStudi || '-'}</dd>

                    <dt class="col-sm-3 col-lg-2 text-body-secondary">Semester</dt>
                    <dd class="col-sm-9 col-lg-10">${periode.semester || periode.Semester || '-'}</dd>

                    <dt class="col-sm-3 col-lg-2 text-body-secondary">Tahun Ajaran</dt>
                    <dd class="col-sm-9 col-lg-10">${formatTahunAjaran(periode.tahun_ajaran || periode.TahunAjaran)}</dd>
                </dl>
            </div>
        </div>

        <!-- Rekapitulasi -->
        <div class="row g-3 mb-4">
            <div class="col-md-4">
                <div class="card border-0 shadow-sm bg-primary text-white">
                    <div class="card-body text-center">
                        <p class="mb-0 small opacity-75">Total SKS</p>
                        <p class="fs-3 fw-bold mb-0">${rekap.total_sks || rekap.TotalSKS || 0}</p>
                    </div>
                </div>
            </div>
            <div class="col-md-4">
                <div class="card border-0 shadow-sm bg-success text-white">
                    <div class="card-body text-center">
                        <p class="mb-0 small opacity-75">Total Mutu</p>
                        <p class="fs-3 fw-bold mb-0">${rekap.total_mutu || rekap.TotalMutu || 0}</p>
                    </div>
                </div>
            </div>
            <div class="col-md-4">
                <div class="card border-0 shadow-sm bg-info text-white">
                    <div class="card-body text-center">
                        <p class="mb-0 small opacity-75">IPK</p>
                        <p class="fs-3 fw-bold mb-0">${rekap.ipk || rekap.IPK || '0.00'}</p>
                    </div>
                </div>
            </div>
        </div>

        <!-- Mata Kuliah Table -->
        <div class="card border-0 shadow-sm mb-4">
            <div class="card-header bg-success text-white">
                <h5 class="mb-0"><i class="bi bi-journal-check"></i> Mata Kuliah & Nilai</h5>
            </div>
            <div class="card-body p-0">
                <div class="table-responsive">
                    <table class="table table-hover table-striped table-sm align-middle mb-0">
                        <thead class="table-dark">
                            <tr>
                                <th class="text-center">No</th>
                                <th>Kode</th>
                                <th>Mata Kuliah</th>
                                <th>Dosen</th>
                                <th class="text-center">SKS</th>
                                <th>Nilai</th>
                                <th class="text-center">Mutu</th>
                            </tr>
                        </thead>
                        <tbody>${rows || '<tr><td colspan="7" class="text-center text-body-secondary py-3">Tidak ada data mata kuliah.</td></tr>'}</tbody>
                    </table>
                </div>
            </div>
        </div>
    `;
}

function formatTahunAjaran(ta) {
    if (!ta) return '-';
    const awal = ta.awal || ta.Awal || '';
    const akhir = ta.akhir || ta.Akhir || '';
    if (!awal) return '-';
    return `${awal}/${akhir}`;
}