import React, { useState, useEffect, useCallback } from 'react';
import { useAIGeneration } from '../contexts/AIGenerationContext';
import { requirements as requirementsApi } from '../api';
import AIGenReviewPanel from './AIGenReviewPanel';
import FolderTreeSelect from './FolderTreeSelect';

const FORMAT_LABELS = {
    json: 'JSON',
    csv: 'CSV',
    markdown_table: 'Markdown Table',
    numbered_list: 'Numbered List',
    ai: 'AI-Parsed',
};

function DebugRow({ label, value, mono, strong, highlight }) {
    return (
        <>
            <span style={{ fontSize: '0.76rem', color: 'var(--text-secondary)', whiteSpace: 'nowrap' }}>{label}</span>
            <span style={{
                fontSize: '0.76rem',
                color: highlight ? '#fbbf24' : 'var(--text-primary)',
                fontFamily: mono ? 'monospace' : 'inherit',
                fontWeight: strong ? 700 : 400,
            }}>{value}</span>
        </>
    );
}

export default function AIImportReview({ onAccepted, onBack }) {
    const ai = useAIGeneration();
    const [folderId, setFolderId] = useState(ai.selectedFolderId || '');
    const [requirementId, setRequirementId] = useState(ai.activeRequirement?.id || '');
    const [allReqs, setAllReqs] = useState([]);
    const [showUnparseable, setShowUnparseable] = useState(false);
    const [debugExpanded, setDebugExpanded] = useState(false);
    const [contextExpanded, setContextExpanded] = useState(false);

    // Use persistent review state from context
    const drafts = ai.importReviewDrafts;
    const acceptedIds = ai.importAcceptedIds;
    const discardedIds = ai.importDiscardedIds;

    // Use folders from context (eagerly loaded on mount)
    const folders = ai.folders || [];

    // Initialize review drafts from parsed import drafts (only on first mount or new parse)
    useEffect(() => {
        if (ai.importDrafts.length > 0 && drafts.length === 0) {
            ai.setImportReviewDrafts([...ai.importDrafts]);
        }
    }, [ai.importDrafts]); // eslint-disable-line react-hooks/exhaustive-deps

    // Auto-select first folder if none selected
    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time default fill once context-loaded folders become available; folderId remains user-editable afterward via FolderTreeSelect
        if (!folderId && folders.length > 0) setFolderId(folders[0].id);
    }, [folders.length]); // eslint-disable-line react-hooks/exhaustive-deps

    useEffect(() => {
        requirementsApi.list()
            .then(data => setAllReqs(Array.isArray(data) ? data : []))
            .catch(() => setAllReqs([]));
    }, []);

    // Drafts with discarded removed
    const visibleDrafts = drafts.filter(d => !discardedIds.has(d.temp_id));
    const readyDrafts = visibleDrafts.filter(d => acceptedIds.has(d.temp_id));
    const dupeSet = new Set((ai.importDuplicateNames || []).map(n => n.toLowerCase()));

    // ── Accept / Discard handlers for AIGenReviewPanel (use context methods) ──
    const handleAcceptDraft = useCallback((draft) => {
        ai.importAcceptDraft(draft);
    }, [ai]);

    const handleDiscardDraft = useCallback((tempId) => {
        ai.importDiscardDraft(tempId);
    }, [ai]);

    const handleAcceptAll = useCallback(() => {
        const pending = visibleDrafts.filter(d => !acceptedIds.has(d.temp_id));
        pending.forEach(d => ai.importAcceptDraft(d));
    }, [visibleDrafts, acceptedIds, ai]);

    const handleDiscardAll = useCallback(() => {
        const pendingIds = visibleDrafts.filter(d => !acceptedIds.has(d.temp_id)).map(d => d.temp_id);
        pendingIds.forEach(id => ai.importDiscardDraft(id));
    }, [visibleDrafts, acceptedIds, ai]);

    const handleAcceptGroup = useCallback((groupDrafts) => {
        groupDrafts.forEach(d => ai.importAcceptDraft(d));
    }, [ai]);

    const handleDiscardGroup = useCallback((ids) => {
        ids.forEach(id => ai.importDiscardDraft(id));
    }, [ai]);

    const handleEdit = useCallback((tempId, changes) => {
        ai.importEditDraft(tempId, changes);
    }, [ai]);

    // ── Final import ──
    const handleImport = async () => {
        if (readyDrafts.length === 0 || !folderId) return;
        try {
            await ai.acceptImport(folderId, requirementId, readyDrafts);
            ai.clearImport();
            onAccepted?.();
        } catch {
            // Error handled by context
        }
    };

    const importDisabled = readyDrafts.length === 0 || !folderId || ai.importAccepting;

    return (
        <div style={styles.root}>
            {/* ── Header ── */}
            <div style={styles.header}>
                <div style={styles.headerLeft}>
                    <button onClick={onBack} style={styles.backBtn} className="ir-back-btn" title="Back to paste">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/>
                        </svg>
                    </button>
                    <div>
                        <h3 style={styles.title}>Review Imported Test Cases</h3>
                        <div style={styles.titleMeta}>
                            <span style={{
                                ...styles.formatTag,
                                ...(ai.importFormat === 'ai' ? styles.formatTagAI : {}),
                            }}>
                                {ai.importFormat === 'ai' && (
                                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                        <path d="M12 2a4 4 0 0 1 4 4v1a3 3 0 0 1 3 3v1a2 2 0 0 1-2 2h-1l-1 5H9l-1-5H7a2 2 0 0 1-2-2v-1a3 3 0 0 1 3-3V6a4 4 0 0 1 4-4z"/>
                                    </svg>
                                )}
                                {FORMAT_LABELS[ai.importFormat] || ai.importFormat}
                            </span>
                            <span style={styles.countMeta}>
                                {visibleDrafts.length} parsed
                            </span>
                            {acceptedIds.size > 0 && (
                                <span style={styles.acceptedTag}>
                                    <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                        <polyline points="20 6 9 17 4 12"/>
                                    </svg>
                                    {acceptedIds.size} ready to import
                                </span>
                            )}
                            {discardedIds.size > 0 && (
                                <span style={styles.discardedTag}>
                                    {discardedIds.size} discarded
                                </span>
                            )}
                        </div>
                    </div>
                </div>
            </div>

            {/* ── LLM Feedback panel (shown when AI-parsed) ── */}
            {ai.importDebug && (
                <div style={styles.debugCard}>
                    <button
                        onClick={() => setDebugExpanded(e => !e)}
                        style={styles.debugHeader}
                        className="ir-debug-toggle"
                    >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ color: '#818cf8' }}>
                                <circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/>
                            </svg>
                            <span style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', letterSpacing: '0.04em', textTransform: 'uppercase' }}>LLM Feedback</span>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                            <span style={styles.debugChip}>
                                ⏱ {ai.importDebug.duration_ms >= 1000
                                    ? `${(ai.importDebug.duration_ms / 1000).toFixed(1)}s`
                                    : `${ai.importDebug.duration_ms}ms`}
                            </span>
                            {ai.importDebug.usage && (
                                <span style={styles.debugChip}>
                                    🪙 {ai.importDebug.usage.total_tokens?.toLocaleString() ?? '—'} tokens
                                </span>
                            )}
                            <svg
                                width="13" height="13" viewBox="0 0 24 24" fill="none"
                                stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                                style={{ color: 'var(--text-secondary)', transition: 'transform 0.2s', transform: debugExpanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
                            >
                                <polyline points="6 9 12 15 18 9"/>
                            </svg>
                        </div>
                    </button>

                    {debugExpanded && (
                        <div style={styles.debugBody}>
                            <div style={styles.debugGrid}>
                                <DebugRow label="Duration"
                                    value={ai.importDebug.duration_ms >= 1000
                                        ? `${(ai.importDebug.duration_ms / 1000).toFixed(2)}s`
                                        : `${ai.importDebug.duration_ms}ms`} />
                                <DebugRow label="Model (reported)" value={ai.importDebug.model || '—'} mono />
                                <DebugRow label="Provider" value={`${ai.importDebug.provider_label} (${ai.importDebug.provider_type})`} />
                                <DebugRow label="Finish reason" value={ai.importDebug.finish_reason || '—'} />
                                <DebugRow label="Max tokens budget" value={ai.importDebug.max_tokens_budget?.toLocaleString() ?? '—'} />
                                {ai.importDebug.usage ? (
                                    <>
                                        <DebugRow label="Prompt tokens" value={ai.importDebug.usage.prompt_tokens?.toLocaleString() ?? '—'} />
                                        <DebugRow label="Completion tokens" value={ai.importDebug.usage.completion_tokens?.toLocaleString() ?? '—'} />
                                        <DebugRow label="Total tokens" value={ai.importDebug.usage.total_tokens?.toLocaleString() ?? '—'} strong />
                                    </>
                                ) : (
                                    <DebugRow label="Token usage" value="Not reported by provider" />
                                )}
                            </div>

                            {/* Expandable Request Context */}
                            {ai.importDebug.request_context && (
                                <div style={{ marginTop: 12, borderTop: '1px solid var(--border-color)', paddingTop: 10 }}>
                                    <button
                                        onClick={() => setContextExpanded(e => !e)}
                                        style={{
                                            background: 'none', border: 'none', cursor: 'pointer', padding: 0,
                                            display: 'flex', alignItems: 'center', gap: 6, width: '100%',
                                        }}
                                    >
                                        <svg
                                            width="11" height="11" viewBox="0 0 24 24" fill="none"
                                            stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"
                                            style={{ color: 'var(--text-secondary)', transition: 'transform 0.2s', transform: contextExpanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
                                        >
                                            <polyline points="6 9 12 15 18 9"/>
                                        </svg>
                                        <span style={{ fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', letterSpacing: '0.04em', textTransform: 'uppercase' }}>
                                            Full Request Context
                                        </span>
                                    </button>
                                    {contextExpanded && (
                                        <pre style={{
                                            marginTop: 8,
                                            padding: 14,
                                            background: 'rgba(0,0,0,0.25)',
                                            borderRadius: 6,
                                            fontSize: '0.78rem',
                                            lineHeight: 1.6,
                                            color: 'var(--text-secondary)',
                                            whiteSpace: 'pre-wrap',
                                            wordBreak: 'break-word',
                                            maxHeight: 400,
                                            overflow: 'auto',
                                            border: '1px solid var(--border-color)',
                                        }}>
                                            {ai.importDebug.request_context}
                                        </pre>
                                    )}
                                </div>
                            )}
                        </div>
                    )}
                </div>
            )}

            {/* ── Destination section (matches AI Generate page) ── */}
            <div style={styles.section}>
                <div style={styles.sectionLabel}>
                    <span style={styles.sectionDot} />
                    Destination
                </div>
                <div style={styles.twoCol}>
                    <div>
                        <label style={styles.fieldLabel}>Save Tests To Folder</label>
                        <FolderTreeSelect
                            folders={folders}
                            value={folderId}
                            onChange={setFolderId}
                            disabled={ai.importAccepting}
                        />
                    </div>
                    <div>
                        <label style={styles.fieldLabel}>
                            Requirement
                            <span style={styles.optionalTag}>optional</span>
                        </label>
                        <div style={styles.selectWrap}>
                            <svg style={styles.selectIcon} width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/>
                                <line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>
                            </svg>
                            <select
                                className="modern-select"
                                style={styles.select}
                                value={requirementId}
                                onChange={(e) => setRequirementId(e.target.value)}
                                disabled={ai.importAccepting}
                            >
                                <option value="">None</option>
                                {allReqs.map(r => (
                                    <option key={r.id} value={r.id}>{r.identifier} — {r.title}</option>
                                ))}
                            </select>
                        </div>
                    </div>
                </div>
            </div>

            {/* Truncation warning */}
            {ai.importTruncated && (
                <div style={styles.warning}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
                        <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
                    </svg>
                    <span>Content contained {ai.importTotalFound} test cases — only the first 50 are shown (limit per import session).</span>
                </div>
            )}

            {/* Duplicate names warning */}
            {dupeSet.size > 0 && (
                <div style={styles.dupeWarning}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
                        <circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
                    </svg>
                    <span>{dupeSet.size} test case name{dupeSet.size !== 1 ? 's match' : ' matches'} existing cases in the target folder.</span>
                </div>
            )}

            {/* ── Reuse AIGenReviewPanel ── */}
            <div style={styles.reviewWrap}>
                <AIGenReviewPanel
                    drafts={visibleDrafts}
                    acceptedIds={acceptedIds}
                    onAccept={handleAcceptDraft}
                    onDiscard={handleDiscardDraft}
                    onAcceptAll={handleAcceptAll}
                    onDiscardAll={handleDiscardAll}
                    onAcceptGroup={handleAcceptGroup}
                    onDiscardGroup={handleDiscardGroup}
                    onEdit={handleEdit}
                />
            </div>

            {/* ── Unparseable items ── */}
            {ai.importUnparseable?.length > 0 && (
                <div style={styles.unparseableSection}>
                    <button
                        onClick={() => setShowUnparseable(!showUnparseable)}
                        style={styles.unparseableToggle}
                        className="ir-unparseable-toggle"
                    >
                        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round"
                            style={{ transition: 'transform 0.15s', transform: showUnparseable ? 'rotate(90deg)' : 'rotate(0deg)' }}>
                            <polyline points="9 18 15 12 9 6"/>
                        </svg>
                        <span>{ai.importUnparseable.length} unparseable item{ai.importUnparseable.length !== 1 ? 's' : ''}</span>
                    </button>
                    {showUnparseable && (
                        <div style={styles.unparseableList}>
                            {ai.importUnparseable.map((item, i) => (
                                <div key={i} style={styles.unparseableItem}>
                                    <span style={styles.unparseableLine}>Line {item.line_number}</span>
                                    <code style={styles.unparseableText}>{item.raw_text}</code>
                                    <span style={styles.unparseableReason}>{item.reason}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}

            {/* ── Import button ── */}
            <div style={styles.importRow}>
                <button
                    onClick={handleImport}
                    disabled={importDisabled}
                    className="ir-import-btn"
                    style={{
                        ...styles.importBtn,
                        ...(importDisabled ? styles.importBtnDisabled : {}),
                    }}
                >
                    {ai.importAccepting ? (
                        <>
                            <span style={styles.spinner} />
                            Importing…
                        </>
                    ) : (
                        <>
                            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="20 6 9 17 4 12"/>
                            </svg>
                            Import {readyDrafts.length} Test Case{readyDrafts.length !== 1 ? 's' : ''}
                        </>
                    )}
                </button>
                {!folderId && (
                    <div style={styles.folderHint}>
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
                        </svg>
                        Select a target folder to enable import
                    </div>
                )}
            </div>

            {/* Import error */}
            {ai.importError && (
                <div style={styles.error}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
                        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <span>{ai.importError}</span>
                </div>
            )}

            <style>{`
                @keyframes spin { to { transform: rotate(360deg); } }
                .ir-back-btn:hover {
                    color: var(--text-primary) !important;
                    border-color: rgba(99,102,241,0.4) !important;
                    background: rgba(99,102,241,0.04) !important;
                }
                .ir-import-btn:not(:disabled):hover {
                    filter: brightness(1.1);
                    transform: translateY(-1px);
                    box-shadow: 0 6px 22px rgba(99,102,241,0.35) !important;
                }
                .ir-import-btn:not(:disabled):active {
                    transform: translateY(0);
                }
                .ir-unparseable-toggle:hover {
                    color: var(--text-primary) !important;
                }
                .ir-debug-toggle:hover {
                    background: rgba(99,102,241,0.05) !important;
                }
            `}</style>
        </div>
    );
}

const styles = {
    root: {
        display: 'flex', flexDirection: 'column', gap: 16,
        padding: '20px 0', maxWidth: 950,
    },
    header: {
        display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start',
        flexWrap: 'wrap', gap: 12,
        paddingBottom: 16,
        borderBottom: '1px solid var(--border-color)',
    },
    headerLeft: { display: 'flex', alignItems: 'flex-start', gap: 12 },
    backBtn: {
        display: 'flex', alignItems: 'center', padding: 7, marginTop: 1,
        background: 'none', border: '1px solid var(--border-color)',
        borderRadius: 7, color: 'var(--text-secondary)', cursor: 'pointer',
        transition: 'all 0.15s',
    },
    title: {
        margin: 0, fontSize: '1.05rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.01em',
    },
    titleMeta: {
        display: 'flex', alignItems: 'center', gap: 8, marginTop: 5,
        flexWrap: 'wrap',
    },
    formatTag: {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        fontSize: '0.7rem', padding: '2px 8px', borderRadius: 4, fontWeight: 600,
        background: 'rgba(99,102,241,0.08)', color: '#818cf8',
        border: '1px solid rgba(99,102,241,0.15)',
    },
    formatTagAI: {
        background: 'rgba(168,85,247,0.1)', color: '#a855f7',
        border: '1px solid rgba(168,85,247,0.2)',
    },
    countMeta: {
        fontSize: '0.78rem', color: 'var(--text-secondary)',
    },
    acceptedTag: {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        fontSize: '0.7rem', padding: '2px 8px', borderRadius: 4, fontWeight: 600,
        background: 'rgba(34,197,94,0.08)', color: '#22c55e',
        border: '1px solid rgba(34,197,94,0.15)',
    },
    discardedTag: {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        fontSize: '0.7rem', padding: '2px 8px', borderRadius: 4, fontWeight: 600,
        background: 'rgba(239,68,68,0.06)', color: '#f87171',
        border: '1px solid rgba(239,68,68,0.12)',
    },
    warning: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '10px 14px', borderRadius: 8, fontSize: '0.82rem',
        background: 'rgba(245,158,11,0.08)', color: '#f59e0b',
        border: '1px solid rgba(245,158,11,0.2)', lineHeight: 1.4,
    },
    dupeWarning: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '10px 14px', borderRadius: 8, fontSize: '0.82rem',
        background: 'rgba(251,146,60,0.08)', color: '#fb923c',
        border: '1px solid rgba(251,146,60,0.2)', lineHeight: 1.4,
    },
    reviewWrap: {},
    // ── Section styles (matching AIGeneratePage) ──
    section: {
        display: 'flex', flexDirection: 'column', gap: 8,
        padding: '14px 16px', borderRadius: 10,
        background: 'rgba(99,102,241,0.02)',
        border: '1px solid var(--border-color)',
    },
    sectionLabel: {
        display: 'flex', alignItems: 'center', gap: 6,
        fontSize: '0.75rem', fontWeight: 700,
        color: 'var(--text-secondary)',
        textTransform: 'uppercase', letterSpacing: '0.06em',
        marginBottom: 2,
    },
    sectionDot: {
        width: 5, height: 5, borderRadius: '50%',
        background: 'linear-gradient(135deg, #6366f1, #14b8a6)',
        flexShrink: 0,
    },
    twoCol: {
        display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12,
    },
    fieldLabel: {
        display: 'flex', alignItems: 'center', gap: 5,
        marginBottom: 6, fontSize: '0.78rem',
        color: 'var(--text-secondary)', fontWeight: 500,
    },
    selectWrap: { position: 'relative' },
    selectIcon: {
        position: 'absolute', left: 10, top: '50%',
        transform: 'translateY(-50%)',
        color: 'var(--text-secondary)', pointerEvents: 'none', zIndex: 1,
    },
    select: { width: '100%', paddingLeft: 30 },
    optionalTag: {
        fontSize: '0.7rem', fontWeight: 500, color: 'var(--text-secondary)',
        background: 'rgba(255,255,255,0.05)', border: '1px solid var(--border-color)',
        padding: '0 6px', borderRadius: 4, textTransform: 'none',
        letterSpacing: '0.01em', marginLeft: 4,
    },
    // ── Unparseable ──
    unparseableSection: {
        borderTop: '1px solid var(--border-color)', paddingTop: 12,
    },
    unparseableToggle: {
        display: 'flex', alignItems: 'center', gap: 6,
        background: 'none', border: 'none', color: 'var(--text-secondary)',
        fontSize: '0.8rem', cursor: 'pointer', padding: '4px 0',
        fontFamily: 'inherit', fontWeight: 500, transition: 'color 0.15s',
    },
    unparseableList: {
        marginTop: 8, display: 'flex', flexDirection: 'column', gap: 4,
    },
    unparseableItem: {
        display: 'flex', gap: 8, alignItems: 'baseline', fontSize: '0.78rem',
        padding: '6px 10px', borderRadius: 6,
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
    },
    unparseableLine: { color: 'var(--text-secondary)', fontWeight: 700, whiteSpace: 'nowrap', fontSize: '0.72rem' },
    unparseableText: {
        color: 'var(--text-primary)', fontSize: '0.73rem',
        wordBreak: 'break-all', flex: 1,
        fontFamily: "'SF Mono', 'Fira Code', monospace",
    },
    unparseableReason: { color: 'var(--text-secondary)', fontStyle: 'italic', whiteSpace: 'nowrap', fontSize: '0.72rem' },
    // ── Import button ──
    importRow: {
        display: 'flex', alignItems: 'center', gap: 14,
        flexWrap: 'wrap',
    },
    importBtn: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '11px 26px', borderRadius: 9, fontSize: '0.88rem',
        background: 'linear-gradient(135deg, #4338ca 0%, #6366f1 100%)',
        color: '#fff', border: 'none',
        cursor: 'pointer', fontWeight: 700, whiteSpace: 'nowrap',
        fontFamily: 'inherit',
        boxShadow: '0 3px 14px rgba(99,102,241,0.3)',
        transition: 'all 0.18s cubic-bezier(0.16, 1, 0.3, 1)',
        letterSpacing: '0.01em',
    },
    importBtnDisabled: {
        opacity: 0.4, cursor: 'not-allowed',
        boxShadow: 'none',
    },
    folderHint: {
        display: 'flex', alignItems: 'center', gap: 6,
        fontSize: '0.78rem', color: 'var(--text-secondary)',
        fontStyle: 'italic',
    },
    spinner: {
        display: 'inline-block', width: 14, height: 14,
        border: '2px solid rgba(255,255,255,0.3)', borderTopColor: '#fff',
        borderRadius: '50%', animation: 'spin 0.6s linear infinite',
    },
    error: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '10px 14px', borderRadius: 8, fontSize: '0.82rem',
        background: 'rgba(239,68,68,0.07)', color: '#f87171',
        border: '1px solid rgba(239,68,68,0.2)', lineHeight: 1.4,
    },
    // ── Debug / LLM Feedback styles (matching AIGeneratePage) ──
    debugCard: {
        borderRadius: 10,
        border: '1px solid rgba(99,102,241,0.15)',
        background: 'rgba(99,102,241,0.03)',
        overflow: 'hidden',
    },
    debugHeader: {
        width: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 12,
        padding: '9px 14px',
        background: 'none',
        border: 'none',
        cursor: 'pointer',
        fontFamily: 'inherit',
        textAlign: 'left',
        transition: 'background 0.15s',
    },
    debugChip: {
        fontSize: '0.75rem',
        color: 'var(--text-secondary)',
        background: 'rgba(255,255,255,0.04)',
        border: '1px solid rgba(255,255,255,0.07)',
        padding: '1px 8px',
        borderRadius: 4,
        fontFamily: 'monospace',
        whiteSpace: 'nowrap',
    },
    debugBody: {
        padding: '10px 14px 14px',
        borderTop: '1px solid rgba(99,102,241,0.1)',
    },
    debugGrid: {
        display: 'grid',
        gridTemplateColumns: 'max-content 1fr',
        rowGap: 7,
        columnGap: 16,
    },
};
