// ============================================================================
// edit-khs.js — Entry point for /eval/:npm/khs/:file/edit
// Wires: row add/remove, PDF zoom toolbar, form submit to POST GT.
// ============================================================================

import { postJSON } from './shared/api-client.js';
import { bindPdfZoom } from './shared/pdf-zoom.js';
import { appendRow, bindRowRemovers, readRows } from './shared/row-ops.js';
import { wireAutoExtract } from './shared/auto-extract.js';

// Column schema must mirror the KHS table in edit_khs.html.
const KHS_COLUMNS = [
    { name: 'no', type: 'number' },
    { name: 'kode', type: 'text' },
    { name: 'nama', type: 'text' },
    { name: 'dosen', type: 'text' },
    { name: 'sks', type: 'number' },
    { name: 'nilai', type: 'text' },
    { name: 'mutu', type: 'number' },
];

bindRowRemovers();
bindPdfZoom();

// Auto-extract: populate form from PDF parser output
wireAutoExtract({
    npm: document.querySelector('#gtForm')?.dataset.npm || '',
    file: document.querySelector('#gtForm')?.dataset.file || '',
    docType: 'khs',
    tableSelector: '#mkTable',
    columns: KHS_COLUMNS,
    extractHeader: (data) => {
        const khs = data.khs?.khs || data.khs || {};
        const m = khs.mahasiswa || {};
        const p = khs.periode || {};
        const ta = p.tahun_ajaran || {};
        const r = khs.rekapitulasi || {};
        return {
            npm: m.npm || '',
            nama: m.nama || '',
            program_studi: m.program_studi || '',
            semester: p.semester || '',
            tahun_awal: ta.awal || '',
            tahun_akhir: ta.akhir || '',
            total_sks: r.total_sks || '',
            total_mutu: r.total_mutu || '',
            ipk: r.ipk || '',
        };
    },
    extractCourses: (data) => {
        const khs = data.khs?.khs || data.khs || {};
        return khs.mata_kuliah || [];
    },
    mapCourseRow: (c) => ({
        no: c.no || '',
        kode: c.kode || '',
        nama: c.nama || '',
        dosen: c.dosen || '',
        sks: c.sks || '',
        nilai: c.nilai || '',
        mutu: c.mutu || '',
    }),
});

document.querySelector('.add-row')?.addEventListener('click', () => {
    appendRow('#mkTable', KHS_COLUMNS);
});

document.getElementById('gtForm')?.addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.currentTarget;
    const formData = new FormData(form);

    const data = {
        confirm_overwrite: formData.get('confirm_overwrite') === 'true',
        khs: {
            khs: {
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
                mata_kuliah: readRows('#mkTable', KHS_COLUMNS),
                rekapitulasi: {
                    total_sks: parseInt(formData.get('total_sks') || '0', 10),
                    total_mutu: parseInt(formData.get('total_mutu') || '0', 10),
                    ipk: parseFloat(formData.get('ipk') || '0'),
                },
            },
        },
    };

    const npm = formData.get('npm');
    const file = form.dataset.file;
    const result = await postJSON(`/api/v1/eval/${npm}/khs/${file}`, data);

    if (result.ok) {
        window.location.href = `/eval/${npm}`;
    } else {
        alert(result.message || 'Gagal menyimpan');
    }
});
