// ============================================================================
// edit-krs.js — Entry point for /eval/:npm/krs/:file/edit
// Wires: row add/remove, PDF zoom toolbar, form submit to POST GT.
// ============================================================================

import { postJSON } from './shared/api-client.js';
import { bindPdfZoom } from './shared/pdf-zoom.js';
import { appendRow, bindRowRemovers, readRows } from './shared/row-ops.js';
import { wireAutoExtract } from './shared/auto-extract.js';
import { confirmOverwrite } from './shared/confirm-modal.js';

// Column schema must mirror the KRS table in edit_krs.html.
const KRS_COLUMNS = [
    { name: 'no', type: 'number' },
    { name: 'kode', type: 'text' },
    { name: 'nama', type: 'text' },
    { name: 'sks', type: 'number' },
    { name: 'kelas', type: 'text' },
    { name: 'dosen', type: 'text' },
    { name: 'hari', type: 'text' },
    { name: 'waktu_mulai', type: 'text' },
    { name: 'waktu_selesai', type: 'text' },
];

bindRowRemovers();
bindPdfZoom();

// Auto-extract: populate form from PDF parser output
wireAutoExtract({
    npm: document.querySelector('#gtForm')?.dataset.npm || '',
    file: document.querySelector('#gtForm')?.dataset.file || '',
    docType: 'krs',
    tableSelector: '#mkTable',
    columns: KRS_COLUMNS,
    extractHeader: (data) => {
        const krs = data.krs?.krs || data.krs || {};
        const m = krs.mahasiswa || {};
        const p = krs.periode || {};
        const ta = p.tahun_ajaran || {};
        return {
            npm: m.npm || '',
            nama: m.nama || '',
            program_studi: m.program_studi || '',
            semester: p.semester || '',
            tahun_awal: ta.awal || '',
            tahun_akhir: ta.akhir || '',
            total_sks: krs.total_sks || '',
        };
    },
    extractCourses: (data) => {
        const krs = data.krs?.krs || data.krs || {};
        return krs.mata_kuliah || [];
    },
    mapCourseRow: (c) => ({
        no: c.no || '',
        kode: c.kode || '',
        nama: c.nama || '',
        sks: c.sks || '',
        kelas: c.kelas || '',
        dosen: c.dosen || '',
        hari: c.jadwal?.hari || '',
        waktu_mulai: c.jadwal?.waktu_mulai || '',
        waktu_selesai: c.jadwal?.waktu_selesai || '',
    }),
});

document.querySelector('.add-row')?.addEventListener('click', () => {
    appendRow('#mkTable', KRS_COLUMNS);
});

document.getElementById('gtForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.currentTarget;
    
    // Check if GT exists via data attribute
    const gtExists = form.dataset.gtExists === 'true';
    
    // Show confirmation modal only when GT exists
    if (gtExists) {
        const confirmed = await confirmOverwrite();
        if (!confirmed) return; // User clicked "Tidak" — abort submit
    }
    
    const formData = new FormData(form);

    const data = {
        confirm_overwrite: gtExists,  // true only if user confirmed via modal
        krs: {
            krs: {
                mahasiswa: {
                    npm: formData.get('npm'),
                    nama: formData.get('nama'),
                    program_studi: formData.get('program_studi'),
                },
                periode: {
                    semester: formData.get('semester'),
                    tahun_ajaran: {
                        awal: formData.get('tahun_awal'),
                        akhir: formData.get('tahun_akhir'),
                    },
                },
                mata_kuliah: readRows('#mkTable', KRS_COLUMNS).map((r) => ({
                    no: r.no,
                    kode: r.kode,
                    nama: r.nama,
                    sks: r.sks,
                    kelas: r.kelas,
                    dosen: r.dosen,
                    jadwal: {
                        hari: r.hari,
                        waktu_mulai: r.waktu_mulai,
                        waktu_selesai: r.waktu_selesai,
                    },
                })),
                total_sks: parseInt(formData.get('total_sks') || '0', 10),
            },
        },
    };

    const npm = formData.get('npm');
    const file = form.dataset.file;
    const result = await postJSON(`/api/v1/eval/${npm}/krs/${file}`, data);

    if (result.ok) {
        window.location.href = `/eval/${npm}`;
    } else {
        alert(result.message || 'Gagal menyimpan');
    }
});
