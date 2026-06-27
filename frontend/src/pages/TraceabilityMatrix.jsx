import React, { useState, useEffect, useRef, useMemo } from 'react';
import { Link } from 'react-router-dom';
import { traceability, requirements as reqApi, getTests } from '../api';

/**
 * TraceabilityMatrix — /traceability
 *
 * Shows every requirement alongside its linked test cases in a unified,
 * scannable table. Features:
 *   - 4-card coverage summary + progress bar
 *   - Live search + "Show gaps only" filter
 *   - Inline add / remove links without navigating away
 */
export default function TraceabilityMatrix() {
    const [matrix, setMatrix]     = useState(null);
    const [loading, setLoading]   = useState(true);
    const [allTests, setAllTests] = useState([]);

    const [gapsOnly, setGapsOnly]       = useState(false);
    const [searchTerm, setSearchTerm]   = useState('');

    // Per-row dropdown: reqId → { open, search, saving }
    const [rowDropdown, setRowDropdown] = useState({});
    const dropdownRefs = useRef({});

    // ── Data loading ──────────────────────────────────────────────────────────

    const load = () => {
        setLoading(true);
        Promise.all([traceability.getMatrix(), getTests([], undefined, { view: 'list' })])
            .then(([mat, tests]) => { setMatrix(mat); setAllTests(tests || []); })
            .catch(() => {})
            .finally(() => setLoading(false));
    };

    useEffect(() => { load(); }, []);

    // Close dropdown on outside click
    useEffect(() => {
        const handler = (e) => {
            setRowDropdown(prev => {
                const next = { ...prev };
                Object.keys(next).forEach(id => {
                    const ref = dropdownRefs.current[id];
                    if (ref && !ref.contains(e.target)) next[id] = { ...next[id], open: false };
                });
                return next;
            });
        };
        document.addEventListener('mousedown', handler);
        return () => document.removeEventListener('mousedown', handler);
    }, []);

    // ── Link management ───────────────────────────────────────────────────────

    const openDropdown = (reqId) =>
        setRowDropdown(prev => ({ ...prev, [reqId]: { open: true, search: '', saving: false, ...(prev[reqId] || {}) } }));

    const setDropSearch = (reqId, val) =>
        setRowDropdown(prev => ({ ...prev, [reqId]: { ...prev[reqId], search: val } }));

    const handleAddLink = (row, testCase) => {
        setRowDropdown(prev => ({ ...prev, [row.requirement_id]: { ...prev[row.requirement_id], saving: true } }));
        reqApi.createLink(row.requirement_id, testCase.id)
            .then(() => {
                setMatrix(prev => {
                    const rows = prev.rows.map(r => {
                        if (r.requirement_id !== row.requirement_id) return r;
                        const updated = {
                            ...r,
                            linked_test_cases: [...r.linked_test_cases, { test_case_id: testCase.id, test_case_name: testCase.name }],
                            covered: true,
                        };
                        return updated;
                    });
                    return { rows, summary: recalcSummary(rows) };
                });
                setRowDropdown(prev => ({ ...prev, [row.requirement_id]: { open: false, search: '', saving: false } }));
            })
            .catch(() => {});
    };

    const handleRemoveLink = (row, testCaseId) => {
        reqApi.deleteLink(row.requirement_id, testCaseId)
            .then(() => {
                setMatrix(prev => {
                    const rows = prev.rows.map(r => {
                        if (r.requirement_id !== row.requirement_id) return r;
                        const newLinked = r.linked_test_cases.filter(tc => tc.test_case_id !== testCaseId);
                        return { ...r, linked_test_cases: newLinked, covered: newLinked.length > 0 };
                    });
                    return { rows, summary: recalcSummary(rows) };
                });
            })
            .catch(() => {});
    };

    // ── Derived state ─────────────────────────────────────────────────────────

    const visibleRows = useMemo(() => {
        if (!matrix?.rows) return [];
        const q = searchTerm.trim().toLowerCase();
        return matrix.rows.filter(row => {
            if (gapsOnly && row.covered) return false;
            if (q && !row.identifier.toLowerCase().includes(q) && !row.title.toLowerCase().includes(q) && !(row.description || '').toLowerCase().includes(q)) return false;
            return true;
        });
    }, [matrix, gapsOnly, searchTerm]);

    // ── Loading / empty ───────────────────────────────────────────────────────

    if (loading) {
        return <div style={{ padding: 32, textAlign: 'center', color: 'var(--text-secondary)' }}>Loading traceability matrix…</div>;
    }
    if (!matrix) return null;

    const { rows, summary } = matrix;
    const pct = summary.percentage ?? 0;
    const pctColor = pct >= 80 ? 'var(--accent-green, #34d399)' : pct >= 50 ? '#facc15' : 'var(--accent-red, #f87171)';

    return (
        <div style={{ padding: '24px 28px', maxWidth: 1100, margin: '0 auto' }}>

            {/* ── Header ── */}
            <div style={{ marginBottom: 20 }}>
                <h2 style={{ margin: '0 0 4px', fontSize: '1.4rem' }}>Traceability Matrix</h2>
                <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                    Requirements coverage at a glance — see which are covered by test cases and which are gaps.
                </p>
            </div>

            {/* ── Coverage summary cards ── */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 12, marginBottom: 12 }}>
                <SummaryCard label="Total" value={summary.total} color="var(--text-primary)" />
                <SummaryCard label="Covered" value={summary.covered} color="var(--accent-green, #34d399)" />
                <SummaryCard label="Gaps" value={summary.uncovered} color="var(--accent-red, #f87171)" />
                <SummaryCard
                    label="Coverage"
                    value={`${pct}%`}
                    color={pctColor}
                    subtitle={summary.uncovered === 0 ? 'All covered ✓' : `${summary.uncovered} uncovered`}
                />
            </div>

            {/* ── Progress bar ── */}
            <div style={{ marginBottom: 20 }}>
                <div style={{ height: 6, borderRadius: 3, background: 'rgba(255,255,255,0.08)', overflow: 'hidden' }}>
                    <div style={{ width: `${pct}%`, height: '100%', background: pctColor, transition: 'width 0.5s ease', borderRadius: 3 }} />
                </div>
            </div>

            {/* ── Filter bar ── */}
            <div style={{ display: 'flex', gap: 10, marginBottom: 14, alignItems: 'center', flexWrap: 'wrap' }}>
                <div style={{ position: 'relative', flex: '1 1 240px', maxWidth: 360 }}>
                    <span style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text-secondary)', fontSize: '0.88rem', pointerEvents: 'none' }}>🔍</span>
                    <input
                        className="modern-input"
                        style={{ width: '100%', paddingLeft: 30, boxSizing: 'border-box', fontSize: '0.875rem' }}
                        placeholder="Search requirements…"
                        value={searchTerm}
                        onChange={e => setSearchTerm(e.target.value)}
                    />
                </div>
                <button
                    className={gapsOnly ? 'primary-btn' : 'action-btn'}
                    style={{ fontSize: '0.875rem', flexShrink: 0 }}
                    onClick={() => setGapsOnly(v => !v)}
                >
                    {gapsOnly ? '⚠ Gaps Only ✕' : '⚠ Show Gaps Only'}
                </button>
                {(searchTerm || gapsOnly) && (
                    <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginLeft: 2 }}>
                        {visibleRows.length} of {rows.length} shown
                    </span>
                )}
            </div>

            {/* ── Empty states ── */}
            {rows.length === 0 && (
                <div className="glass-panel" style={{ padding: 56, textAlign: 'center' }}>
                    <div style={{ fontSize: '2.5rem', marginBottom: 10 }}>📋</div>
                    <div style={{ fontWeight: 600, marginBottom: 6 }}>No requirements yet</div>
                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginBottom: 20 }}>
                        Add requirements on the{' '}
                        <Link to="/requirements" style={{ color: 'var(--accent-purple, #a78bfa)' }}>Requirements</Link>{' '}
                        page, then link test cases here or from the test case detail view.
                    </div>
                </div>
            )}
            {rows.length > 0 && gapsOnly && summary.uncovered === 0 && (
                <div className="glass-panel" style={{ padding: 32, textAlign: 'center', color: 'var(--accent-green, #34d399)', fontWeight: 600 }}>
                    ✓ All {summary.total} requirements are covered — no gaps!
                </div>
            )}
            {rows.length > 0 && visibleRows.length === 0 && !(gapsOnly && summary.uncovered === 0) && (
                <div className="glass-panel" style={{ padding: 32, textAlign: 'center', color: 'var(--text-secondary)' }}>
                    No requirements match <strong>"{searchTerm}"</strong>
                </div>
            )}

            {/* ── Matrix table ── */}
            {visibleRows.length > 0 && (
                <div className="glass-panel" style={{ overflow: 'visible' }}>
                    {/* Table header */}
                    <div style={{ display: 'grid', gridTemplateColumns: '260px 1fr 110px', gap: 0, borderBottom: '1px solid var(--border-color)', padding: '8px 16px' }}>
                        <div style={thStyle}>Requirement</div>
                        <div style={thStyle}>Linked Test Cases</div>
                        <div style={{ ...thStyle, textAlign: 'right' }}>Link</div>
                    </div>

                    {visibleRows.map((row, idx) => {
                        const dd = rowDropdown[row.requirement_id] || {};
                        const ddSearch = (dd.search || '').toLowerCase();
                        const linkedIds = new Set(row.linked_test_cases.map(tc => tc.test_case_id));
                        const filteredTests = allTests.filter(t =>
                            !linkedIds.has(t.id) &&
                            (ddSearch === '' || t.name.toLowerCase().includes(ddSearch))
                        );
                        const isLast = idx === visibleRows.length - 1;

                        return (
                            <div
                                key={row.requirement_id}
                                style={{
                                    display: 'grid',
                                    gridTemplateColumns: '260px 1fr 110px',
                                    gap: 0,
                                    borderBottom: isLast ? 'none' : '1px solid rgba(255,255,255,0.04)',
                                    borderLeft: `3px solid ${row.covered ? 'var(--accent-green, #34d399)' : 'var(--accent-red, #f87171)'}`,
                                    transition: 'background 0.15s',
                                }}
                                onMouseEnter={e => e.currentTarget.style.background = 'rgba(255,255,255,0.025)'}
                                onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                            >
                                {/* ── Requirement info ── */}
                                <div style={{ padding: '14px 16px', borderRight: '1px solid rgba(255,255,255,0.04)' }}>
                                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: row.title ? 4 : 0 }}>
                                        <span style={{
                                            fontWeight: 700, fontSize: '0.8rem', letterSpacing: '0.02em',
                                            color: 'var(--accent-purple, #a78bfa)',
                                            background: 'rgba(167,139,250,0.1)',
                                            padding: '2px 7px', borderRadius: 4, flexShrink: 0,
                                        }}>
                                            {row.identifier}
                                        </span>
                                        <span style={{
                                            fontSize: '0.65rem', fontWeight: 700, padding: '1px 7px', borderRadius: 20, flexShrink: 0,
                                            background: row.covered ? 'rgba(52,211,153,0.12)' : 'rgba(248,113,113,0.12)',
                                            color: row.covered ? 'var(--accent-green, #34d399)' : 'var(--accent-red, #f87171)',
                                            border: `1px solid ${row.covered ? 'rgba(52,211,153,0.3)' : 'rgba(248,113,113,0.3)'}`,
                                        }}>
                                            {row.covered ? '✓' : '✗'}
                                        </span>
                                    </div>
                                    <div style={{ fontWeight: 600, fontSize: '0.88rem', lineHeight: 1.35 }}>{row.title}</div>
                                    {row.description && (
                                        <div style={{
                                            fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: 3, lineHeight: 1.4,
                                            display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden',
                                        }}>
                                            {row.description}
                                        </div>
                                    )}
                                </div>

                                {/* ── Linked test cases ── */}
                                <div style={{ padding: '14px 14px', display: 'flex', flexWrap: 'wrap', gap: 5, alignItems: 'flex-start', alignContent: 'flex-start', borderRight: '1px solid rgba(255,255,255,0.04)' }}>
                                    {row.linked_test_cases.length === 0 ? (
                                        <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', fontStyle: 'italic', alignSelf: 'center' }}>
                                            No test cases linked yet
                                        </span>
                                    ) : (
                                        row.linked_test_cases.map(tc => (
                                            <TestCaseChip
                                                key={tc.test_case_id}
                                                tc={tc}
                                                onRemove={() => handleRemoveLink(row, tc.test_case_id)}
                                            />
                                        ))
                                    )}
                                </div>

                                {/* ── Add link button + dropdown ── */}
                                <div style={{ padding: '14px 12px', display: 'flex', alignItems: 'flex-start', justifyContent: 'flex-end' }}>
                                    <div
                                        style={{ position: 'relative' }}
                                        ref={el => dropdownRefs.current[row.requirement_id] = el}
                                    >
                                        <button
                                            className="action-btn"
                                            style={{ fontSize: '0.78rem', padding: '3px 10px', whiteSpace: 'nowrap' }}
                                            onClick={() => openDropdown(row.requirement_id)}
                                            disabled={dd.saving}
                                            title="Link a test case to this requirement"
                                        >
                                            {dd.saving ? '…' : '+ Add'}
                                        </button>
                                        {dd.open && (
                                            <div style={{
                                                position: 'absolute', top: '100%', right: 0, zIndex: 50, marginTop: 4,
                                                background: 'var(--surface-bg, #1e1e2e)',
                                                border: '1px solid var(--border-color)',
                                                borderRadius: 8, boxShadow: '0 8px 24px rgba(0,0,0,0.5)', width: 300,
                                            }}>
                                                <div style={{ padding: '8px 8px 4px' }}>
                                                    <input
                                                        className="modern-input"
                                                        style={{ width: '100%', fontSize: '0.82rem', boxSizing: 'border-box' }}
                                                        placeholder="Filter test cases…"
                                                        value={dd.search || ''}
                                                        onChange={e => setDropSearch(row.requirement_id, e.target.value)}
                                                        autoFocus
                                                    />
                                                </div>
                                                <div style={{ maxHeight: 220, overflowY: 'auto' }}>
                                                    {filteredTests.length === 0 ? (
                                                        <div style={{ padding: '10px 12px', color: 'var(--text-secondary)', fontSize: '0.82rem', textAlign: 'center' }}>
                                                            {allTests.length === linkedIds.size ? 'All test cases already linked.' : 'No matches.'}
                                                        </div>
                                                    ) : (
                                                        filteredTests.map(t => (
                                                            <div
                                                                key={t.id}
                                                                style={{ padding: '8px 12px', cursor: 'pointer', fontSize: '0.82rem', borderTop: '1px solid rgba(255,255,255,0.04)' }}
                                                                onMouseDown={() => handleAddLink(row, t)}
                                                                onMouseEnter={e => e.currentTarget.style.background = 'rgba(255,255,255,0.07)'}
                                                                onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                                                            >
                                                                <div style={{ fontWeight: 500, lineHeight: 1.3 }}>{t.name}</div>
                                                            </div>
                                                        ))
                                                    )}
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            </div>
                        );
                    })}

                    {/* Footer row count */}
                    {(searchTerm || gapsOnly) && visibleRows.length > 0 && (
                        <div style={{ padding: '8px 16px', borderTop: '1px solid var(--border-color)', fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
                            Showing {visibleRows.length} of {rows.length} requirements
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

// ── Sub-components ────────────────────────────────────────────────────────────

function SummaryCard({ label, value, color, subtitle }) {
    return (
        <div className="glass-panel" style={{ padding: '14px 18px', textAlign: 'center' }}>
            <div style={{ fontSize: '1.6rem', fontWeight: 700, color }}>{value}</div>
            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.06em', marginTop: 2 }}>{label}</div>
            {subtitle && <div style={{ fontSize: '0.7rem', color: 'var(--text-secondary)', marginTop: 3, opacity: 0.7 }}>{subtitle}</div>}
        </div>
    );
}

function TestCaseChip({ tc, onRemove }) {
    const [hovered, setHovered] = useState(false);
    return (
        <span
            style={{
                display: 'inline-flex', alignItems: 'center', gap: 4,
                fontSize: '0.78rem', padding: '3px 6px 3px 8px', borderRadius: 4,
                background: 'rgba(99,102,241,0.12)',
                border: '1px solid rgba(99,102,241,0.3)',
                maxWidth: 220,
            }}
        >
            <Link
                to={`/library/tests/${tc.test_case_id}`}
                style={{
                    color: 'var(--text-primary)', textDecoration: 'none',
                    overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                    maxWidth: 180,
                }}
                title={tc.test_case_name || tc.test_case_id}
            >
                {tc.test_case_name || tc.test_case_id}
            </Link>
            <button
                onClick={onRemove}
                onMouseEnter={() => setHovered(true)}
                onMouseLeave={() => setHovered(false)}
                title="Remove link"
                style={{
                    background: 'none', border: 'none', cursor: 'pointer', padding: '0 1px',
                    color: hovered ? 'var(--accent-red, #f87171)' : 'var(--text-secondary)',
                    fontSize: '0.9rem', lineHeight: 1, flexShrink: 0,
                    transition: 'color 0.15s',
                }}
            >
                ×
            </button>
        </span>
    );
}

function recalcSummary(rows) {
    const covered   = rows.filter(r => r.covered).length;
    const total     = rows.length;
    const uncovered = total - covered;
    const pct       = total > 0 ? Math.round((covered / total) * 1000) / 10 : 0;
    return { total, covered, uncovered, percentage: pct };
}

// ── Styles ────────────────────────────────────────────────────────────────────

const thStyle = {
    fontSize: '0.72rem', fontWeight: 600,
    color: 'var(--text-secondary)',
    textTransform: 'uppercase', letterSpacing: '0.06em',
};
