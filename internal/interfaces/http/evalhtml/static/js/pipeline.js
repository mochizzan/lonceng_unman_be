// ============================================================================
// pipeline.js — Pipeline Visualization page: tab switching + continuous
// pipeline animation. 11 stages, particle flow, progress tracking.
// Vanilla ES module — no external libraries required.
// ============================================================================

// ============================================================================
// Pipeline Stages — 11 phases matching the HTML structure
// ============================================================================
const PIPELINE_STAGES = [
    { id: 1,  name: 'PDF Reading',         icon: 'bi-file-earmark-pdf',     color: '#3498db' },
    { id: 2,  name: 'Coordinate Transform', icon: 'bi-arrow-down',           color: '#2ecc71' },
    { id: 3,  name: 'Row Grouping',        icon: 'bi-grid-3x3',             color: '#17a2b8' },
    { id: 4,  name: 'Row Construction',    icon: 'bi-layout-text-sidebar-reverse', color: '#f39c12' },
    { id: 5,  name: 'Column Detection',    icon: 'bi-layout-columns',        color: '#e74c3c' },
    { id: 6,  name: 'Column Extraction',   icon: 'bi-text-paragraph',       color: '#3498db' },
    { id: 7,  name: 'Smart Word Split',    icon: 'bi-magic',                color: '#2ecc71' },
    { id: 8,  name: 'KRS Parsing',         icon: 'bi-journal-text',         color: '#17a2b8' },
    { id: 9,  name: 'KHS Parsing',         icon: 'bi-journal-check',        color: '#f39c12' },
    { id: 10, name: 'Deduplication',       icon: 'bi-funnel',               color: '#e74c3c' },
    { id: 11, name: 'JSON Output',         icon: 'bi-file-earmark-code',    color: '#3498db' },
];

// ============================================================================
// Animation State
// ============================================================================
const ANIMATION_STATE = {
    currentStageIndex: 0,
    isAnimating: false,
    isPaused: false,
    particlesActive: 0,
    maxConcurrentParticles: 6,
    stageHoldMs: 1800,    // Time a stage stays active
    particleDuration: 600, // Particle travel time between stages
    completionFlashMs: 1200,
};

// ============================================================================
// DOM References (populated on init)
// ============================================================================
let dom = {};

// ============================================================================
// Tab Switching — Bootstrap native tab API
// ============================================================================
function initTabSwitching() {
    const tabButtons = document.querySelectorAll('#pipelineTabs [data-bs-toggle="tab"]');
    tabButtons.forEach(tabButton => {
        tabButton.addEventListener('shown.bs.tab', (event) => {
            const targetId = event.target.getAttribute('data-bs-target');
            if (targetId === '#animation') {
                ANIMATION_STATE.isPaused = false;
                if (!ANIMATION_STATE.isAnimating) {
                    startAnimationLoop();
                } else {
                    resumeAnimation();
                }
            } else {
                pauseAnimation();
            }
        });
    });
}

// ============================================================================
// Animation Loop — continuous infinite cycle through all 11 stages
// ============================================================================
function startAnimationLoop() {
    if (ANIMATION_STATE.isAnimating) return;
    ANIMATION_STATE.isAnimating = true;
    ANIMATION_STATE.currentStageIndex = 0;
    resetAllStages();
    runStageSequence();
}

function pauseAnimation() {
    ANIMATION_STATE.isPaused = true;
}

function resumeAnimation() {
    ANIMATION_STATE.isPaused = false;
    // Resume from current stage if animation is still running
    if (ANIMATION_STATE.isAnimating) {
        runStageSequence();
    }
}

/**
 * Advances through one full cycle of all 11 stages, then restarts.
 * Uses setTimeout for stage timing and spawns particles between stages.
 */
async function runStageSequence() {
    while (ANIMATION_STATE.isAnimating && !ANIMATION_STATE.isPaused) {
        const idx = ANIMATION_STATE.currentStageIndex;
        const stage = PIPELINE_STAGES[idx];

        // Activate current stage
        activateStage(stage);
        updateProgressUI(stage, idx);

        // Wait for the stage hold duration
        await sleep(ANIMATION_STATE.stageHoldMs);

        if (ANIMATION_STATE.isPaused || !ANIMATION_STATE.isAnimating) break;

        // Determine destination (next stage, or wrap to first after last)
        const nextIdx = (idx + 1) % PIPELINE_STAGES.length;
        const nextStage = PIPELINE_STAGES[nextIdx];

        // Mark current stage as completed
        markStageCompleted(stage);

        // Spawn particles flowing to next stage
        spawnParticles(stage, nextStage);

        // Wait for particles to travel (minus a small overlap so flow feels continuous)
        await sleep(Math.max(ANIMATION_STATE.particleDuration - 200, 200));

        if (ANIMATION_STATE.isPaused || !ANIMATION_STATE.isAnimating) break;

        // Advance to next stage
        ANIMATION_STATE.currentStageIndex = nextIdx;

        // If we've completed all 11 stages, show completion flash then restart
        if (ANIMATION_STATE.currentStageIndex === 0) {
            await showCompletionFlash();
        }
    }
}

// ============================================================================
// Stage Visual States
// ============================================================================
function activateStage(stage) {
    const el = getStageElement(stage.id);
    if (!el) return;
    // Remove completed state from all, then set this one active
    el.classList.remove('completed');
    el.classList.add('active');
    // Update the icon to glowing
    const iconEl = el.querySelector('.stage-icon');
    if (iconEl) {
        iconEl.style.boxShadow = `0 0 20px ${stage.color}80, 0 0 40px ${stage.color}40`;
        iconEl.style.transform = 'scale(1.15)';
    }
}

function markStageCompleted(stage) {
    const el = getStageElement(stage.id);
    if (!el) return;
    el.classList.remove('active');
    el.classList.add('completed');
    const iconEl = el.querySelector('.stage-icon');
    if (iconEl) {
        iconEl.style.boxShadow = 'none';
        iconEl.style.transform = 'scale(1)';
    }
    // Swap icon to checkmark
    const iconI = iconEl?.querySelector('i');
    if (iconI) {
        iconI.className = 'bi bi-check-lg';
        iconI.style.color = stage.color;
    }
}

function resetAllStages() {
    PIPELINE_STAGES.forEach((stage) => {
        const el = getStageElement(stage.id);
        if (!el) return;
        el.classList.remove('active', 'completed');
        const iconEl = el.querySelector('.stage-icon');
        if (iconEl) {
            iconEl.style.boxShadow = '';
            iconEl.style.transform = '';
        }
        // Restore original icon
        const iconI = iconEl?.querySelector('i');
        if (iconI) {
            iconI.className = `bi ${stage.icon}`;
            iconI.style.color = '';
        }
    });
}

function getStageElement(stageId) {
    return document.getElementById(`stage-${stageId}`);
}

// ============================================================================
// Particle Animation — dynamically created, CSS transform-based
// ============================================================================
function spawnParticles(sourceStage, destStage) {
    // Limit concurrent particles for performance
    const availableSlots = ANIMATION_STATE.maxConcurrentParticles - ANIMATION_STATE.particlesActive;
    if (availableSlots <= 0) return;

    const count = Math.min(3, availableSlots); // Spawn up to 3 per transition
    for (let i = 0; i < count; i++) {
        // Stagger particle creation slightly for visual richness
        setTimeout(() => {
            createParticle(sourceStage, destStage);
        }, i * 120);
    }
}

function createParticle(sourceStage, destStage) {
    const sourceEl = getStageElement(sourceStage.id);
    const destEl = getStageElement(destStage.id);
    if (!sourceEl || !destEl) return;

    ANIMATION_STATE.particlesActive++;

    const sourceRect = sourceEl.getBoundingClientRect();
    const destRect = destEl.getBoundingClientRect();

    // Calculate relative translation from source to destination
    const dx = destRect.left + destRect.width / 2 - (sourceRect.left + sourceRect.width / 2);
    const dy = destRect.top + destRect.height / 2 - (sourceRect.top + sourceRect.height / 2);

    // Create particle element
    const particle = document.createElement('div');
    particle.className = 'data-particle';
    particle.style.cssText = `
        position: fixed;
        left: ${sourceRect.left + sourceRect.width / 2}px;
        top: ${sourceRect.top + sourceRect.height / 2}px;
        width: 12px;
        height: 12px;
        border-radius: 50%;
        background: ${sourceStage.color};
        box-shadow: 0 0 8px ${sourceStage.color}90, 0 0 16px ${sourceStage.color}40;
        transform: translate(-50%, -50%);
        transition: transform ${ANIMATION_STATE.particleDuration}ms cubic-bezier(0.4, 0, 0.2, 1);
        z-index: 1000;
        pointer-events: none;
    `;

    document.body.appendChild(particle);

    // Force reflow so transition triggers
    particle.offsetHeight;

    // Animate to destination
    particle.style.transform = `translate(calc(-50% + ${dx}px), calc(-50% + ${dy}px)) scale(0.6)`;

    // Cleanup after animation
    const cleanup = () => {
        particle.remove();
        ANIMATION_STATE.particlesActive = Math.max(0, ANIMATION_STATE.particlesActive - 1);
    };

    // Use transitionend for precise cleanup, with setTimeout fallback
    let cleanedUp = false;
    const onTransitionEnd = (e) => {
        if (e.propertyName === 'transform' && !cleanedUp) {
            cleanedUp = true;
            cleanup();
        }
    };
    particle.addEventListener('transitionend', onTransitionEnd, { once: true });
    // Fallback: force cleanup after duration + buffer
    setTimeout(() => {
        if (!cleanedUp) {
            cleanedUp = true;
            particle.removeEventListener('transitionend', onTransitionEnd);
            cleanup();
        }
    }, ANIMATION_STATE.particleDuration + 200);
}

// ============================================================================
// Completion Flash — brief celebratory flash after all 11 stages complete
// ============================================================================
async function showCompletionFlash() {
    const container = dom.animationContainer;
    if (!container) return;

    // Flash all stages briefly
    PIPELINE_STAGES.forEach((stage) => {
        const el = getStageElement(stage.id);
        if (el) {
            el.style.boxShadow = `0 0 30px ${stage.color}60`;
        }
    });

    // Show completion label
    if (dom.completionLabel) {
        dom.completionLabel.classList.add('show');
    }

    await sleep(ANIMATION_STATE.completionFlashMs);

    // Reset flash
    PIPELINE_STAGES.forEach((stage) => {
        const el = getStageElement(stage.id);
        if (el) {
            el.style.boxShadow = '';
        }
    });

    if (dom.completionLabel) {
        dom.completionLabel.classList.remove('show');
    }

    // Small pause before restarting the loop
    await sleep(400);
    resetAllStages();
}

// ============================================================================
// Progress UI Updates
// ============================================================================
function updateProgressUI(stage, index) {
    // Progress bar fill
    if (dom.progressBar) {
        const pct = Math.round(((index + 1) / PIPELINE_STAGES.length) * 100);
        dom.progressBar.style.width = `${pct}%`;
    }

    // Stage counter
    if (dom.stageCounter) {
        dom.stageCounter.textContent = `Stage ${stage.id} / ${PIPELINE_STAGES.length}`;
    }

    // Current stage label
    if (dom.currentStageLabel) {
        dom.currentStageLabel.textContent = stage.name;
        dom.currentStageLabel.style.color = stage.color;
    }

    // Active stage description
    if (dom.stageDescription) {
        dom.stageDescription.textContent = getStageDescription(stage.id);
    }
}

/**
 * Returns a brief description for each stage to display in the info panel.
 */
function getStageDescription(stageId) {
    const descriptions = {
        1: 'ReadPDFWithPosition() — Mengekstrak span teks dengan posisi X,Y dari PDF',
        2: 'flipY = pageHeight - span.Y — Membalik sumbu Y dari origin PDF (bawah) ke layar (atas)',
        3: 'roundY(y) = float64(int(y*4))/4.0 — Membulatkan Y ke 0.25 terdekat untuk mengelompokkan baris',
        4: 'RowToLine() — Mengelompokkan kata berdasarkan posisi X, menggabungkan tanpa spasi',
        5: 'FindColumnPositions() — Mendeteksi batas kolom dari header, menghitung midpoint',
        6: 'ExtractColumnsFromRow() — Mengambil kata dalam rentang [Start, End) per kolom',
        7: 'SmartWordSplit() — Memisahkan kata CamelCase, digit, dan deteksi singkatan',
        8: 'parseKRSMataKuliah() — Memindai semua baris, deduplikasi kode:kelas',
        9: 'parseKHSMataKuliah() — Entri multi-baris (nama, data, dosen), deduplikasi no',
        10: 'Deduplicasi — KRS: kode:kelas, KHS: no — Mencegah entri mata kuliah duplikat',
        11: 'MarshalToJSON() — Serialisasi data terstruktur ke format JSON',
    };
    return descriptions[stageId] || '';
}

// ============================================================================
// Utility
// ============================================================================
function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}

// ============================================================================
// Collect DOM References
// ============================================================================
function collectDomRefs() {
    dom = {
        animationContainer: document.getElementById('pipeline-animation-container'),
        progressBar: document.getElementById('pipelineProgressBar'),
        stageCounter: document.getElementById('stageCounter'),
        currentStageLabel: document.getElementById('currentStageLabel'),
        stageDescription: document.getElementById('stageDescription'),
        completionLabel: document.getElementById('completionLabel'),
        formulaTab: document.getElementById('formulas-tab'),
        animationTab: document.getElementById('animation-tab'),
    };
}

// ============================================================================
// Initialize — DOMContentLoaded entry point
// ============================================================================
document.addEventListener('DOMContentLoaded', () => {
    collectDomRefs();
    initTabSwitching();

    // Auto-start animation if the animation tab is already active on load
    const animationTabBtn = dom.animationTab;
    if (animationTabBtn && animationTabBtn.classList.contains('active')) {
        startAnimationLoop();
    }
});
