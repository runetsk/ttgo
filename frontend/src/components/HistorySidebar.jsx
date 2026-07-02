import React, { useState, useEffect, useCallback, useRef } from 'react';
import { versions as versionsApi } from '../api';
import RestoreConfirmDialog from './RestoreConfirmDialog';
import SafeHTML from './shared/SafeHTML';
import { stripHtml } from '../utils/htmlUtils';

// ─── Helpers ────────────────────────────────────────────────────────────────

const EVENT_LABELS = {
    create:  { label: 'Created',  color: '#22c55e' },
    edit:    { label: 'Edited',   color: '#3b82f6' },
    restore: { label: 'Restored', color: '#f59e0b' },
};

function safeParseSnap(raw) {
    if (!raw) return null;
    try { return typeof raw === 'string' ? JSON.parse(raw) : raw; } catch { return null; }
}


function formatCustomValue(cv) {
    if (!cv.value || cv.value === 'null') return '—';
    try {
        const parsed = JSON.parse(cv.value);
        if (typeof parsed === 'boolean') return parsed ? 'Yes' : 'No';
        if (Array.isArray(parsed)) return parsed.join(', ') || '—';
        return String(parsed) || '—';
    } catch {
        return cv.value;
    }
}

function formatDate(iso) {
    if (!iso) return '';
    return new Date(iso).toLocaleString(undefined, {
        month: 'short', day: 'numeric', year: 'numeric',
        hour: '2-digit', minute: '2-digit',
    });
}

function computeChangeSummary(snapNew, snapOld) {
    if (!snapOld) return null;

    const chips = [];

    // ── Content fields ────────────────────────────────────────────────────────
    if ((snapNew.name || '') !== (snapOld.name || ''))
        chips.push({ type: 'field', label: 'Name' });

    if (stripHtml(snapNew.description) !== stripHtml(snapOld.description))
        chips.push({ type: 'field', label: 'Description' });

    // ── Steps ─────────────────────────────────────────────────────────────────
    const stepsNew = snapNew.steps || [];
    const stepsOld = snapOld.steps || [];
    const delta = stepsNew.length - stepsOld.length;

    if (delta > 0) chips.push({ type: 'added',   label: `+${delta} step${delta > 1 ? 's' : ''}` });
    if (delta < 0) chips.push({ type: 'removed', label: `${delta} step${Math.abs(delta) > 1 ? 's' : ''}` });

    let modified = 0;
    const minLen = Math.min(stepsNew.length, stepsOld.length);
    for (let i = 0; i < minLen; i++) {
        if (stripHtml(stepsNew[i].action)          !== stripHtml(stepsOld[i].action) ||
            stripHtml(stepsNew[i].expected_result) !== stripHtml(stepsOld[i].expected_result))
            modified++;
    }
    if (modified > 0)
        chips.push({ type: 'modified', label: `~${modified} step${modified > 1 ? 's' : ''}` });

    // ── Categories ────────────────────────────────────────────────────────────────
    const categoryIdsNew = new Set((snapNew.categories || []).map(s => s.id));
    const categoryIdsOld = new Set((snapOld.categories || []).map(s => s.id));
    const categoriesAdded   = [...categoryIdsNew].filter(id => !categoryIdsOld.has(id)).length;
    const categoriesRemoved = [...categoryIdsOld].filter(id => !categoryIdsNew.has(id)).length;
    if (categoriesAdded > 0)
        chips.push({ type: 'added',   label: `+${categoriesAdded} ${categoriesAdded > 1 ? 'categories' : 'category'}` });
    if (categoriesRemoved > 0)
        chips.push({ type: 'removed', label: `-${categoriesRemoved} ${categoriesRemoved > 1 ? 'categories' : 'category'}` });

    // ── Custom field values ───────────────────────────────────────────────────
    const cvNewMap = Object.fromEntries((snapNew.custom_values || []).map(cv => [cv.field_id, cv.value]));
    const cvOldMap = Object.fromEntries((snapOld.custom_values || []).map(cv => [cv.field_id, cv.value]));
    const allFieldIds = new Set([...Object.keys(cvNewMap), ...Object.keys(cvOldMap)]);
    for (const fid of allFieldIds) {
        if (cvNewMap[fid] !== cvOldMap[fid]) {
            const cv = (snapNew.custom_values || []).find(c => c.field_id === fid)
                    || (snapOld.custom_values || []).find(c => c.field_id === fid);
            chips.push({ type: 'field', label: cv?.field_name || 'Custom field' });
        }
    }

    if (chips.length === 0)
        chips.push({ type: 'none', label: 'No changes' });

    return chips;
}

const CHIP_STYLES = {
    field:    { background: '#1e3a5f', color: '#93c5fd' },
    added:    { background: '#14532d', color: '#4ade80' },
    removed:  { background: '#4c0519', color: '#f87171' },
    modified: { background: '#451a03', color: '#fcd34d' },
    none:     { background: 'transparent', color: '#555' },
};

// ─── SnapshotPreview (right panel) ──────────────────────────────────────────

function SnapshotPreview({ version, onRestore, restoring }) {
    const snap = safeParseSnap(version.snapshot);
    const meta = EVENT_LABELS[version.event_type] || { label: version.event_type, color: '#888' };

    if (!snap) {
        return <div style={styles.previewEmpty}>Unable to parse snapshot.</div>;
    }

    return (
        <div style={styles.preview}>
            {/* version header */}
            <div style={styles.previewHeader}>
                <span style={{ ...styles.badge, background: meta.color }}>{meta.label}</span>
                <span style={styles.previewTimestamp}>{formatDate(version.created_at)}</span>
                {version.user_name && (
                    <span style={styles.previewAuthor}>by {version.user_name}</span>
                )}
            </div>

            <div style={styles.previewDivider} />

            {/* snapshot content */}
            <div style={styles.previewBody}>
                <div style={styles.previewSection}>
                    <div style={styles.previewLabel}>Name</div>
                    <div style={styles.previewValue}>{snap.name || <em style={{ color: '#555' }}>—</em>}</div>
                </div>

                {(snap.categories || []).length > 0 && (
                    <div style={styles.previewSection}>
                        <div style={styles.previewLabel}>Categories</div>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
                            {snap.categories.map(s => (
                                <span key={s.id} style={styles.categoryBadge}>{s.name}</span>
                            ))}
                        </div>
                    </div>
                )}

                {(snap.custom_values || []).length > 0 && (
                    <div style={styles.previewSection}>
                        <div style={styles.previewLabel}>Custom Fields</div>
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            {snap.custom_values.map(cv => (
                                <div key={cv.field_id} style={styles.cvRow}>
                                    <span style={styles.cvLabel}>{cv.field_name || cv.field_id}</span>
                                    <span style={styles.cvValue}>{formatCustomValue(cv)}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {stripHtml(snap.description) && (
                    <div style={styles.previewSection}>
                        <div style={styles.previewLabel}>Description</div>
                        <SafeHTML style={styles.previewValue} html={snap.description} />
                    </div>
                )}

                {(snap.steps || []).length > 0 && (
                    <div style={styles.previewSection}>
                        <div style={styles.previewLabel}>Steps</div>
                        <ol style={styles.stepList}>
                            {snap.steps.map((s, i) => (
                                <li key={s.id || i} style={styles.stepItem}>
                                    <SafeHTML style={styles.stepText} html={s.action} />
                                    {stripHtml(s.expected_result) && (
                                        <div style={styles.stepExpected}>
                                            Expected:{' '}
                                            <SafeHTML as="span" html={s.expected_result} />
                                        </div>
                                    )}
                                </li>
                            ))}
                        </ol>
                    </div>
                )}
            </div>

            {/* restore button */}
            <div style={styles.previewFooter}>
                <button
                    style={{ ...styles.restoreBtn, opacity: restoring ? 0.6 : 1 }}
                    onClick={() => onRestore(version)}
                    disabled={restoring}
                >
                    {restoring ? 'Restoring…' : 'Restore this version'}
                </button>
            </div>
        </div>
    );
}

// ─── Main component ──────────────────────────────────────────────────────────

export default function HistorySidebar({ testCaseId, onClose, onTestCaseRestored }) {
    const [versionList, setVersionList]     = useState([]);
    const [loading, setLoading]             = useState(true);
    const [error, setError]                 = useState(null);
    const [selectedId, setSelectedId]       = useState(null);
    const [restoreTarget, setRestoreTarget] = useState(null);
    const [restoring, setRestoring]         = useState(false);
    // Tracks whether a version list has ever loaded successfully, so fetchVersions
    // (below) can skip the full-loading spinner on refetches (avoids UI flicker on
    // restore/retry) without needing versionList itself as a dependency — reading
    // versionList.length directly would give fetchVersions a new identity on every
    // successful load and re-trigger the mount effect, causing a fetch loop.
    const hasLoadedRef = useRef(false);

    const fetchVersions = useCallback(async () => {
        if (!hasLoadedRef.current) setLoading(true);
        setError(null);
        try {
            const data = await versionsApi.list(testCaseId);
            hasLoadedRef.current = true;
            setVersionList(data);
            // auto-select newest version
            if (data.length > 0) setSelectedId(id => id ?? data[0].id);
        } catch {
            setError('Failed to load history. Please try again.');
        } finally {
            setLoading(false);
        }
    }, [testCaseId]);

    useEffect(() => { fetchVersions(); }, [fetchVersions]);

    // close on Escape
    useEffect(() => {
        const handler = e => { if (e.key === 'Escape') onClose(); };
        document.addEventListener('keydown', handler);
        return () => document.removeEventListener('keydown', handler);
    }, [onClose]);

    async function handleConfirmRestore(version) {
        setRestoring(true);
        try {
            const updated = await versionsApi.restore(testCaseId, version.id);
            setSelectedId(null); // reset so auto-select picks newest after refresh
            await fetchVersions();
            onTestCaseRestored?.(updated);
            setRestoreTarget(null);
        } catch {
            // global axios interceptor shows toast
        } finally {
            setRestoring(false);
        }
    }

    const selectedVersion = versionList.find(v => v.id === selectedId);

    return (
        <>
            {/* Backdrop — uses project CSS class for blur + fade animation */}
            <div
                className="modal-overlay"
                style={{ zIndex: 1500 }}
                onClick={onClose}
            >
                {/* Dialog — stop propagation so clicks inside don't close */}
                <div style={styles.dialog} onClick={e => e.stopPropagation()}>

                    {/* Header */}
                    <div style={styles.header}>
                        <span style={styles.title}>Version History</span>
                        <button style={styles.closeBtn} onClick={onClose} title="Close">✕</button>
                    </div>

                    {/* Body */}
                    <div style={styles.body}>
                        {/* ── Left: version list ── */}
                        <div style={styles.listPanel}>
                            {loading ? (
                                <div style={styles.center}>Loading…</div>
                            ) : error ? (
                                <div style={styles.errorBox}>
                                    <span>{error}</span>
                                    <button style={styles.retryBtn} onClick={fetchVersions}>Retry</button>
                                </div>
                            ) : versionList.length === 0 ? (
                                <div style={styles.center}>No history yet.</div>
                            ) : (
                                <ul style={styles.list}>
                                    {versionList.map((v, idx) => {
                                        const meta = EVENT_LABELS[v.event_type] || { label: v.event_type, color: '#888' };
                                        const isSelected = v.id === selectedId;
                                        const prev = versionList[idx + 1];
                                        const chips = computeChangeSummary(
                                            safeParseSnap(v.snapshot),
                                            prev ? safeParseSnap(prev.snapshot) : null
                                        );

                                        return (
                                            <li
                                                key={v.id}
                                                style={{
                                                    ...styles.item,
                                                    background:  isSelected ? '#1e2d4a' : 'transparent',
                                                    borderLeft:  `3px solid ${isSelected ? '#3b82f6' : 'transparent'}`,
                                                }}
                                                onClick={() => setSelectedId(v.id)}
                                            >
                                                <div style={styles.itemTop}>
                                                    <span style={{ ...styles.badge, background: meta.color }}>
                                                        {meta.label}
                                                    </span>
                                                </div>
                                                <div style={styles.itemMeta}>
                                                    <span style={styles.timestamp}>{formatDate(v.created_at)}</span>
                                                    {v.user_name && (
                                                        <span style={styles.author}>by {v.user_name}</span>
                                                    )}
                                                </div>
                                                {chips && (
                                                    <div style={styles.chipRow}>
                                                        {chips.map((chip, i) => (
                                                            <span
                                                                key={i}
                                                                style={{ ...styles.chip, ...CHIP_STYLES[chip.type] }}
                                                            >
                                                                {chip.label}
                                                            </span>
                                                        ))}
                                                    </div>
                                                )}
                                            </li>
                                        );
                                    })}
                                </ul>
                            )}
                        </div>

                        {/* ── Right: snapshot preview ── */}
                        <div style={styles.previewPanel}>
                            {selectedVersion ? (
                                <SnapshotPreview
                                    version={selectedVersion}
                                    onRestore={setRestoreTarget}
                                    restoring={restoring}
                                />
                            ) : (
                                <div style={styles.center}>Select a version to preview.</div>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {restoreTarget && (
                <RestoreConfirmDialog
                    version={restoreTarget}
                    loading={restoring}
                    onConfirm={() => handleConfirmRestore(restoreTarget)}
                    onCancel={() => setRestoreTarget(null)}
                />
            )}
        </>
    );
}

// ─── Styles ──────────────────────────────────────────────────────────────────

const styles = {
    dialog: {
        background: '#12122a',
        borderRadius: 10,
        width: 860,
        maxWidth: '90vw',
        height: 600,
        maxHeight: '90vh',
        display: 'flex',
        flexDirection: 'column',
        boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
        border: '1px solid #2a2a4a',
        overflow: 'hidden',
    },
    header: {
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '16px 20px',
        borderBottom: '1px solid #2a2a4a',
        flexShrink: 0,
    },
    title: { fontWeight: 600, fontSize: 15, color: '#e0e0e0' },
    closeBtn: {
        background: 'none', border: 'none', color: '#888', cursor: 'pointer',
        fontSize: 16, padding: '0 4px', lineHeight: 1,
    },
    body: {
        flex: 1,
        display: 'flex',
        overflow: 'hidden',
    },

    // ── Left panel ──
    listPanel: {
        width: 240,
        borderRight: '1px solid #2a2a4a',
        overflowY: 'auto',
        flexShrink: 0,
    },
    list: { listStyle: 'none', margin: 0, padding: '8px 0' },
    item: {
        padding: '10px 14px',
        cursor: 'pointer',
        display: 'flex', flexDirection: 'column', gap: 4,
        transition: 'background 0.1s',
    },
    itemTop: { display: 'flex', alignItems: 'center', gap: 6 },
    badge: {
        fontSize: 10, fontWeight: 700, color: '#fff',
        padding: '2px 7px', borderRadius: 10,
    },
    itemMeta: { display: 'flex', flexDirection: 'column', gap: 1 },
    timestamp: { fontSize: 11, color: '#888' },
    author:    { fontSize: 10, color: '#666' },
    chipRow:   { display: 'flex', flexWrap: 'wrap', gap: 3, marginTop: 2 },
    chip: {
        fontSize: 10, fontWeight: 600,
        padding: '1px 6px', borderRadius: 10,
        letterSpacing: '0.02em',
    },

    // ── Right panel ──
    previewPanel: {
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        background: '#0e0e22',
    },
    preview: {
        flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden',
    },
    previewHeader: {
        display: 'flex', alignItems: 'center', gap: 10,
        padding: '14px 20px 12px',
        flexShrink: 0,
    },
    previewTimestamp: { fontSize: 12, color: '#aaa' },
    previewAuthor:    { fontSize: 12, color: '#666' },
    previewDivider:   { height: 1, background: '#2a2a4a', flexShrink: 0 },
    previewBody: {
        flex: 1, overflowY: 'auto',
        padding: '16px 20px',
        display: 'flex', flexDirection: 'column', gap: 18,
    },
    previewSection: {},
    previewLabel: {
        fontSize: 10, fontWeight: 700, color: '#555',
        textTransform: 'uppercase', letterSpacing: '0.06em',
        marginBottom: 5,
    },
    previewValue: { fontSize: 13, color: '#d0d0d0', lineHeight: 1.6 },
    previewEmpty: { padding: 24, color: '#666', fontSize: 13 },
    previewFooter: {
        padding: '12px 20px',
        borderTop: '1px solid #2a2a4a',
        display: 'flex', justifyContent: 'flex-end',
        flexShrink: 0,
    },
    restoreBtn: {
        background: '#f59e0b', border: 'none', color: '#000',
        padding: '8px 18px', borderRadius: 6, cursor: 'pointer',
        fontSize: 13, fontWeight: 600,
    },

    stepList: { margin: 0, paddingLeft: 20, display: 'flex', flexDirection: 'column', gap: 10 },
    stepItem: { fontSize: 13, color: '#d0d0d0', lineHeight: 1.5 },
    stepText: { marginBottom: 2 },
    stepExpected: { fontSize: 11, color: '#666', paddingLeft: 4 },

    categoryBadge: {
        fontSize: 11, fontWeight: 600,
        padding: '2px 8px', borderRadius: 10,
        background: '#2a1f6a', color: '#a78bfa',
        border: '1px solid #3730a3',
    },
    cvRow: { display: 'flex', alignItems: 'baseline', gap: 8 },
    cvLabel: { fontSize: 11, color: '#888', minWidth: 90, flexShrink: 0 },
    cvValue: { fontSize: 13, color: '#d0d0d0' },

    // shared
    center: { padding: 32, textAlign: 'center', color: '#555', fontSize: 13 },
    errorBox: {
        display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10,
        padding: 32, color: '#f87171', fontSize: 13, textAlign: 'center',
    },
    retryBtn: {
        background: 'none', border: '1px solid #f87171', color: '#f87171',
        padding: '4px 12px', borderRadius: 4, cursor: 'pointer', fontSize: 12,
    },
};
