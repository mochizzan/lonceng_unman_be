// ============================================================================
// krs.js — KRS page: fetches cached KRS data by NPM via POST /api/v1/lms/krs/data.
// Uses Bootstrap 5 card layout with responsive table.
// ============================================================================

import { postJSON } from '/eval/static/js/shared/api-client.js';

const container = document.getElementById('krsDataContainer');
const npmInput = document.getElementById('npmInput');
const loadBtn = document.getElementById('loadKRSBtn');

// Parse NPM from query string (e.g., ?npm=12345678)
const params = new URLSearchParams(window.location.search);
const initialNpm = params.get('npm');
if (initialNpm) {
    npmInput.value = initialNpm;
    loadKRS(initialNpm);
}

loadBtn.addEventListener('click', () => {
    const npm = npmInput.value.trim();
    if (npm) loadKRS(npm);
});

npmInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
        const npm = npmInput.value.trim();
        if (npm) loadKRS(npm);
    }
});

async function loadKRS(npm) {
    container.innerHTML = `
        <div class="text-center py-5">
            <div class="spinner-border text-primary" role="status">
                <span class="visually-hidden">Memuat...</span>
            </div>
            <p class="text-body-secondary mt-3">Memuat data KRS untuk NPM ${npm}...</p>
        </div>`;

    try {
        const resp = await postJSON('/api/v1/lms/krs/data', { npm });
        if (resp.ok && resp.data) {
            renderKRS(resp.data);
        } else {
            container.innerHTML = `
                <div class="alert alert-warning" role="alert">
                    <i class="bi bi-exclamation-triangle"></i> ${resp.message || 'Data KRS tidak ditemukan untuk NPM ini.'}
                    <hr>
                    <p class="mb-0 small">Pastikan PDF KRS telah diekstrak terlebih dahulu melalui POST /api/v1/lms/krs/extract.</p>
                </div>`;
        }
    } catch (err) {
        container.innerHTML = `
            <div class="alert alert-danger" role="alert">
                <i class="bi bi-x-circle"></i> Gagal memuat data: ${err.message}
            </div>`;
    }
}

function renderKRS(data) {
    const krs = data.krs || data.KRS || data;
    const mahasiswa = krs.mahasiswa || krs.Mahasiswa || {};
    const periode = krs.periode || krs.Periode || {};
    const mataKuliah = krs.mata_kuliah || krs.MataKuliah || [];
    const totalSKS = krs.total_sks || krs.TotalSKS || 0;
    const penerbitan = krs.penerbitan || krs.Penerbitan || {};

    const rows = (Array.isArray(mataKuliah) ? mataKuliah : []).map((mk) => `
        <tr>
            <td>${mk.no || mk.No || ''}</td>
            <td>${mk.kode || mk.Kode || ''}</td>
            <td>${mk.nama || mk.Nama || ''}</td>
            <td class="text-center">${mk.sks || mk.SKS || ''}</td>
            <td>${mk.kelas || mk.Kelas || ''}</td>
            <td>${mk.dosen || mk.Dosen || ''}</td>
            <td>${formatJadwal(mk.jadwal || mk.Jadwal)}</td>
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

                    <dt class="col-sm-3 col-lg-2 text-body-secondary">Total SKS</dt>
                    <dd class="col-sm-9 col-lg-10">${totalSKS}</dd>
                </dl>
            </div>
        </div>

        <!-- Mata Kuliah Table -->
        <div class="card border-0 shadow-sm mb-4">
            <div class="card-header bg-primary text-white">
                <h5 class="mb-0"><i class="bi bi-journal-text"></i> Mata Kuliah</h5>
            </div>
            <div class="card-body p-0">
                <div class="table-responsive">
                    <table class="table table-hover table-striped table-sm align-middle mb-0">
                        <thead class="table-dark">
                            <tr>
                                <th class="text-center">No</th>
                                <th>Kode</th>
                                <th>Mata Kuliah</th>
                                <th class="text-center">SKS</th>
                                <th>Kelas</th>
                                <th>Dosen</th>
                                <th>Jadwal</th>
                            </tr>
                        </thead>
                        <tbody>${rows || '<tr><td colspan="7" class="text-center text-body-secondary py-3">Tidak ada data mata kuliah.</td></tr>'}</tbody>
                    </table>
                </div>
            </div>
        </div>
    `;
}

function formatJadwal(jadwal) {
    if (!jadwal) return '-';
    const hari = jadwal.hari || jadwal.Hari || '';
    const mulai = jadwal.waktu_mulai || jadwal.WaktuMulai || '';
    const selesai = jadwal.waktu_selesai || jadwal.WaktuSelesai || '';
    if (!hari) return '-';
    return `${hari}, ${mulai} - ${selesai}`;
}

function formatTahunAjaran(ta) {
    if (!ta) return '-';
    const awal = ta.awal || ta.Awal || '';
    const akhir = ta.akhir || ta.Akhir || '';
    if (!awal) return '-';
    return `${awal}/${akhir}`;
}