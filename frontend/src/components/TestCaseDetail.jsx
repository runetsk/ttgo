import React, { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import StepsEditor from './StepsEditor';
import RichTextField from './RichTextField';
import HistorySidebar from './HistorySidebar';
import RequirementLinkPanel from './RequirementLinkPanel';
import QTestSyncPanel from './QTestSyncPanel';
import { updateTest, getCustomFields, getTest, getCategories, getFolderTree, listTestExecutions, qtest, versions as versionsApi, requirements as requirementsApi } from '../api';
import { useAbortController } from '../hooks/useAbortController';

function formatRelative(iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (!d.getTime() || d.getFullYear() < 2) return '';
    const diffMs = Date.now() - d.getTime();
    const sec = Math.round(diffMs / 1000);
    if (sec < 60) return `${sec}s ago`;
    const min = Math.round(sec / 60);
    if (min < 60) return `${min}m ago`;
    const hr = Math.round(min / 60);
    if (hr < 24) return `${hr}h ago`;
    const day = Math.round(hr / 24);
    if (day < 30) return `${day}d ago`;
    return d.toLocaleDateString();
}

function parseCustomValue(rawValue) {
    if (rawValue === null || rawValue === undefined) return "";
    if (typeof rawValue !== 'string') return rawValue;
    try {
        return JSON.parse(rawValue);
    } catch {
        return rawValue;
    }
}

function parseFieldOptions(rawOptions) {
    if (!rawOptions) return [];
    if (Array.isArray(rawOptions)) return rawOptions;
    if (typeof rawOptions === 'string') {
        try {
            const parsed = JSON.parse(rawOptions);
            return Array.isArray(parsed) ? parsed : [];
        } catch {
            return [];
        }
    }
    return [];
}

export default function TestCaseDetail({ test: initialTest, onClose, onUpdate, onTestLoad, folderMissing = false, inlinePane = false, paneWidth, onPaneResizeStart }) {
    const { testId } = useParams();
    const navigate = useNavigate();

    const [test, setTest] = useState(initialTest || null);
    const [loading, setLoading] = useState(!initialTest && !!testId);

    const [name, setName] = useState("");
    const [description, setDescription] = useState("");
    const [steps, setSteps] = useState([]);
    const [customValues, setCustomValues] = useState([]);
    const [fieldDefs, setFieldDefs] = useState([]);
    const [allCategories, setAllCategories] = useState([]);
    const [assignedCategories, setAssignedCategories] = useState([]);
    const [selectedCategoryId, setSelectedCategoryId] = useState("");

    const [saving, setSaving] = useState(false);
    const [showHistory, setShowHistory] = useState(false);
    const [error, setError] = useState(null);
    const [activeTab, setActiveTab] = useState('steps');
    const [qtestMapping, setQtestMapping] = useState(null);
    const [versionHistory, setVersionHistory] = useState([]);
    const [linkedReqs, setLinkedReqs] = useState([]);

    // Execution history
    const [executions, setExecutions] = useState([]);
    const [execLoading, setExecLoading] = useState(false);

    // Move-to-folder state (shown when folderMissing)
    const [moveFolders, setMoveFolders] = useState([]);
    const [moveFolderId, setMoveFolderId] = useState('');
    const [moving, setMoving] = useState(false);
    const getSignal = useAbortController();

    // Load execution history when test is available
    useEffect(() => {
        const id = test?.id || testId;
        if (!id) return;
        setExecLoading(true);
        listTestExecutions(id)
            .then(data => setExecutions(Array.isArray(data) ? data : []))
            .catch(() => setExecutions([]))
            .finally(() => setExecLoading(false));
    }, [test?.id, testId]);

    // Load QTest mapping so the header badge can show sync status
    useEffect(() => {
        const id = test?.id || testId;
        if (!id || !inlinePane) return;
        setQtestMapping(null);
        qtest.getMapping(id)
            .then(data => setQtestMapping(data?.linked === false ? null : data))
            .catch(() => setQtestMapping(null));
    }, [test?.id, testId, inlinePane]);

    // Load version history (for Activity timeline + footer author) and linked requirements (count)
    useEffect(() => {
        const id = test?.id || testId;
        if (!id || !inlinePane) return;
        setVersionHistory([]);
        setLinkedReqs([]);
        versionsApi.list(id)
            .then(data => setVersionHistory(Array.isArray(data) ? data : []))
            .catch(() => setVersionHistory([]));
        requirementsApi.listByTestCase(id)
            .then(data => setLinkedReqs(Array.isArray(data) ? data : (data?.requirements || [])))
            .catch(() => setLinkedReqs([]));
    }, [test?.id, testId, inlinePane]);

    // Initial Load
    useEffect(() => {
        getCustomFields().then(setFieldDefs);
        getCategories().then(data => setAllCategories(data.categories || []));
    }, []);

    // Load test data if ID is present and no initialTest is provided
    useEffect(() => {
        if (testId && !initialTest) {
            const signal = getSignal();
            // Only show loading spinner on first load, not when switching between tests
            if (!test) setLoading(true);
            getTest(testId, { signal })
                .then(t => {
                    if (signal.aborted) return;
                    setTest(t);
                    if (onTestLoad) onTestLoad(t);
                })
                .catch(err => {
                    if (signal.aborted) return;
                    setError(err.message || "Failed to load test");
                    setTest(null);
                })
                .finally(() => {
                    if (!signal.aborted) setLoading(false);
                });
        } else if (initialTest) {
            setTest(initialTest);
            setLoading(false);
            if (onTestLoad) onTestLoad(initialTest);
        }
    }, [testId, initialTest, onTestLoad, getSignal]);

    // Update state when test data is available
    useEffect(() => {
        if (test) {
            setName(test.name);
            setDescription(test.description || "");
            setSteps(test.steps || []);
            setCustomValues(test.custom_values || []);
            setAssignedCategories(test.categories || []);
            setError(null);
        }
    }, [test]);

    // Fetch folder list when this test is orphaned so user can pick a new folder
    useEffect(() => {
        if (!folderMissing) return;
        function flatten(nodes, depth = 0) {
            const out = [];
            for (const n of nodes) {
                out.push({ id: n.id, name: n.name, depth });
                if (n.children?.length) out.push(...flatten(n.children, depth + 1));
            }
            return out;
        }
        getFolderTree().then(tree => setMoveFolders(flatten(Array.isArray(tree) ? tree : [])));
    }, [folderMissing]);

    const handleMove = () => {
        if (!moveFolderId) return;
        setMoving(true);
        updateTest({ ...test, folder_id: moveFolderId })
            .then(savedTest => {
                setTest(savedTest);
                if (onTestLoad) onTestLoad(savedTest);
            })
            .catch(() => setMoving(false));
    };

    const handleCustomValueChange = (fieldId, value) => {
        const existing = customValues.find(cv => cv.custom_field_id === fieldId);
        if (existing) {
            setCustomValues(customValues.map(cv =>
                cv.custom_field_id === fieldId ? { ...cv, value: JSON.stringify(value) } : cv
            ));
        } else {
            setCustomValues([...customValues, { custom_field_id: fieldId, value: JSON.stringify(value) }]);
        }
    };

    const getFieldValue = (fieldId) => {
        const cv = customValues.find(cv => cv.custom_field_id === fieldId);
        if (!cv) return "";
        return parseCustomValue(cv.value);
    };
    const handleAddCategory = () => {
        if (!selectedCategoryId) return;
        const category = allCategories.find(s => s.id === selectedCategoryId);
        if (category && !assignedCategories.find(s => s.id === category.id)) {
            setAssignedCategories([...assignedCategories, category]);
            setSelectedCategoryId("");
        }
    };

    const handleRemoveCategory = (categoryId) => {
        setAssignedCategories(assignedCategories.filter(s => s.id !== categoryId));
    };

    const handleSave = () => {
        setSaving(true);
        setError(null);

        const updatedTest = {
            ...test,
            name,
            description,
            steps,
            custom_values: customValues,
            categories: assignedCategories
        };

        updateTest(updatedTest)
            .then(savedTest => {
                if (onUpdate) onUpdate(savedTest);
                handleClose();
            })
            .catch(err => {
                setError(err.message || "Failed to save");
                setSaving(false);
            });
    };

    const handleClose = () => {
        if (onClose) {
            onClose(test?.folder_id);
        } else {
            navigate('/');
        }
    };

    if (loading) return <div style={{ padding: 24 }}>Loading...</div>;
    if (!test && !loading) return <div style={{ padding: 24 }}>Test not found</div>;

    const content = (
        <>
            {/* ── Compact header: title input + close ── */}
            <header className="modal-header modal-header--compact">
                <input
                    className="modal-title-input"
                    value={name}
                    onChange={e => setName(e.target.value)}
                    placeholder="Test case name…"
                    data-testid="test-case-name-input"
                />
                <button
                    onClick={() => setShowHistory(true)}
                    className="action-btn"
                    data-testid="history-button"
                    title="View version history"
                    style={{ marginRight: 6, fontSize: 12 }}
                >History</button>
                <button
                    onClick={handleClose}
                    className="modal-close-btn"
                    data-testid="close-modal-button"
                >×</button>
            </header>

            {/* ── Single-row meta bar: categories + custom props ── */}
            <div className="meta-bar">
                {/* Categories */}
                {assignedCategories.map(s => (
                    <span key={s.id} className="meta-chip" data-testid={`assigned-category-${s.id}`}>
                        {s.name}
                        <button
                            className="meta-chip-remove"
                            onClick={() => handleRemoveCategory(s.id)}
                            data-testid={`remove-category-${s.id}`}
                        >×</button>
                    </span>
                ))}
                <div className="meta-category-add">
                    <select
                        className="meta-select"
                        value={selectedCategoryId}
                        onChange={e => setSelectedCategoryId(e.target.value)}
                        data-testid="category-select"
                    >
                        <option value="">+ Category</option>
                        {allCategories
                            .filter(s => !assignedCategories.find(as => as.id === s.id))
                            .map(s => <option key={s.id} value={s.id}>{s.name}</option>)
                        }
                    </select>
                    {selectedCategoryId && (
                        <button className="meta-add-btn" onClick={handleAddCategory} data-testid="add-category-button">
                            Add
                        </button>
                    )}
                </div>

                {/* Divider between categories and custom props */}
                {fieldDefs.length > 0 && <div className="meta-bar-divider" />}

                {/* Custom properties inline */}
                {fieldDefs.map(def => (
                    <div key={def.id} className="meta-field" title={def.name}>
                        <span className="meta-field-label">
                            {def.name.length > 14 ? def.name.slice(0, 14) + '…' : def.name}
                            {def.is_mandatory && <span style={{ color: 'var(--accent-red)' }}>*</span>}
                        </span>
                        {def.type === 'SELECT' ? (
                            <select
                                className="meta-select"
                                value={getFieldValue(def.id)}
                                onChange={e => handleCustomValueChange(def.id, e.target.value)}
                                data-testid={`custom-field-${def.id}`}
                            >
                                <option value="">–</option>
                                {parseFieldOptions(def.options).map(opt => (
                                    <option key={opt} value={opt}>{opt}</option>
                                ))}
                            </select>
                        ) : def.type === 'CHECKBOX' ? (
                            <input
                                type="checkbox"
                                checked={getFieldValue(def.id) === true}
                                onChange={e => handleCustomValueChange(def.id, e.target.checked)}
                                data-testid={`custom-field-${def.id}`}
                            />
                        ) : (
                            <input
                                className="meta-select"
                                style={{ width: 80 }}
                                type={def.type === 'NUMBER' ? 'number' : 'text'}
                                value={getFieldValue(def.id)}
                                onChange={e => handleCustomValueChange(def.id, def.type === 'NUMBER' ? parseFloat(e.target.value) : e.target.value)}
                                data-testid={`custom-field-${def.id}`}
                            />
                        )}
                    </div>
                ))}
            </div>

            {/* ── Scrollable body: description + steps ── */}
            <div className="modal-body" style={{ flex: 1, overflowY: 'auto' }}>
                {folderMissing && (
                    <div style={{ background: 'rgba(245,158,11,0.12)', color: 'var(--warning-color)', border: '1px solid rgba(245,158,11,0.35)', padding: '10px 14px', borderRadius: 6, marginBottom: 16, fontSize: '0.85rem' }}>
                        <div>⚠ This test's folder has been deleted. Move it to an existing folder:</div>
                        <div style={{ display: 'flex', gap: 8, marginTop: 8, alignItems: 'center' }}>
                            <select
                                value={moveFolderId}
                                onChange={e => setMoveFolderId(e.target.value)}
                                className="meta-select"
                                style={{ flex: 1, fontSize: '0.82rem' }}
                            >
                                <option value="">Select a folder…</option>
                                {moveFolders.map(f => (
                                    <option key={f.id} value={f.id}>
                                        {'\u00a0\u00a0'.repeat(f.depth)}{f.name}
                                    </option>
                                ))}
                            </select>
                            <button
                                className="primary-btn"
                                style={{ fontSize: '0.82rem', padding: '4px 12px' }}
                                onClick={handleMove}
                                disabled={!moveFolderId || moving}
                            >
                                {moving ? 'Moving…' : 'Move'}
                            </button>
                        </div>
                    </div>
                )}
                {error && <div className="error-banner" style={{ background: 'rgba(239, 68, 68, 0.1)', color: 'var(--accent-red)', padding: 12, borderRadius: 6, marginBottom: 16 }}>{error}</div>}

                <div data-testid="test-case-description-input" style={{ marginBottom: 16 }}>
                    <RichTextField
                        value={description}
                        onChange={setDescription}
                        placeholder="Add description…"
                    />
                </div>

                <StepsEditor steps={steps} onChange={setSteps} />

                {/* ── Linked Requirements panel (007-req-traceability) ── */}
                {test?.id && <RequirementLinkPanel testCaseId={test.id} />}

                {/* ── Latest Executions ── */}
                <div style={{ marginTop: 20 }}>
                    <h4 style={{ fontSize: '0.82rem', textTransform: 'uppercase', letterSpacing: '0.04em', color: 'var(--text-secondary)', margin: '0 0 8px' }}>
                        Latest Executions
                        {executions.length > 0 && <span style={{ opacity: 0.6, marginLeft: 6 }}>({executions.length})</span>}
                    </h4>
                    {execLoading && <p style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', margin: '4px 0' }}>Loading…</p>}
                    {!execLoading && executions.length === 0 && (
                        <p style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', fontStyle: 'italic', margin: '4px 0' }}>No executions recorded yet.</p>
                    )}
                    {!execLoading && executions.length > 0 && (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            {executions.map(ex => {
                                const statusColor = ex.status === 'PASS' ? 'var(--accent-green)'
                                    : ex.status === 'FAIL' ? 'var(--accent-red)'
                                    : ex.status === 'SKIP' ? 'var(--text-secondary)'
                                    : 'var(--warning-color)';
                                const dur = ex.duration_ms
                                    ? (ex.duration_ms < 1000 ? `${ex.duration_ms}ms` : `${(ex.duration_ms / 1000).toFixed(1)}s`)
                                    : null;
                                return (
                                    <div key={ex.id} style={{
                                        display: 'flex', alignItems: 'center', gap: 10,
                                        padding: '6px 10px', borderRadius: 6,
                                        background: 'rgba(255,255,255,0.03)', border: '1px solid var(--border-color)',
                                    }}>
                                        <span className={`status-badge ${ex.status.toLowerCase()}`} style={{ fontSize: '0.7rem', padding: '2px 8px', flexShrink: 0 }}>
                                            {ex.status}
                                        </span>
                                        <Link
                                            to={`/runs/run/${ex.run_id}`}
                                            style={{ fontSize: '0.82rem', color: 'var(--accent-indigo)', textDecoration: 'none', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                                            onClick={e => e.stopPropagation()}
                                        >
                                            {ex.run_name}
                                        </Link>
                                        {ex.defect_type && ex.defect_type !== '' && (
                                            <span style={{
                                                fontSize: '0.7rem', padding: '1px 6px', borderRadius: 4,
                                                background: ex.defect_type === 'product_bug' ? 'rgba(239,68,68,0.1)' : 'rgba(139,92,246,0.1)',
                                                color: ex.defect_type === 'product_bug' ? '#dc2626' : '#7c3aed',
                                            }}>
                                                {ex.defect_type.replace('_', ' ')}
                                            </span>
                                        )}
                                        {dur && <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', fontFamily: 'monospace', flexShrink: 0 }}>{dur}</span>}
                                        {ex.environment && <span style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', flexShrink: 0 }}>{ex.environment}</span>}
                                        <span style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', flexShrink: 0 }}>
                                            {(() => { const d = new Date(ex.created_at); return d.getFullYear() > 1 ? d.toLocaleDateString() : '—'; })()}
                                        </span>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </div>

                {/* ── QTest Sync panel (013-qtest-sync) ── */}
                {test?.id && <QTestSyncPanel testCaseId={test.id} />}
            </div>

            <footer className="modal-footer">
                <button className="action-btn" onClick={handleClose} disabled={saving} data-testid="cancel-button">
                    Cancel
                </button>
                <button
                    className="primary-btn"
                    onClick={handleSave}
                    disabled={saving}
                    data-testid="save-test-case-button"
                >
                    {saving ? 'Saving...' : 'Save Changes'}
                </button>
            </footer>

            {showHistory && test?.id && (
                <HistorySidebar
                    testCaseId={test.id}
                    onClose={() => setShowHistory(false)}
                    onTestCaseRestored={(updated) => {
                        setTest(updated);
                        setName(updated.name || '');
                        setDescription(updated.description || '');
                        setSteps(updated.steps || []);
                        if (onUpdate) onUpdate(updated);
                    }}
                />
            )}
        </>
    );

    if (inlinePane) {
        const shortId = test?.id ? String(test.id).slice(0, 8) : '';
        const passCount = executions.filter(e => e.status === 'PASS').length;
        const failCount = executions.filter(e => e.status === 'FAIL').length;
        const totalRuns = executions.length;
        const passRate = totalRuns > 0 ? passCount / totalRuns : 0;
        const passColor = passRate > 0.85 ? 'var(--accent-green)' : passRate > 0.7 ? 'var(--warning-color)' : 'var(--accent-red)';
        const last10 = executions.slice(0, 10);
        const rawStatus = qtestMapping?.sync_status || test?.qtest_status || null;
        const statusMap = {
            synced: { label: 'Synced', cls: 'qtest-synced' },
            changes_pending: { label: 'Changes pending', cls: 'qtest-modified' },
            broken: { label: 'Broken', cls: 'qtest-conflict' },
            Synced: { label: 'Synced', cls: 'qtest-synced' },
            Pending: { label: 'Pending', cls: 'qtest-pending' },
            Modified: { label: 'Modified', cls: 'qtest-modified' },
            Conflict: { label: 'Conflict', cls: 'qtest-conflict' },
        };
        const qtestDescriptor = rawStatus ? statusMap[rawStatus] : null;
        const qtestStatus = qtestDescriptor?.label || null;
        const qtestClass = qtestDescriptor?.cls || 'qtest-none';
        const updatedLabel = formatRelative(test?.updated_at);

        const defects = executions.filter(e => e.defect_type && e.defect_type !== '');
        const tabs = [
            { k: 'steps',    l: 'Steps',    n: steps.length },
            { k: 'runs',     l: 'Runs',     n: totalRuns },
            { k: 'defects',  l: 'Defects',  n: defects.length },
            { k: 'reqs',     l: 'Reqs',     n: linkedReqs.length },
            { k: 'activity', l: 'Activity', n: null },
        ];

        const latestVersionWithAuthor = versionHistory.find(v => v.user_name);
        const authorName = latestVersionWithAuthor?.user_name || '';
        const avatarInitial = (authorName || '?').trim()[0]?.toUpperCase() || '?';
        const avatarHash = authorName
            ? authorName.split('').reduce((a, c) => a + c.charCodeAt(0), 0)
            : 0;
        const avatarPalette = [
            ['#6366f1', '#4f46e5'], ['#14b8a6', '#0d9488'], ['#f59e0b', '#d97706'],
            ['#ec4899', '#db2777'], ['#8b5cf6', '#7c3aed'], ['#22c55e', '#16a34a'],
            ['#ef4444', '#dc2626'], ['#0ea5e9', '#0284c7'],
        ];
        const [avatarA, avatarB] = avatarPalette[avatarHash % avatarPalette.length];

        const eventLabels = {
            create:  { label: 'created this test',   tone: '#22c55e' },
            edit:    { label: 'edited this test',    tone: '#3b82f6' },
            restore: { label: 'restored a version',  tone: '#f59e0b' },
        };

        return (
            <aside
                className="detail-pane"
                data-testid="test-detail-pane"
                style={paneWidth ? { width: paneWidth } : undefined}
            >
                {onPaneResizeStart && (
                    <div
                        className="detail-pane-resize-handle"
                        onMouseDown={onPaneResizeStart}
                        role="separator"
                        aria-orientation="vertical"
                        aria-label="Resize detail panel"
                    />
                )}

                <div key={test?.id || 'empty'} className="detail-pane-content-fade">
                <div className="detail-pane-header">
                    <div className="detail-pane-header-top">
                        {shortId && <span className="detail-pane-id-pill">{shortId}</span>}
                        {qtestStatus && (
                            <span className={`detail-pane-qtest-badge ${qtestClass}`}>
                                <span className="detail-pane-qtest-dot" /> {qtestStatus}
                            </span>
                        )}
                        {test?.reverification_flagged && (
                            <span className="detail-pane-reverify-badge" data-testid="reverify-badge" title="This test needs reverification">
                                <span aria-hidden>⚑</span> Needs reverify
                            </span>
                        )}
                        <div style={{ flex: 1 }} />
                        <button
                            onClick={() => setShowHistory(true)}
                            className="detail-pane-icon-btn"
                            data-testid="history-button"
                            title="Version history"
                        >History</button>
                        <button
                            onClick={() => setActiveTab('activity')}
                            className="detail-pane-icon-btn detail-pane-more-btn"
                            data-testid="more-button"
                            title="More"
                            aria-label="More actions"
                        >⋮</button>
                        <button
                            onClick={handleClose}
                            className="detail-pane-icon-btn detail-pane-close-btn"
                            data-testid="close-modal-button"
                            title="Close"
                        >×</button>
                    </div>
                    <input
                        className="detail-pane-title-input"
                        value={name}
                        onChange={e => setName(e.target.value)}
                        placeholder="Test case name…"
                        data-testid="test-case-name-input"
                    />
                    <div className="detail-pane-categories">
                        {assignedCategories.map(s => (
                            <span key={s.id} className="detail-pane-chip" data-testid={`assigned-category-${s.id}`}>
                                {s.name}
                                <button
                                    className="detail-pane-chip-remove"
                                    onClick={() => handleRemoveCategory(s.id)}
                                    data-testid={`remove-category-${s.id}`}
                                    aria-label={`Remove ${s.name}`}
                                >×</button>
                            </span>
                        ))}
                        <select
                            className="detail-pane-category-add"
                            value={selectedCategoryId}
                            onChange={e => {
                                setSelectedCategoryId(e.target.value);
                                if (e.target.value) {
                                    const cat = allCategories.find(c => c.id === e.target.value);
                                    if (cat && !assignedCategories.find(a => a.id === cat.id)) {
                                        setAssignedCategories([...assignedCategories, cat]);
                                    }
                                    setSelectedCategoryId('');
                                }
                            }}
                            data-testid="category-select"
                        >
                            <option value="">+ Category</option>
                            {allCategories
                                .filter(s => !assignedCategories.find(as => as.id === s.id))
                                .map(s => <option key={s.id} value={s.id}>{s.name}</option>)}
                        </select>
                    </div>
                </div>

                <div className="detail-pane-tabs">
                    {tabs.map(t => (
                        <button
                            key={t.k}
                            className={`detail-pane-tab ${activeTab === t.k ? 'active' : ''}`}
                            onClick={() => setActiveTab(t.k)}
                            data-testid={`detail-pane-tab-${t.k}`}
                        >
                            {t.l}{t.n != null && <span className="detail-pane-tab-count"> · {t.n}</span>}
                        </button>
                    ))}
                </div>

                <div className="detail-pane-body">
                    {folderMissing && (
                        <div style={{ background: 'rgba(245,158,11,0.12)', color: 'var(--warning-color)', border: '1px solid rgba(245,158,11,0.35)', padding: '10px 14px', borderRadius: 6, marginBottom: 16, fontSize: '0.85rem' }}>
                            <div>⚠ This test's folder has been deleted. Move it to an existing folder:</div>
                            <div style={{ display: 'flex', gap: 8, marginTop: 8, alignItems: 'center' }}>
                                <select
                                    value={moveFolderId}
                                    onChange={e => setMoveFolderId(e.target.value)}
                                    className="meta-select"
                                    style={{ flex: 1, fontSize: '0.82rem' }}
                                >
                                    <option value="">Select a folder…</option>
                                    {moveFolders.map(f => (
                                        <option key={f.id} value={f.id}>
                                            {'\u00a0\u00a0'.repeat(f.depth)}{f.name}
                                        </option>
                                    ))}
                                </select>
                                <button className="primary-btn" style={{ fontSize: '0.82rem', padding: '4px 12px' }} onClick={handleMove} disabled={!moveFolderId || moving}>
                                    {moving ? 'Moving…' : 'Move'}
                                </button>
                            </div>
                        </div>
                    )}
                    {error && <div className="error-banner" style={{ background: 'rgba(239, 68, 68, 0.1)', color: 'var(--accent-red)', padding: 12, borderRadius: 6, marginBottom: 16 }}>{error}</div>}

                    {activeTab === 'steps' && (
                        <>
                            <div className="detail-pane-section-label">Description</div>
                            <div data-testid="test-case-description-input" style={{ marginBottom: 16 }}>
                                <RichTextField
                                    value={description}
                                    onChange={setDescription}
                                    placeholder="Add description…"
                                />
                            </div>
                            <StepsEditor steps={steps} onChange={setSteps} />

                            {fieldDefs.length > 0 && (
                                <div className="detail-pane-custom-card">
                                    <div className="detail-pane-section-label" style={{ marginBottom: 8 }}>Custom fields</div>
                                    <div className="detail-pane-custom-grid">
                                        {fieldDefs.map(def => (
                                            <React.Fragment key={def.id}>
                                                <span className="detail-pane-custom-key">
                                                    {def.name}
                                                    {def.is_mandatory && <span style={{ color: 'var(--accent-red)' }}>*</span>}
                                                </span>
                                                {def.type === 'SELECT' ? (
                                                    <select
                                                        className="detail-pane-custom-input"
                                                        value={getFieldValue(def.id)}
                                                        onChange={e => handleCustomValueChange(def.id, e.target.value)}
                                                        data-testid={`custom-field-${def.id}`}
                                                    >
                                                        <option value="">–</option>
                                                        {parseFieldOptions(def.options).map(opt => (
                                                            <option key={opt} value={opt}>{opt}</option>
                                                        ))}
                                                    </select>
                                                ) : def.type === 'CHECKBOX' ? (
                                                    <input
                                                        type="checkbox"
                                                        checked={getFieldValue(def.id) === true}
                                                        onChange={e => handleCustomValueChange(def.id, e.target.checked)}
                                                        data-testid={`custom-field-${def.id}`}
                                                    />
                                                ) : (
                                                    <input
                                                        className="detail-pane-custom-input"
                                                        type={def.type === 'NUMBER' ? 'number' : 'text'}
                                                        value={getFieldValue(def.id)}
                                                        onChange={e => handleCustomValueChange(def.id, def.type === 'NUMBER' ? parseFloat(e.target.value) : e.target.value)}
                                                        data-testid={`custom-field-${def.id}`}
                                                    />
                                                )}
                                            </React.Fragment>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {test?.id && <div style={{ marginTop: 16 }}><QTestSyncPanel testCaseId={test.id} /></div>}
                        </>
                    )}

                    {activeTab === 'runs' && (
                        <>
                            <div className="detail-pane-runs-summary">
                                <div>
                                    <div className="detail-pane-section-label" style={{ marginBottom: 2 }}>Pass rate</div>
                                    <div className="detail-pane-runs-rate" style={{ color: passColor }}>
                                        {totalRuns === 0 ? '—' : `${Math.round(passRate * 100)}%`}
                                    </div>
                                </div>
                                <div style={{ flex: 1 }}>
                                    <div className="detail-pane-section-label" style={{ marginBottom: 4 }}>Last 10</div>
                                    {last10.length === 0 ? (
                                        <div style={{ fontSize: 11, color: 'var(--text-secondary)', fontStyle: 'italic' }}>No runs yet</div>
                                    ) : (
                                        <div className="detail-pane-runs-bar">
                                            {last10.map((r, i) => {
                                                const bg = r.status === 'PASS' ? '#22c55e' : r.status === 'FAIL' ? '#ef4444' : '#555';
                                                return <div key={r.id || i} className="detail-pane-runs-cell" style={{ background: bg, opacity: r.status === 'PASS' ? 0.78 : 0.9 }} title={`${r.status}${r.run_name ? ` · ${r.run_name}` : ''}`} />;
                                            })}
                                        </div>
                                    )}
                                </div>
                                {totalRuns > 0 && (
                                    <div style={{ textAlign: 'right' }}>
                                        <div style={{ fontSize: 10, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.08em', fontWeight: 700 }}>P/F</div>
                                        <div style={{ fontSize: 12, fontVariantNumeric: 'tabular-nums', color: 'var(--text-primary)' }}>
                                            <span style={{ color: 'var(--accent-green)' }}>{passCount}</span>
                                            <span style={{ color: 'var(--text-secondary)' }}> / </span>
                                            <span style={{ color: 'var(--accent-red)' }}>{failCount}</span>
                                        </div>
                                    </div>
                                )}
                            </div>

                            {execLoading && <p style={{ fontSize: 12, color: 'var(--text-secondary)' }}>Loading…</p>}
                            {!execLoading && totalRuns === 0 && (
                                <p style={{ fontSize: 12, color: 'var(--text-secondary)', fontStyle: 'italic' }}>No executions recorded yet.</p>
                            )}
                            {!execLoading && totalRuns > 0 && (
                                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                                    {executions.map(ex => {
                                        const dur = ex.duration_ms
                                            ? (ex.duration_ms < 1000 ? `${ex.duration_ms}ms` : `${(ex.duration_ms / 1000).toFixed(1)}s`)
                                            : null;
                                        return (
                                            <div key={ex.id} className="detail-pane-exec-row">
                                                <span className={`detail-pane-exec-status status-${ex.status.toLowerCase()}`}>{ex.status}</span>
                                                <Link
                                                    to={`/runs/run/${ex.run_id}`}
                                                    className="detail-pane-exec-link"
                                                    onClick={e => e.stopPropagation()}
                                                >
                                                    {ex.run_name}
                                                </Link>
                                                {dur && <span className="detail-pane-exec-meta mono">{dur}</span>}
                                                {ex.environment && <span className="detail-pane-exec-meta">{ex.environment}</span>}
                                            </div>
                                        );
                                    })}
                                </div>
                            )}
                        </>
                    )}

                    {activeTab === 'defects' && (
                        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                            {defects.length === 0 ? (
                                <p style={{ fontSize: 12, color: 'var(--text-secondary)', fontStyle: 'italic', margin: 0 }}>
                                    No linked defects.
                                </p>
                            ) : defects.map(d => {
                                const kind = d.defect_type === 'product_bug' ? { label: 'BUG', cls: 'severity-p1' } :
                                             d.defect_type === 'test_bug'    ? { label: 'TEST', cls: 'severity-p3' } :
                                                                               { label: (d.defect_type || '').toUpperCase() || 'OTHER', cls: 'severity-p2' };
                                return (
                                    <div key={d.id} className="detail-pane-defect-row">
                                        <span className={`detail-pane-defect-severity ${kind.cls}`}>{kind.label}</span>
                                        <Link to={`/runs/run/${d.run_id}`} className="detail-pane-exec-link">{d.run_name}</Link>
                                        <span className="detail-pane-exec-meta">{(d.defect_type || '').replace('_', ' ')}</span>
                                    </div>
                                );
                            })}
                        </div>
                    )}

                    {activeTab === 'reqs' && test?.id && <RequirementLinkPanel testCaseId={test.id} />}
                    {activeTab === 'reqs' && !test?.id && <p style={{ fontSize: 12, color: 'var(--text-secondary)' }}>No test loaded.</p>}

                    {activeTab === 'activity' && (
                        <div className="detail-pane-activity-list">
                            {versionHistory.length === 0 && (
                                <p style={{ fontSize: 12, color: 'var(--text-secondary)', fontStyle: 'italic', margin: 0 }}>
                                    No history yet.
                                </p>
                            )}
                            {versionHistory.map(v => {
                                const ev = eventLabels[v.event_type] || { label: v.event_type || 'changed this test', tone: '#7a7a80' };
                                const who = v.user_name || 'Someone';
                                const whoHash = who.split('').reduce((a, c) => a + c.charCodeAt(0), 0);
                                const [ga, gb] = avatarPalette[whoHash % avatarPalette.length];
                                return (
                                    <div key={v.id} className="detail-pane-activity-row">
                                        <span
                                            className="detail-pane-avatar"
                                            style={{ background: `linear-gradient(135deg, ${ga}, ${gb})` }}
                                            aria-hidden
                                        >
                                            {who[0].toUpperCase()}
                                        </span>
                                        <div style={{ flex: 1, minWidth: 0 }}>
                                            <div style={{ fontSize: 12, color: 'var(--text-primary)' }}>
                                                <span style={{ fontWeight: 600 }}>{who}</span>{' '}
                                                <span style={{ color: 'var(--text-secondary)' }}>{ev.label}</span>
                                            </div>
                                            <div style={{ fontSize: 11, color: 'var(--text-secondary)', marginTop: 2 }}>
                                                {formatRelative(v.created_at) || new Date(v.created_at).toLocaleString()}
                                            </div>
                                        </div>
                                        <span
                                            className="detail-pane-activity-dot"
                                            style={{ background: ev.tone }}
                                            aria-hidden
                                        />
                                    </div>
                                );
                            })}
                            <button
                                className="action-btn"
                                onClick={() => setShowHistory(true)}
                                style={{ alignSelf: 'flex-start', marginTop: 4 }}
                            >
                                View full history →
                            </button>
                        </div>
                    )}
                </div>

                <footer className="detail-pane-footer">
                    <div className="detail-pane-footer-meta">
                        {authorName && (
                            <span
                                className="detail-pane-avatar detail-pane-avatar-sm"
                                style={{ background: `linear-gradient(135deg, ${avatarA}, ${avatarB})` }}
                                aria-hidden
                            >{avatarInitial}</span>
                        )}
                        <span>
                            {authorName && <span style={{ color: 'var(--text-primary)', fontWeight: 500 }}>{authorName}</span>}
                            {authorName && updatedLabel ? ' · ' : ''}
                            {updatedLabel ? `updated ${updatedLabel}` : ''}
                        </span>
                    </div>
                    <div className="detail-pane-footer-actions">
                        <button className="action-btn" onClick={handleClose} disabled={saving} data-testid="cancel-button">Cancel</button>
                        <button
                            className="primary-btn"
                            onClick={handleSave}
                            disabled={saving}
                            data-testid="save-test-case-button"
                        >
                            {saving ? 'Saving…' : 'Save changes'}
                        </button>
                    </div>
                </footer>
                </div>

                {showHistory && test?.id && (
                    <HistorySidebar
                        testCaseId={test.id}
                        onClose={() => setShowHistory(false)}
                        onTestCaseRestored={(updated) => {
                            setTest(updated);
                            setName(updated.name || '');
                            setDescription(updated.description || '');
                            setSteps(updated.steps || []);
                            if (onUpdate) onUpdate(updated);
                        }}
                    />
                )}
            </aside>
        );
    }

    if (initialTest) {
        return (
            <div className="modal-overlay" onClick={handleClose}>
                <div
                    className="modal-content"
                    onClick={e => e.stopPropagation()}
                    style={{ width: '800px', maxWidth: '90vw', maxHeight: '90vh', display: 'flex', flexDirection: 'column' }}
                >
                    {content}
                </div>
            </div>
        );
    }

    return (
        <div className="glass-panel" style={{ flex: 1, marginTop: 24, display: 'flex', flexDirection: 'column', height: '100%', padding: 0, overflow: 'hidden' }}>
            <div style={{ display: 'flex', flexDirection: 'column', height: '100%', padding: 24 }}>
                {content}
            </div>
        </div>
    );
}
