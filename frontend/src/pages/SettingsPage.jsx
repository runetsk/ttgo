import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { getCustomFields, createCustomField, deleteCustomField, users as usersApi, seed as seedApi, jira as jiraApi, confluence as confApi } from '../api';
import { toast } from '../toast';
import Modal from '../components/Modal';
import TokenSettings from '../components/TokenSettings';
import WebhookSettings from '../components/WebhookSettings';
import IntegrationSettings from '../components/IntegrationSettings';
import AIGenSettings from '../components/AIGenSettings';
import AIFailureAnalysisSettings from '../components/AIFailureAnalysisSettings';
import AIFeaturesToggle from '../components/AIFeaturesToggle';
import { useAuth } from '../contexts/AuthContext';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';
import { BackupsSettings } from './BackupsPage';

export default function SettingsPage() {
    const { user: currentUser } = useAuth();
    const isAdmin = currentUser?.role === 'admin';

    const [fields, setFields] = useState([]);
    const [name, setName] = useState('');
    const [type, setType] = useState('TEXT');
    const [options, setOptions] = useState('');
    const [saving, setSaving] = useState(false);
    const [modal, setModal] = useState(null);
    const location = useLocation();
    const hashTab = (location.hash || '').replace(/^#/, '');
    const [activeTab, setActiveTab] = useState(hashTab || 'custom-fields');

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- pre-existing pattern, unmasked by the loadFields reordering below; out of scope for this task (owned by the set-state-in-effect cleanup)
        if (hashTab && hashTab !== activeTab) setActiveTab(hashTab);
    }, [hashTab]);

    const loadFields = () => {
        // T013: removed [DEBUG] log lines
        getCustomFields().then(setFields).catch(err => console.error('Failed to load fields:', err));
    };

    useEffect(() => {
        loadFields();
    }, []);

    const handleAdd = () => {
        if (!name) return;
        setSaving(true);

        let opts = null;
        if (type === 'SELECT' && options) {
            opts = options.split(',').map(s => s.trim());
        }

        createCustomField(name, type, opts, false)
            .then(() => {
                setName('');
                setOptions('');
                loadFields();
            })
            .finally(() => setSaving(false));
    };

    const handleDelete = (field) => {
        setModal({
            type: 'confirm',
            title: 'Delete Custom Field',
            message: `Are you sure you want to delete "${field.name}"? This will permanently remove this field and all associated data from ALL test cases. This action cannot be undone.`,
            confirmText: 'Delete Forever',
            confirmStyle: 'danger',
            onConfirm: () => {
                deleteCustomField(field.id)
                    .then(() => {
                        setModal(null);
                        loadFields();
                    })
                    .catch(err => {
                        console.error('Deletion failed:', err);
                        toast.error("Failed to delete field: " + (err.response?.data?.error || err.message));
                    });
            }
        });
    };

    const tabGroups = [
        {
            label: 'General',
            tabs: [
                { id: 'custom-fields', label: 'Custom Fields', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg> },
                { id: 'api-tokens', label: 'API Tokens', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg> },
                { id: 'webhooks', label: 'Webhooks', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg> },
            ],
        },
        {
            label: 'Integrations',
            tabs: [
                { id: 'jira', label: 'Jira', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/></svg> },
                { id: 'confluence', label: 'Confluence', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/></svg> },
                { id: 'ai-test-generation', label: 'AI Generation', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg> },
            ],
        },
        ...(isAdmin ? [{
            label: 'Admin',
            tabs: [
                { id: 'backups', label: 'Backups', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> },
                { id: 'users', label: 'Users', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg> },
                { id: 'demo-data', label: 'Demo Data', icon: <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"/><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"/></svg> },
            ],
        }] : []),
    ];

    return (
        <div style={{ display: 'flex', gap: 0, width: '100%', padding: '24px 24px 24px 0', minHeight: 'calc(100vh - 80px)' }}>
            {/* Sidebar nav */}
            <nav style={{
                width: 200, flexShrink: 0,
                padding: '8px 0',
                borderRight: '1px solid var(--border-color)',
                marginRight: 32,
            }}>
                <h2 style={{ margin: '0 0 20px', padding: '0 16px', fontSize: '1.15rem' }}>Settings</h2>
                {tabGroups.map((group, gi) => (
                    <div key={group.label} style={{ marginBottom: gi < tabGroups.length - 1 ? 16 : 0 }}>
                        <div style={{
                            padding: '0 16px', marginBottom: 4,
                            fontSize: '0.68rem', fontWeight: 700, textTransform: 'uppercase',
                            letterSpacing: '0.06em', color: 'var(--text-secondary)', opacity: 0.6,
                        }}>
                            {group.label}
                        </div>
                        {group.tabs.map(tab => {
                            const active = activeTab === tab.id;
                            return (
                                <button
                                    key={tab.id}
                                    onClick={() => {
                                        setActiveTab(tab.id);
                                        if (typeof window !== 'undefined') {
                                            window.history.replaceState(null, '', `#${tab.id}`);
                                        }
                                    }}
                                    className={active ? 'settings-tab-active' : 'settings-tab'}
                                    style={{
                                        display: 'flex', alignItems: 'center', gap: 8,
                                        width: '100%', padding: '7px 16px',
                                        border: 'none', borderRadius: 7,
                                        cursor: 'pointer', fontFamily: 'inherit',
                                        fontSize: '0.83rem',
                                        fontWeight: active ? 600 : 400,
                                        background: active
                                            ? 'linear-gradient(135deg, rgba(99,102,241,0.14) 0%, rgba(99,102,241,0.08) 100%)'
                                            : 'transparent',
                                        color: active ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                                        transition: 'all 0.15s ease',
                                        textAlign: 'left',
                                        margin: '1px 8px 1px 0',
                                        borderLeft: active ? '2px solid var(--accent-indigo)' : '2px solid transparent',
                                    }}
                                >
                                    <span style={{ opacity: active ? 1 : 0.55, display: 'flex', flexShrink: 0 }}>{tab.icon}</span>
                                    {tab.label}
                                </button>
                            );
                        })}
                    </div>
                ))}
            </nav>

            {/* Content area */}
            <div style={{ flex: 1, minWidth: 0, maxWidth: 800, padding: '8px 0' }}>
                <style>{`
                    .settings-tab:hover {
                        background: rgba(255,255,255,0.05) !important;
                        color: var(--text-primary) !important;
                    }
                    .settings-tab:hover span { opacity: 0.8 !important; }
                `}</style>

            {activeTab === 'custom-fields' && (
                <>
                    <div className="glass-panel" style={{ padding: 24, marginBottom: 32 }}>
                        <h3 style={{ marginTop: 0 }}>Add New Field</h3>
                        <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start', flexWrap: 'wrap' }}>
                            <div style={{ flex: 1, minWidth: 200 }}>
                                <label style={{ display: 'block', marginBottom: 4, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Field Name</label>
                                <input className="modern-input" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Priority" style={{ width: '100%' }} />
                            </div>

                            <div style={{ width: 150 }}>
                                <label style={{ display: 'block', marginBottom: 4, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Type</label>
                                <select className="modern-select" value={type} onChange={e => setType(e.target.value)} style={{ width: '100%' }}>
                                    <option value="TEXT">Text</option>
                                    <option value="SELECT">Select Dropdown</option>
                                    <option value="NUMBER">Number</option>
                                    <option value="CHECKBOX">Checkbox</option>
                                    <option value="DATE">Date</option>
                                </select>
                            </div>

                            {type === 'SELECT' && (
                                <div style={{ flex: 2, minWidth: 300 }}>
                                    <label style={{ display: 'block', marginBottom: 4, fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Options (comma separated)</label>
                                    <input className="modern-input" value={options} onChange={e => setOptions(e.target.value)} placeholder="Low, Medium, High" style={{ width: '100%' }} />
                                </div>
                            )}

                            <div style={{ paddingTop: 20 }}>
                                <button className="primary-btn" onClick={handleAdd} disabled={saving || !name}>
                                    {saving ? 'Adding...' : '+ Add Field'}
                                </button>
                            </div>
                        </div>
                    </div>

                    <h3 style={{ marginBottom: 16 }}>Existing Fields</h3>
                    {fields.length === 0 && <div style={{ color: 'var(--text-secondary)' }}>No custom fields defined.</div>}

                    <div style={{ display: 'grid', gap: 12 }}>
                        {fields.map(f => (
                            <div key={f.id} className="glass-panel" style={{ padding: '12px 16px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                                <div>
                                    <div style={{ fontWeight: 600 }}>{f.name}</div>
                                    <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                                        {f.type} {f.type === 'SELECT' && f.options && (() => {
                                            // Backend returns options as an array; tolerate a legacy JSON string too.
                                            const opts = Array.isArray(f.options)
                                                ? f.options
                                                : (() => { try { return JSON.parse(f.options); } catch { return []; } })();
                                            return opts.length ? `(${opts.join(', ')})` : '';
                                        })()}
                                    </div>
                                </div>
                                <button className="action-btn" style={{ color: 'var(--accent-red)' }} onClick={() => handleDelete(f)}>Delete</button>
                            </div>
                        ))}
                    </div>
                </>
            )}

            {activeTab === 'api-tokens' && <TokenSettings />}
            {activeTab === 'webhooks' && <WebhookSettings />}
            {activeTab === 'jira' && (
                <IntegrationSettings
                    provider="jira"
                    providerLabel="Jira"
                    description="Connect to Jira Cloud to auto-populate requirements from Jira tickets. API tokens are stored server-side and masked in responses."
                    apiGetConfig={jiraApi.getConfig}
                    apiUpsertConfig={jiraApi.upsertConfig}
                    extraFields={[
                        { key: 'default_project_key', label: 'Default Project Key', placeholder: 'e.g. PROJ', defaultValue: '' },
                        { key: 'default_issue_type', label: 'Default Issue Type', placeholder: 'e.g. Bug', defaultValue: 'Bug' },
                    ]}
                    tokenHintField="api_token_masked"
                    renderTestConnection={() => <JiraTestConnection />}
                />
            )}
            {activeTab === 'confluence' && (
                <IntegrationSettings
                    provider="confluence"
                    providerLabel="Confluence"
                    description="Connect to Confluence Cloud to import requirements from wiki pages. API tokens are stored server-side and never exposed in responses."
                    apiGetConfig={confApi.getConfig}
                    apiUpsertConfig={confApi.upsertConfig}
                    extraFields={[]}
                    tokenHintField="has_token"
                    renderTestConnection={() => <ConfluenceTestConnection />}
                />
            )}
            {activeTab === 'ai-test-generation' && (
                <>
                    <AIFeaturesToggle isAdmin={isAdmin} />
                    <AIGenSettings />
                    <AIFailureAnalysisSettings isAdmin={isAdmin} />
                </>
            )}
            {activeTab === 'backups' && isAdmin && <BackupsSettings />}
            {activeTab === 'users' && isAdmin && <UserSettings />}
            {activeTab === 'demo-data' && isAdmin && <DemoDataSettings />}

            {modal && (
                <Modal
                    {...modal}
                    onCancel={() => setModal(null)}
                />
            )}
            </div>
        </div>
    );
}

// ─────────────────────────────────────────────
// JiraTestConnection — test connection panel for Jira
// ─────────────────────────────────────────────
function JiraTestConnection() {
    const [testTicketId, setTestTicketId] = useState('');
    const [testResult, setTestResult] = useState(null);
    const [testing, setTesting] = useState(false);

    const handleTest = () => {
        if (!testTicketId.trim()) { toast.error('Enter a ticket ID to test, e.g. PROJ-123'); return; }
        setTesting(true);
        setTestResult(null);
        jiraApi.fetchTicket(testTicketId.trim())
            .then(result => setTestResult(result))
            .catch(err => toast.error(err.response?.data?.error || err.message))
            .finally(() => setTesting(false));
    };

    return (
        <div className="glass-panel" style={{ padding: 20 }}>
            <h4 style={{ marginTop: 0, marginBottom: 10 }}>Test Connection</h4>
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 12 }}>
                <input
                    className="modern-input"
                    style={{ width: 180 }}
                    placeholder="e.g. PROJ-123"
                    value={testTicketId}
                    onChange={e => setTestTicketId(e.target.value)}
                />
                <button className="action-btn" onClick={handleTest} disabled={testing}>
                    {testing ? 'Fetching...' : 'Fetch Ticket'}
                </button>
            </div>
            {testResult && (
                <div style={{
                    padding: '10px 14px',
                    borderRadius: 6,
                    border: `1px solid ${testResult.success ? 'rgba(52,211,153,0.4)' : 'rgba(248,113,113,0.4)'}`,
                    background: testResult.success ? 'rgba(52,211,153,0.08)' : 'rgba(248,113,113,0.08)',
                    fontSize: '0.875rem',
                }}>
                    {testResult.success ? (
                        <>
                            <div style={{ fontWeight: 600, color: 'var(--accent-green, #34d399)', marginBottom: 4 }}>
                                {'✓'} {testResult.identifier}
                            </div>
                            <div><strong>Title:</strong> {testResult.title}</div>
                            {testResult.description && <div style={{ marginTop: 4, color: 'var(--text-secondary)' }}>{testResult.description}</div>}
                        </>
                    ) : (
                        <div style={{ color: 'var(--accent-red, #f87171)' }}>{'✗'} {testResult.error}</div>
                    )}
                </div>
            )}
        </div>
    );
}

// ─────────────────────────────────────────────
// ConfluenceTestConnection — test connection panel for Confluence
// ─────────────────────────────────────────────
function ConfluenceTestConnection() {
    const [testResult, setTestResult] = useState(null);
    const [testing, setTesting] = useState(false);

    const handleTest = () => {
        setTesting(true);
        setTestResult(null);
        confApi.listSpaces(null, 5)
            .then(result => {
                const count = result.spaces?.length ?? 0;
                setTestResult({ success: true, message: `Connected successfully. Found ${count} space${count !== 1 ? 's' : ''}.` });
            })
            .catch(err => {
                setTestResult({ success: false, message: err.response?.data?.error || err.message || 'Connection failed.' });
            })
            .finally(() => setTesting(false));
    };

    return (
        <div className="glass-panel" style={{ padding: 20 }}>
            <h4 style={{ marginTop: 0, marginBottom: 10 }}>Test Connection</h4>
            <div style={{ marginBottom: 12 }}>
                <button className="action-btn" onClick={handleTest} disabled={testing}>
                    {testing ? 'Testing...' : 'Test Connection'}
                </button>
            </div>
            {testResult && (
                <div style={{
                    padding: '10px 14px',
                    borderRadius: 6,
                    border: `1px solid ${testResult.success ? 'rgba(52,211,153,0.4)' : 'rgba(248,113,113,0.4)'}`,
                    background: testResult.success ? 'rgba(52,211,153,0.08)' : 'rgba(248,113,113,0.08)',
                    fontSize: '0.875rem',
                }}>
                    {testResult.success ? (
                        <div style={{ fontWeight: 600, color: 'var(--accent-green, #34d399)' }}>{'✓'} {testResult.message}</div>
                    ) : (
                        <div style={{ color: 'var(--accent-red, #f87171)' }}>{'✗'} {testResult.message}</div>
                    )}
                </div>
            )}
        </div>
    );
}

// ─────────────────────────────────────────────
// UserSettings — admin-only user management tab
// ─────────────────────────────────────────────
function UserSettings() {
    const [userList, setUserList] = useState([]);
    const [loading, setLoading] = useState(true);
    const [showAddModal, setShowAddModal] = useState(false);
    const [resetModal, setResetModal] = useState(null); // { user }
    const [toast, setToast] = useState(null);
    const [showDeleted, setShowDeleted] = useState(false);
    const [confirmDelete, setConfirmDelete] = useState(null); // { user }

    const showToast = (msg, type = 'success') => {
        setToast({ msg, type });
        setTimeout(() => setToast(null), 3000);
    };

    const loadUsers = useCallback(() => {
        setLoading(true);
        usersApi.list(true)
            .then(data => setUserList(data.users || []))
            .catch(() => showToast('Failed to load users', 'error'))
            .finally(() => setLoading(false));
    }, []);

    // 018-websocket-realtime: subscribe to settings updates
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription('settings:*', useCallback((event) => {
        if (event.type === 'user_updated') {
            loadUsers();
        }
    }, [loadUsers]));

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- async load result: loadUsers fetches the user list and wires up the websocket refresh subscription
        loadUsers();
        registerRefresh('settingsUsers', loadUsers);
        return () => unregisterRefresh('settingsUsers');
    }, [loadUsers, registerRefresh, unregisterRefresh]);

    const toggleActive = async (u) => {
        try {
            await usersApi.update(u.id, { active: !u.active });
            showToast(`User ${u.active ? 'deactivated' : 'reactivated'}`);
        } catch (err) {
            showToast(err?.response?.data?.error || 'Update failed', 'error');
        }
    };

    const deleteUser = async (u) => {
        setConfirmDelete(null);
        try {
            await usersApi.delete(u.id);
            showToast(`User ${u.display_name || u.email} deleted`);
        } catch (err) {
            showToast(err?.response?.data?.error || 'Delete failed', 'error');
        }
    };

    const restoreUser = async (u) => {
        try {
            await usersApi.restore(u.id);
            showToast(`User ${u.display_name || u.email} restored`);
        } catch (err) {
            showToast(err?.response?.data?.error || 'Restore failed', 'error');
        }
    };

    const deletedCount = userList.filter(u => u.deleted).length;
    const visibleUsers = userList.filter(u => {
        if (u.deleted && !showDeleted) return false;
        return true;
    });

    return (
        <div>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                <h3 style={{ margin: 0 }}>User Accounts</h3>
                <div style={{ display: 'flex', gap: 10, alignItems: 'center' }}>
                    {deletedCount > 0 && (
                        <button
                            className="action-btn"
                            style={{
                                fontSize: '0.8rem',
                                padding: '4px 10px',
                                opacity: showDeleted ? 1 : 0.7,
                            }}
                            onClick={() => setShowDeleted(v => !v)}
                        >
                            {showDeleted ? 'Hide deleted' : `Show deleted (${deletedCount})`}
                        </button>
                    )}
                    <button className="primary-btn" onClick={() => setShowAddModal(true)}>+ Add User</button>
                </div>
            </div>

            {toast && (
                <div style={{
                    marginBottom: 12,
                    padding: '8px 14px',
                    borderRadius: 6,
                    background: toast.type === 'error' ? 'rgba(239,68,68,0.15)' : 'rgba(34,197,94,0.15)',
                    color: toast.type === 'error' ? 'var(--accent-red, #ef4444)' : 'var(--accent-green, #22c55e)',
                    fontSize: '0.875rem',
                }}>
                    {toast.msg}
                </div>
            )}

            {loading ? (
                <div style={{ color: 'var(--text-secondary)' }}>Loading…</div>
            ) : (
                <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid var(--border-color)', textAlign: 'left' }}>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Email</th>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Display Name</th>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Role</th>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Status</th>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Created</th>
                                <th style={{ padding: '8px 12px', fontWeight: 600 }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {visibleUsers.map(u => (
                                <tr key={u.id} style={{
                                    borderBottom: '1px solid var(--border-color)',
                                    opacity: u.deleted ? 0.5 : 1,
                                }}>
                                    <td style={{ padding: '8px 12px' }}>{u.email}</td>
                                    <td style={{ padding: '8px 12px' }}>{u.display_name || '—'}</td>
                                    <td style={{ padding: '8px 12px' }}>
                                        <span style={{
                                            fontSize: '0.75rem',
                                            padding: '2px 6px',
                                            borderRadius: 4,
                                            background: u.role === 'admin' ? 'var(--accent-indigo)' : 'var(--bg-secondary)',
                                            color: u.role === 'admin' ? '#fff' : 'var(--text-secondary)',
                                            fontWeight: 600,
                                        }}>
                                            {u.role}
                                        </span>
                                    </td>
                                    <td style={{ padding: '8px 12px' }}>
                                        <span style={{
                                            fontSize: '0.75rem',
                                            padding: '2px 6px',
                                            borderRadius: 4,
                                            background: u.deleted
                                                ? 'rgba(239,68,68,0.25)'
                                                : u.active ? 'rgba(34,197,94,0.15)' : 'rgba(239,68,68,0.15)',
                                            color: u.deleted
                                                ? 'var(--accent-red, #ef4444)'
                                                : u.active ? 'var(--accent-green, #22c55e)' : 'var(--accent-red, #ef4444)',
                                            textDecoration: u.deleted ? 'line-through' : 'none',
                                        }}>
                                            {u.deleted ? 'Deleted' : u.active ? 'Active' : 'Inactive'}
                                        </span>
                                    </td>
                                    <td style={{ padding: '8px 12px', color: 'var(--text-secondary)' }}>
                                        {u.created_at ? new Date(u.created_at).toLocaleDateString() : '—'}
                                    </td>
                                    <td style={{ padding: '8px 12px' }}>
                                        <div style={{ display: 'flex', gap: 6 }}>
                                            {u.deleted ? (
                                                <button
                                                    className="action-btn"
                                                    style={{ fontSize: '0.8rem', padding: '3px 8px' }}
                                                    onClick={() => restoreUser(u)}
                                                >
                                                    Restore
                                                </button>
                                            ) : (
                                                <>
                                                    <button
                                                        className="action-btn"
                                                        style={{ fontSize: '0.8rem', padding: '3px 8px' }}
                                                        onClick={() => toggleActive(u)}
                                                    >
                                                        {u.active ? 'Deactivate' : 'Reactivate'}
                                                    </button>
                                                    <button
                                                        className="action-btn"
                                                        style={{ fontSize: '0.8rem', padding: '3px 8px' }}
                                                        onClick={() => setResetModal({ user: u })}
                                                    >
                                                        Reset PW
                                                    </button>
                                                    <button
                                                        className="action-btn"
                                                        style={{
                                                            fontSize: '0.8rem',
                                                            padding: '3px 8px',
                                                            color: 'var(--accent-red, #ef4444)',
                                                        }}
                                                        onClick={() => setConfirmDelete({ user: u })}
                                                    >
                                                        Delete
                                                    </button>
                                                </>
                                            )}
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {showAddModal && (
                <AddUserModal
                    onClose={() => setShowAddModal(false)}
                    onCreated={() => { setShowAddModal(false); showToast('User created'); }}
                />
            )}

            {resetModal && (
                <ResetPasswordModal
                    user={resetModal.user}
                    onClose={() => setResetModal(null)}
                    onSuccess={() => { setResetModal(null); showToast('Password reset'); }}
                />
            )}

            {confirmDelete && (
                <div style={{
                    position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
                    background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center',
                    justifyContent: 'center', zIndex: 1000,
                }} onClick={() => setConfirmDelete(null)}>
                    <div style={{
                        background: 'var(--bg-primary)', borderRadius: 8, padding: 24,
                        minWidth: 360, maxWidth: 440, boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
                    }} onClick={e => e.stopPropagation()}>
                        <h3 style={{ margin: '0 0 12px' }}>Delete User</h3>
                        <p style={{ color: 'var(--text-secondary)', margin: '0 0 20px' }}>
                            Are you sure you want to delete <strong>{confirmDelete.user.display_name || confirmDelete.user.email}</strong>?
                            The user will be deactivated and marked as deleted. You can restore them later.
                        </p>
                        <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                            <button className="action-btn" onClick={() => setConfirmDelete(null)}>Cancel</button>
                            <button
                                className="primary-btn"
                                style={{ background: 'var(--accent-red, #ef4444)' }}
                                onClick={() => deleteUser(confirmDelete.user)}
                            >
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

// ─────────────────────────────────────────────
// DemoDataSettings — admin-only demo data tab
// ─────────────────────────────────────────────
function DemoDataSettings() {
    const [seedStatus, setSeedStatus] = useState(null);
    const [loading, setLoading] = useState(true);
    const [operating, setOperating] = useState(false);
    const [confirmModal, setConfirmModal] = useState(null);
    const [showEraseModal, setShowEraseModal] = useState(false);
    const [eraseInput, setEraseInput] = useState('');
    const navigate = useNavigate();

    const loadStatus = useCallback(() => {
        setLoading(true);
        seedApi.status()
            .then(data => setSeedStatus(data))
            .catch(() => toast.error('Failed to load demo data status'))
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => { loadStatus(); }, [loadStatus]);

    const doLoad = async () => {
        setConfirmModal(null);
        setOperating(true);
        try {
            await seedApi.load();
            toast.success('Demo data loaded successfully');
            loadStatus();
        } catch {
            // Global interceptor already shows a toast for errors; nothing extra needed.
        } finally {
            setOperating(false);
        }
    };

    const handleLoad = () => {
        if (seedStatus?.has_demo_data) {
            setConfirmModal({
                action: 'load',
                title: 'Replace Demo Data',
                message: 'This will replace the existing demo dataset. Your own content will not be affected.',
                confirmText: 'Replace',
                confirmStyle: 'danger',
            });
        } else {
            doLoad();
        }
    };

    const doRemove = async () => {
        setConfirmModal(null);
        setOperating(true);
        try {
            await seedApi.remove();
            toast.success('Demo data removed');
            loadStatus();
        } catch {
            // Global interceptor already shows a toast for errors.
        } finally {
            setOperating(false);
        }
    };

    const handleRemove = () => {
        setConfirmModal({
            action: 'remove',
            title: 'Remove Demo Data',
            message: 'This will permanently remove all demo data. Your own content will not be affected.',
            confirmText: 'Remove',
            confirmStyle: 'danger',
        });
    };

    const doEraseAll = async () => {
        setShowEraseModal(false);
        setEraseInput('');
        setOperating(true);
        try {
            await seedApi.resetAll();
            toast.success('All data has been erased');
            navigate('/');
        } catch {
            // Global interceptor handles toast
        } finally {
            setOperating(false);
        }
    };

    const seededDate = seedStatus?.seeded_at
        ? new Date(seedStatus.seeded_at).toLocaleString()
        : null;

    return (
        <div>
            <h3 style={{ marginTop: 0, marginBottom: 8 }}>Demo Data</h3>
            <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginBottom: 24 }}>
                Populate the app with a set of sample test cases, runs, folders, and categories to explore all features.
            </p>

            <div className="glass-panel" style={{ padding: 24, marginBottom: 24 }}>
                {loading ? (
                    <div style={{ color: 'var(--text-secondary)' }}>Loading status…</div>
                ) : (
                    <div style={{ marginBottom: 16, fontSize: '0.9rem' }}>
                        {seedStatus?.has_demo_data ? (
                            <span style={{ color: 'var(--accent-green, #22c55e)' }}>
                                ✓ Demo data loaded on {seededDate}
                                {seedStatus.counts && (
                                    <span style={{ color: 'var(--text-secondary)', marginLeft: 8 }}>
                                        ({seedStatus.counts.test_cases} tests · {seedStatus.counts.test_runs} runs · {seedStatus.counts.folders} folders)
                                    </span>
                                )}
                            </span>
                        ) : (
                            <span style={{ color: 'var(--text-secondary)' }}>No demo data loaded</span>
                        )}
                    </div>
                )}

                <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                    <button
                        className="primary-btn"
                        onClick={handleLoad}
                        disabled={operating || loading}
                    >
                        {operating ? 'Loading…' : 'Load Demo Data'}
                    </button>

                    {seedStatus?.has_demo_data && (
                        <button
                            className="action-btn"
                            style={{ color: 'var(--accent-red)' }}
                            onClick={handleRemove}
                            disabled={operating}
                        >
                            Remove Demo Data
                        </button>
                    )}
                </div>
            </div>

            {confirmModal && (
                <Modal
                    type="confirm"
                    title={confirmModal.title}
                    message={confirmModal.message}
                    confirmText={confirmModal.confirmText}
                    confirmStyle={confirmModal.confirmStyle}
                    onConfirm={confirmModal.action === 'load' ? doLoad : doRemove}
                    onCancel={() => setConfirmModal(null)}
                />
            )}

            {/* ── Danger Zone ──────────────────────────────────────────── */}
            <div style={{
                marginTop: 40,
                border: '1px solid var(--accent-red, #ef4444)',
                borderRadius: 8,
                padding: 24,
            }}>
                <h3 style={{ margin: '0 0 8px', color: 'var(--accent-red, #ef4444)' }}>Danger Zone</h3>
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginBottom: 16 }}>
                    Permanently erase <strong>all</strong> application data — test cases, categories, runs, requirements,
                    folders, configurations, and audit logs. User accounts are preserved.
                </p>
                <button
                    className="action-btn"
                    style={{
                        color: '#fff',
                        background: 'var(--accent-red, #ef4444)',
                        border: 'none',
                        padding: '8px 20px',
                    }}
                    onClick={() => { setEraseInput(''); setShowEraseModal(true); }}
                    disabled={operating}
                    data-testid="erase-all-data-button"
                >
                    Erase All Data
                </button>
            </div>

            {showEraseModal && (
                <div className="modal-overlay" onClick={() => setShowEraseModal(false)}>
                    <div className="modal-content" onClick={e => e.stopPropagation()}>
                        <header className="modal-header">
                            <h3 className="modal-title">Erase All Data</h3>
                        </header>
                        <div className="modal-body">
                            <p style={{ marginBottom: 16, color: 'var(--accent-red, #ef4444)', fontWeight: 600 }}>
                                This action is irreversible. All test cases, runs, categories, requirements, folders, and
                                configurations will be permanently deleted.
                            </p>
                            <p style={{ marginBottom: 12 }}>Type <strong>ERASE</strong> to confirm:</p>
                            <input
                                autoFocus
                                className="modern-input"
                                style={{ width: '100%' }}
                                value={eraseInput}
                                onChange={e => setEraseInput(e.target.value)}
                                placeholder="Type ERASE"
                                data-testid="erase-confirm-input"
                            />
                        </div>
                        <footer className="modal-footer">
                            <button className="action-btn" onClick={() => setShowEraseModal(false)}>Cancel</button>
                            <button
                                className="primary-btn"
                                style={{ background: 'var(--accent-red, #ef4444)' }}
                                disabled={eraseInput !== 'ERASE'}
                                onClick={doEraseAll}
                                data-testid="erase-confirm-button"
                            >
                                Erase Everything
                            </button>
                        </footer>
                    </div>
                </div>
            )}
        </div>
    );
}

function AddUserModal({ onClose, onCreated }) {
    const [email, setEmail] = useState('');
    const [displayName, setDisplayName] = useState('');
    const [password, setPassword] = useState('');
    const [role, setRole] = useState('member');
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [onClose]);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setSubmitting(true);
        try {
            await usersApi.create({ email, display_name: displayName, password, role });
            onCreated();
        } catch (err) {
            setError(err?.response?.data?.error || 'Failed to create user');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div
            onClick={onClose}
            style={{
                position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
                backdropFilter: 'blur(4px)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
            }}
        >
            <div
                onClick={e => e.stopPropagation()}
                style={{
                    width: 420,
                    padding: '32px 36px',
                    position: 'relative',
                    background: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-lg)',
                    boxShadow: '0 24px 64px rgba(0,0,0,0.5)',
                }}
            >
                <button
                    onClick={onClose}
                    style={{
                        position: 'absolute', top: 16, right: 16,
                        background: 'none', border: 'none', cursor: 'pointer',
                        color: 'var(--text-secondary)', fontSize: '1.1rem', lineHeight: 1, padding: 4,
                    }}
                    aria-label="Close"
                >✕</button>

                <h3 style={{ marginTop: 0, marginBottom: 4, fontSize: '1.2rem' }}>Add User</h3>
                <p style={{ margin: '0 0 24px', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                    Create a new account for a team member.
                </p>

                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Email <span style={{ color: 'var(--accent-red, #ef4444)' }}>*</span>
                        </label>
                        <input className="modern-input" type="email" value={email} onChange={e => setEmail(e.target.value)} required autoFocus placeholder="member@example.com" style={{ width: '100%' }} />
                    </div>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Display Name <span style={{ fontSize: '0.8rem', fontWeight: 400, color: 'var(--text-secondary)' }}>(optional)</span>
                        </label>
                        <input className="modern-input" value={displayName} onChange={e => setDisplayName(e.target.value)} placeholder="Jane Doe" style={{ width: '100%' }} />
                    </div>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Password <span style={{ color: 'var(--accent-red, #ef4444)' }}>*</span>
                            <span style={{ fontSize: '0.8rem', fontWeight: 400, color: 'var(--text-secondary)', marginLeft: 6 }}>min 8 characters</span>
                        </label>
                        <input className="modern-input" type="password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} placeholder="••••••••" style={{ width: '100%' }} />
                    </div>
                    <div style={{ marginBottom: 24 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>Role</label>
                        <select className="modern-select" value={role} onChange={e => setRole(e.target.value)} style={{ width: '100%' }}>
                            <option value="member">Member</option>
                            <option value="admin">Admin</option>
                        </select>
                    </div>
                    {error && (
                        <div style={{
                            marginBottom: 16, padding: '10px 14px', borderRadius: 6,
                            background: 'rgba(239,68,68,0.1)', color: 'var(--accent-red, #ef4444)', fontSize: '0.875rem',
                        }}>
                            {error}
                        </div>
                    )}
                    <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button type="button" className="action-btn" onClick={onClose}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={submitting}>{submitting ? 'Creating…' : 'Create User'}</button>
                    </div>
                </form>
            </div>
        </div>
    );
}

function ResetPasswordModal({ user, onClose, onSuccess }) {
    const [newPassword, setNewPassword] = useState('');
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        const onKey = (e) => { if (e.key === 'Escape') onClose(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [onClose]);

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setSubmitting(true);
        try {
            await usersApi.update(user.id, { password: newPassword });
            onSuccess();
        } catch (err) {
            setError(err?.response?.data?.error || 'Failed to reset password');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <div
            onClick={onClose}
            style={{
                position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)',
                backdropFilter: 'blur(4px)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
            }}
        >
            <div
                onClick={e => e.stopPropagation()}
                style={{
                    width: 380,
                    padding: '32px 36px',
                    position: 'relative',
                    background: 'var(--bg-secondary)',
                    border: '1px solid var(--border-color)',
                    borderRadius: 'var(--radius-lg)',
                    boxShadow: '0 24px 64px rgba(0,0,0,0.5)',
                }}
            >
                <button
                    onClick={onClose}
                    style={{
                        position: 'absolute', top: 16, right: 16,
                        background: 'none', border: 'none', cursor: 'pointer',
                        color: 'var(--text-secondary)', fontSize: '1.1rem', lineHeight: 1, padding: 4,
                    }}
                    aria-label="Close"
                >✕</button>
                <h3 style={{ marginTop: 0, marginBottom: 4, fontSize: '1.2rem' }}>Reset Password</h3>
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginBottom: 24 }}>
                    Set a new password for <strong style={{ color: 'var(--text-primary)' }}>{user.email}</strong>.
                </p>
                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 20 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            New Password
                            <span style={{ fontSize: '0.8rem', fontWeight: 400, color: 'var(--text-secondary)', marginLeft: 6 }}>min 8 characters</span>
                        </label>
                        <input className="modern-input" type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} required minLength={8} autoFocus placeholder="••••••••" style={{ width: '100%' }} />
                    </div>
                    {error && (
                        <div style={{
                            marginBottom: 16, padding: '10px 14px', borderRadius: 6,
                            background: 'rgba(239,68,68,0.1)', color: 'var(--accent-red, #ef4444)', fontSize: '0.875rem',
                        }}>
                            {error}
                        </div>
                    )}
                    <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
                        <button type="button" className="action-btn" onClick={onClose}>Cancel</button>
                        <button type="submit" className="primary-btn" disabled={submitting}>{submitting ? 'Saving…' : 'Reset Password'}</button>
                    </div>
                </form>
            </div>
        </div>
    );
}
