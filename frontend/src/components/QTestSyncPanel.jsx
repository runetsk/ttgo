import React, { useState, useEffect, useCallback } from 'react';
import { qtest } from '../api';
import { toast } from '../toast';

const statusColors = {
    synced: { bg: 'rgba(52,211,153,0.12)', border: 'rgba(52,211,153,0.4)', text: '#34d399', label: 'Synced' },
    changes_pending: { bg: 'rgba(251,191,36,0.12)', border: 'rgba(251,191,36,0.4)', text: '#fbbf24', label: 'Changes Pending' },
    broken: { bg: 'rgba(248,113,113,0.12)', border: 'rgba(248,113,113,0.4)', text: '#f87171', label: 'Broken' },
};

function flattenModules(modules, depth = 0) {
    if (!modules) return [];
    return modules.flatMap(m => [
        { id: m.id, name: m.name, depth, hasChildren: !!(m.children && m.children.length > 0) },
        ...flattenModules(m.children, depth + 1),
    ]);
}

export default function QTestSyncPanel({ testCaseId }) {
    const [enabled, setEnabled] = useState(null);
    const [mapping, setMapping] = useState(null);
    const [loading, setLoading] = useState(true);
    const [syncing, setSyncing] = useState(false);
    const [unlinking, setUnlinking] = useState(false);
    const [enabledProjects, setEnabledProjects] = useState([]);

    // Upload form state
    const [showUploadForm, setShowUploadForm] = useState(false);
    const [uploadProjectId, setUploadProjectId] = useState('');
    const [uploadModuleId, setUploadModuleId] = useState('');
    const [modules, setModules] = useState([]);
    const [loadingModules, setLoadingModules] = useState(false);
    const [uploading, setUploading] = useState(false);

    useEffect(() => {
        qtest.getConfig()
            .then(cfg => setEnabled(!!cfg?.enabled))
            .catch(() => setEnabled(false));
        qtest.listEnabledProjects()
            .then(p => setEnabledProjects(p || []))
            .catch(() => {});
    }, []);

    const loadMapping = useCallback(() => {
        if (enabled === false) { setLoading(false); return; }
        setLoading(true);
        qtest.getMapping(testCaseId)
            .then(data => setMapping(data?.linked === false ? null : data))
            .catch(() => setMapping(null))
            .finally(() => setLoading(false));
    }, [testCaseId, enabled]);

    useEffect(() => { if (enabled) loadMapping(); }, [enabled, loadMapping]);

    const handleSync = () => {
        setSyncing(true);
        qtest.sync([testCaseId])
            .then(result => {
                const item = result.items?.[0];
                if (item?.status === 'success') {
                    toast.success('Synced to QTest');
                } else if (item?.status === 'rate_limited') {
                    toast.error('QTest rate limit reached — try again later');
                } else {
                    toast.error(item?.error || 'Sync failed');
                }
                loadMapping();
            })
            .catch(err => toast.error(err.response?.data?.error || 'Sync failed'))
            .finally(() => setSyncing(false));
    };

    const handleUnlink = () => {
        if (!window.confirm('Unlink this test case from QTest? The QTest copy will not be deleted.')) return;
        setUnlinking(true);
        qtest.unlinkMapping(testCaseId)
            .then(() => {
                toast.success('Unlinked from QTest');
                setMapping(null);
            })
            .catch(err => toast.error(err.response?.data?.error || 'Unlink failed'))
            .finally(() => setUnlinking(false));
    };

    // Upload form handlers
    const handleStartUpload = () => {
        setShowUploadForm(true);
        setUploadModuleId('');
        setModules([]);
        if (enabledProjects.length > 0) {
            const def = enabledProjects.find(p => p.is_default) || enabledProjects[0];
            setUploadProjectId(String(def.project_id));
            loadModulesForProject(def.project_id);
        }
    };

    const loadModulesForProject = (projectId) => {
        if (!projectId) return;
        setLoadingModules(true);
        setModules([]);
        setUploadModuleId('');
        qtest.listModules(parseInt(projectId))
            .then(mods => setModules(mods || []))
            .catch(() => toast.error('Failed to load modules'))
            .finally(() => setLoadingModules(false));
    };

    const handleProjectChange = (val) => {
        setUploadProjectId(val);
        loadModulesForProject(parseInt(val));
    };

    const handleDoUpload = () => {
        if (!uploadProjectId || !uploadModuleId) return;
        setUploading(true);
        qtest.upload([testCaseId], parseInt(uploadModuleId), 'skip', parseInt(uploadProjectId))
            .then(result => {
                const item = result.items?.[0];
                if (item?.status === 'success') {
                    toast.success('Uploaded to QTest');
                } else if (item?.status === 'skipped') {
                    toast.info?.('Already linked — skipped') || toast.success('Already linked');
                } else {
                    toast.error(item?.error || 'Upload failed');
                }
                setShowUploadForm(false);
                loadMapping();
            })
            .catch(err => toast.error(err.response?.data?.error || 'Upload failed'))
            .finally(() => setUploading(false));
    };

    if (!enabled || loading) return null;

    // Find project name from mapping
    const projectName = mapping?.qtest_project_id
        ? (enabledProjects.find(p => p.project_id === mapping.qtest_project_id)?.project_name || null)
        : null;

    const flatModules = flattenModules(modules);

    // Not linked state
    if (!mapping) {
        return (
            <div style={{
                marginTop: 16, borderRadius: 8,
                border: '1px solid var(--border-color)',
                background: 'var(--bg-secondary)',
                padding: '14px 18px',
            }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>QTest Integration</div>
                    <span style={{
                        fontSize: '0.75rem', padding: '2px 8px', borderRadius: 4,
                        background: 'var(--bg-primary)', color: 'var(--text-secondary)',
                        border: '1px solid var(--border-color)', fontWeight: 600,
                    }}>
                        Not Linked
                    </span>
                </div>

                {!showUploadForm ? (
                    <div style={{ marginTop: 8 }}>
                        <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: 10 }}>
                            This test case has not been uploaded to QTest.
                        </div>
                        {enabledProjects.length > 0 && (
                            <button
                                className="primary-btn"
                                onClick={handleStartUpload}
                                style={{ fontSize: '0.8rem', padding: '6px 14px' }}
                            >
                                ⬆ Upload to QTest
                            </button>
                        )}
                        {enabledProjects.length === 0 && (
                            <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                No projects enabled — configure in Settings → QTest.
                            </div>
                        )}
                    </div>
                ) : (
                    <div style={{ marginTop: 12 }}>
                        {/* Project selector */}
                        {enabledProjects.length > 1 && (
                            <div style={{ marginBottom: 10 }}>
                                <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.04em' }}>Project</label>
                                <select
                                    className="modern-select"
                                    value={uploadProjectId}
                                    onChange={e => handleProjectChange(e.target.value)}
                                    style={{ width: '100%', fontSize: '0.85rem' }}
                                >
                                    {enabledProjects.map(p => (
                                        <option key={p.project_id} value={String(p.project_id)}>
                                            {p.project_name}{p.is_default ? ' (Default)' : ''}
                                        </option>
                                    ))}
                                </select>
                            </div>
                        )}

                        {/* Module selector */}
                        <div style={{ marginBottom: 12 }}>
                            <label style={{ display: 'block', fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 4, textTransform: 'uppercase', letterSpacing: '0.04em' }}>Module</label>
                            {loadingModules ? (
                                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', padding: '6px 0' }}>Loading modules...</div>
                            ) : (
                                <select
                                    className="modern-select"
                                    value={uploadModuleId}
                                    onChange={e => setUploadModuleId(e.target.value)}
                                    style={{ width: '100%', fontSize: '0.85rem' }}
                                >
                                    <option value="">Select a module...</option>
                                    {flatModules.map(m => (
                                        <option key={m.id} value={m.id}>
                                            {'  '.repeat(m.depth)}{m.name}
                                        </option>
                                    ))}
                                </select>
                            )}
                        </div>

                        {/* Actions */}
                        <div style={{ display: 'flex', gap: 8 }}>
                            <button
                                className="primary-btn"
                                onClick={handleDoUpload}
                                disabled={uploading || !uploadModuleId || !uploadProjectId}
                                style={{ fontSize: '0.8rem', padding: '6px 14px', opacity: (!uploadModuleId ? 0.5 : 1) }}
                            >
                                {uploading ? 'Uploading...' : '⬆ Upload'}
                            </button>
                            <button
                                className="action-btn"
                                onClick={() => setShowUploadForm(false)}
                                disabled={uploading}
                                style={{ fontSize: '0.8rem', padding: '6px 14px' }}
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                )}
            </div>
        );
    }

    const status = statusColors[mapping.sync_status] || statusColors.synced;

    return (
        <div style={{ marginTop: 16, borderRadius: 8, border: `1px solid ${status.border}`, background: status.bg, padding: '14px 18px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>QTest Integration</div>
                <span style={{
                    fontSize: '0.75rem', padding: '2px 8px', borderRadius: 4,
                    background: status.bg, color: status.text, border: `1px solid ${status.border}`,
                    fontWeight: 600,
                }}>
                    {status.label}
                </span>
            </div>

            <div style={{ fontSize: '0.85rem', marginBottom: 6 }}>
                <strong>QTest ID:</strong>{' '}
                {mapping.qtest_url ? (
                    <a href={mapping.qtest_url} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent-indigo)' }}>
                        {mapping.qtest_test_case_pid || `#${mapping.qtest_test_case_id}`}
                    </a>
                ) : (
                    <span>{mapping.qtest_test_case_pid || `#${mapping.qtest_test_case_id}`}</span>
                )}
            </div>

            {projectName && (
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: 6 }}>
                    Project: <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{projectName}</span>
                </div>
            )}

            {mapping.last_synced_at && (
                <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: 6 }}>
                    Last synced: {new Date(mapping.last_synced_at).toLocaleString()}
                </div>
            )}

            {mapping.error_message && (
                <div style={{ fontSize: '0.8rem', color: '#f87171', marginBottom: 8 }}>
                    {mapping.error_message}
                </div>
            )}

            <div style={{ display: 'flex', gap: 8, marginTop: 10 }}>
                <button
                    className="action-btn"
                    onClick={handleSync}
                    disabled={syncing || mapping.sync_status === 'synced'}
                    style={{ fontSize: '0.8rem', padding: '4px 12px' }}
                >
                    {syncing ? 'Syncing...' : 'Sync to QTest'}
                </button>
                <button
                    className="action-btn"
                    onClick={handleUnlink}
                    disabled={unlinking}
                    style={{ fontSize: '0.8rem', padding: '4px 12px', color: 'var(--accent-red)' }}
                >
                    {unlinking ? 'Unlinking...' : 'Unlink'}
                </button>
            </div>
        </div>
    );
}
