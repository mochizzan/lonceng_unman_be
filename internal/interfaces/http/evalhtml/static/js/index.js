// ============================================================================
// index.js — Dashboard page: renders Chart.js confusion matrix visualizations.
// Uses server-side UnifiedEval data embedded in the HTML by the Go template.
// ============================================================================

function getChartColors() {
    const style = getComputedStyle(document.documentElement);
    return {
        tp: style.getPropertyValue('--eval-success').trim() || '#27ae60',
        fn: style.getPropertyValue('--eval-danger').trim() || '#e74c3c',
        fp: style.getPropertyValue('--eval-warning').trim() || '#f39c12',
        tn: style.getPropertyValue('--eval-info').trim() || '#17a2b8',
        primary: style.getPropertyValue('--eval-primary').trim() || '#3498db',
    };
}

function buildDoughnutChart(canvasId, labels, data, colors) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    new Chart(canvas, {
        type: 'doughnut',
        data: {
            labels,
            datasets: [{
                data,
                backgroundColor: colors,
                borderWidth: 2,
                borderColor: '#fff',
            }],
        },
        options: {
            responsive: false,
            maintainAspectRatio: true,
            aspectRatio: 1.5,
            plugins: {
                legend: { position: 'bottom', labels: { padding: 12, usePointStyle: true } },
                tooltip: {
                    callbacks: {
                        label: (ctx) => {
                            const total = ctx.dataset.data.reduce((a, b) => a + b, 0);
                            const pct = total > 0 ? ((ctx.parsed / total) * 100).toFixed(1) : '0.0';
                            return `${ctx.label}: ${ctx.parsed} (${pct}%)`;
                        },
                    },
                },
            },
        },
    });
}

function buildComparisonChart(canvasId) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return;
    const colors = getChartColors();
    const krsP = parseFloat(canvas.dataset.krsPrecision) || 0;
    const krsR = parseFloat(canvas.dataset.krsRecall) || 0;
    const krsF1 = parseFloat(canvas.dataset.krsF1) || 0;
    const khsP = parseFloat(canvas.dataset.khsPrecision) || 0;
    const khsR = parseFloat(canvas.dataset.khsRecall) || 0;
    const khsF1 = parseFloat(canvas.dataset.khsF1) || 0;
    new Chart(canvas, {
        type: 'bar',
        data: {
            labels: ['Precision', 'Recall', 'F1'],
            datasets: [
                {
                    label: 'KRS',
                    data: [krsP, krsR, krsF1],
                    backgroundColor: colors.primary,
                    borderRadius: 4,
                },
                {
                    label: 'KHS',
                    data: [khsP, khsR, khsF1],
                    backgroundColor: colors.tp,
                    borderRadius: 4,
                },
            ],
        },
        options: {
            responsive: false,
            maintainAspectRatio: true,
            aspectRatio: 2,
            scales: {
                y: { beginAtZero: true, max: 1, ticks: { stepSize: 0.2 } },
            },
            plugins: {
                legend: { position: 'top' },
            },
        },
    });
}

// Read embedded data from HTML data attributes set by Go template.
function readMetricsFromCanvas(canvasId) {
    const canvas = document.getElementById(canvasId);
    if (!canvas) return null;
    return {
        tp: parseInt(canvas.dataset.tp, 10) || 0,
        fn: parseInt(canvas.dataset.fn, 10) || 0,
        fp: parseInt(canvas.dataset.fp, 10) || 0,
        tn: parseInt(canvas.dataset.tn, 10) || 0,
        precision: parseFloat(canvas.dataset.precision) || 0,
        recall: parseFloat(canvas.dataset.recall) || 0,
        f1: parseFloat(canvas.dataset.f1) || 0,
    };
}

document.addEventListener('DOMContentLoaded', () => {
    const colors = getChartColors();
    const labels = ['True Positive', 'False Negative', 'False Positive', 'True Negative'];
    const colorList = [colors.tp, colors.fn, colors.fp, colors.tn];

    const krs = readMetricsFromCanvas('krsChart');
    if (krs) {
        buildDoughnutChart('krsChart', labels, [krs.tp, krs.fn, krs.fp, krs.tn], colorList);
    }

    const khs = readMetricsFromCanvas('khsChart');
    if (khs) {
        buildDoughnutChart('khsChart', labels, [khs.tp, khs.fn, khs.fp, khs.tn], colorList);
    }

    // Always render comparison chart (zeros when no data)
    buildComparisonChart('comparisonChart');
});