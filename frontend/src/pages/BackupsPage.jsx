import { useState, useEffect, useRef, useCallback } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { backups } from '../api';
import { toast } from '../toast';
import Modal from '../components/Modal';
import { useSubscription } from '../hooks/useSubscription';
import { useWebSocket } from '../hooks/useWebSocket';

function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '—';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
    return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDate(dateStr) {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleString();
}

const STATUS_STYLES = {
    completed: { background: '#e6f9ed', color: '#1a7d3f', label: 'Completed' },
    in_progress: { background: '#fff8e6', color: '#b8860b', label: 'In Progress' },
    failed: { background: '#fde8e8', color: '#c53030', label: 'Failed' },
};

const TYPE_LABELS = {
    manual: 'Manual',
    automatic: 'Automatic',
    'pre-restore': 'Pre-Restore',
};

export function BackupsSettings() {
    const { user } = useAuth();
    const [backupList, setBackupList] = useState([]);
    const [loading, setLoading] = useState(true);
    const [creating, setCreating] = useState(false);
    const [restoring, setRestoring] = useState(false);
    const [modal, setModal] = useState(null);
    const [activeTab, setActiveTab] = useState('backups');

    // Schedule state
    const [schedule, setSchedule] = useState(null);
    const [scheduleForm, setScheduleForm] = useState({ enabled: false, interval_hours: 24, retention_count: 7 });
    const [savingSchedule, setSavingSchedule] = useState(false);

    // Upload restore state
    const fileInputRef = useRef(null);
    const [uploadFile, setUploadFile] = useState(null);

    // Maintenance mode state
    const [maintenance, setMaintenance] = useState(false);

    const isAdmin = user?.role === 'admin';

    const loadBackups = useCallback(async () => {
        try {
            const data = await backups.list();
            setBackupList(data || []);
        } catch (e) {
            // silent — toast from interceptor
        } finally {
            setLoading(false);
        }
    }, []);

    const loadSchedule = useCallback(async () => {
        try {
            const data = await backups.schedule.get();
            setSchedule(data);
            setScheduleForm({
                enabled: data.enabled,
                interval_hours: data.interval_hours,
                retention_count: data.retention_count,
            });
        } catch (e) {
            // silent
        }
    }, []);

    // 018-websocket-realtime: subscribe to backup events instead of polling
    const { registerRefresh, unregisterRefresh } = useWebSocket();
    useSubscription(isAdmin ? 'backups:*' : null, useCallback((event) => {
        switch (event.type) {
            case 'backup_created':
                if (event.data && event.data.id) {
                    setBackupList(prev => [event.data, ...prev]);
                }
                break;
            case 'backup_deleted':
                if (event.data && event.data.id) {
                    setBackupList(prev => prev.filter(b => b.id !== event.data.id));
                }
                break;
            case 'backup_restored':
                setRestoring(false);
                loadBackups();
                break;
            case 'maintenance_changed':
                setMaintenance(!!event.data?.maintenance);
                if (!event.data?.maintenance) {
                    setRestoring(false);
                }
                break;
            case 'backup_schedule_updated':
                if (event.data) {
                    setSchedule(event.data);
                    setScheduleForm({
                        enabled: event.data.enabled,
                        interval_hours: event.data.interval_hours,
                        retention_count: event.data.retention_count,
                    });
                }
                break;
        }
    }, [loadBackups]));

    useEffect(() => {
        if (!isAdmin) return;
        loadBackups();
        loadSchedule();
        // Register refresh callbacks for reconnection
        registerRefresh('backups', loadBackups);
        registerRefresh('backupSchedule', loadSchedule);
        return () => {
            unregisterRefresh('backups');
            unregisterRefresh('backupSchedule');
        };
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isAdmin, loadBackups, loadSchedule]);

    const handleCreate = async () => {
        setCreating(true);
        try {
            const result = await backups.create();
            toast.success(`Backup created (${formatBytes(result.file_size)})`);
            // loadBackups() removed — server broadcast updates via WebSocket
        } catch (e) {
            // toast from interceptor
        } finally {
            setCreating(false);
        }
    };

    const handleDownload = async (backup) => {
        try {
            const response = await backups.download(backup.id);
            const url = window.URL.createObjectURL(new Blob([response.data]));
            const link = document.createElement('a');
            link.href = url;
            link.setAttribute('download', `backup-${backup.id}.db`);
            document.body.appendChild(link);
            link.click();
            link.parentNode.removeChild(link);
            window.URL.revokeObjectURL(url);
        } catch (e) {
            toast.error('Download failed');
        }
    };

    const handleDelete = (backup) => {
        setModal({
            type: 'confirm',
            title: 'Delete Backup',
            message: `Delete backup from ${formatDate(backup.created_at)}? This cannot be undone.`,
            confirmText: 'Delete',
            confirmStyle: 'danger',
            onConfirm: async () => {
                setModal(null);
                try {
                    await backups.delete(backup.id);
                    toast.success('Backup deleted');
                } catch (e) { /* toast from interceptor */ }
            },
        });
    };

    const handleRestore = (backup) => {
        setModal({
            type: 'prompt',
            title: 'Restore Database',
            message: `This will replace ALL current data with the backup from ${formatDate(backup.created_at)}. A safety backup of the current state will be created first.\n\nType "CONFIRM RESTORE" to proceed:`,
            confirmText: 'Restore',
            confirmStyle: 'danger',
            onConfirm: async (value) => {
                if (value !== 'CONFIRM RESTORE') {
                    toast.error('Please type exactly "CONFIRM RESTORE"');
                    return;
                }
                setModal(null);
                setRestoring(true);
                setMaintenance(true);
                try {
                    const result = await backups.restore(backup.id, value);
                    toast.success(`Database restored. Safety backup: ${result.pre_restore_backup_id?.slice(0, 8)}...`);
                } catch (e) {
                    // toast from interceptor
                } finally {
                    setRestoring(false);
                    setMaintenance(false);
                }
            },
        });
    };

    const handleUploadRestore = () => {
        if (!uploadFile) {
            toast.error('Please select a backup file first');
            return;
        }
        setModal({
            type: 'prompt',
            title: 'Upload & Restore',
            message: `This will replace ALL current data with the uploaded file "${uploadFile.name}". A safety backup of the current state will be created first.\n\nType "CONFIRM RESTORE" to proceed:`,
            confirmText: 'Restore',
            confirmStyle: 'danger',
            onConfirm: async (value) => {
                if (value !== 'CONFIRM RESTORE') {
                    toast.error('Please type exactly "CONFIRM RESTORE"');
                    return;
                }
                setModal(null);
                setRestoring(true);
                setMaintenance(true);
                try {
                    const result = await backups.uploadRestore(uploadFile, value);
                    toast.success(`Database restored from upload. Safety backup: ${result.pre_restore_backup_id?.slice(0, 8)}...`);
                    setUploadFile(null);
                    if (fileInputRef.current) fileInputRef.current.value = '';
                } catch (e) {
                    // toast from interceptor
                } finally {
                    setRestoring(false);
                    setMaintenance(false);
                }
            },
        });
    };

    const handleSaveSchedule = async () => {
        setSavingSchedule(true);
        try {
            const result = await backups.schedule.update(scheduleForm);
            setSchedule(result);
            toast.success('Backup schedule updated');
        } catch (e) {
            // toast from interceptor
        } finally {
            setSavingSchedule(false);
        }
    };

    const tabs = [
        { id: 'backups', label: 'Backups' },
        { id: 'restore', label: 'Restore' },
        { id: 'schedule', label: 'Schedule' },
    ];

    return (
        <div>
            {/* Maintenance overlay */}
            {maintenance && (
                <div style={{
                    position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
                    background: 'rgba(0,0,0,0.6)', zIndex: 9999,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                    <div style={{
                        background: 'var(--bg-primary)', borderRadius: 12, padding: '40px 50px',
                        textAlign: 'center', boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
                    }}>
                        <div style={{ fontSize: 40, marginBottom: 16 }}>🔄</div>
                        <h2 style={{ margin: '0 0 8px', color: 'var(--text-primary)' }}>Restore in Progress</h2>
                        <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                            The system is under maintenance. Please wait...
                        </p>
                    </div>
                </div>
            )}

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 20 }}>
                <h3 style={{ margin: 0, fontSize: '1rem', color: 'var(--text-primary)' }}>
                    Database Backups
                </h3>
                <button
                    onClick={handleCreate}
                    disabled={creating}
                    style={{
                        padding: '8px 18px', borderRadius: 8, border: 'none', cursor: 'pointer',
                        background: 'var(--accent-indigo)', color: '#fff', fontWeight: 600, fontSize: '0.85rem',
                        opacity: creating ? 0.6 : 1,
                    }}
                >
                    {creating ? 'Creating...' : '+ Create Backup'}
                </button>
            </div>

            {/* Tabs */}
            <div style={{ display: 'flex', gap: 4, marginBottom: 20, borderBottom: '1px solid var(--border-color)', paddingBottom: 0 }}>
                {tabs.map(tab => (
                    <button
                        key={tab.id}
                        onClick={() => setActiveTab(tab.id)}
                        style={{
                            padding: '8px 16px', border: 'none', cursor: 'pointer',
                            background: 'transparent', fontSize: '0.85rem', fontWeight: 500,
                            color: activeTab === tab.id ? 'var(--accent-indigo)' : 'var(--text-secondary)',
                            borderBottom: activeTab === tab.id ? '2px solid var(--accent-indigo)' : '2px solid transparent',
                            marginBottom: -1,
                        }}
                    >
                        {tab.label}
                    </button>
                ))}
            </div>

            {/* Backups Tab */}
            {activeTab === 'backups' && (
                <div>
                    {loading ? (
                        <p style={{ color: 'var(--text-secondary)', textAlign: 'center', padding: 40 }}>Loading backups...</p>
                    ) : backupList.length === 0 ? (
                        <div style={{ textAlign: 'center', padding: 60, color: 'var(--text-secondary)' }}>
                            <div style={{ fontSize: 40, marginBottom: 12 }}>💾</div>
                            <p style={{ margin: 0 }}>No backups yet. Click "Create Backup" to get started.</p>
                        </div>
                    ) : (
                        <table className="analytics-table" style={{ width: '100%' }}>
                            <thead>
                                <tr>
                                    <th>Type</th>
                                    <th>Status</th>
                                    <th>Size</th>
                                    <th>Creator</th>
                                    <th>Created</th>
                                    <th>Completed</th>
                                    <th style={{ textAlign: 'right' }}>Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                {backupList.map(b => {
                                    const st = STATUS_STYLES[b.status] || {};
                                    return (
                                        <tr key={b.id}>
                                            <td>
                                                <span style={{
                                                    padding: '2px 8px', borderRadius: 4, fontSize: '0.78rem', fontWeight: 500,
                                                    background: 'var(--bg-secondary)', color: 'var(--text-secondary)',
                                                }}>
                                                    {TYPE_LABELS[b.type] || b.type}
                                                </span>
                                            </td>
                                            <td>
                                                <span style={{
                                                    padding: '2px 8px', borderRadius: 4, fontSize: '0.78rem', fontWeight: 600,
                                                    background: st.background, color: st.color,
                                                }}>
                                                    {st.label || b.status}
                                                </span>
                                            </td>
                                            <td>{formatBytes(b.file_size)}</td>
                                            <td>{b.creator_name || (b.type === 'automatic' ? 'System' : '—')}</td>
                                            <td>{formatDate(b.created_at)}</td>
                                            <td>{formatDate(b.completed_at)}</td>
                                            <td style={{ textAlign: 'right' }}>
                                                <div style={{ display: 'flex', gap: 6, justifyContent: 'flex-end' }}>
                                                    {b.status === 'completed' && (
                                                        <>
                                                            <button
                                                                onClick={() => handleDownload(b)}
                                                                title="Download"
                                                                style={actionBtnStyle}
                                                            >⬇</button>
                                                            <button
                                                                onClick={() => handleRestore(b)}
                                                                title="Restore from this backup"
                                                                style={{ ...actionBtnStyle, color: 'var(--accent-orange, #e67e22)' }}
                                                            >↩</button>
                                                        </>
                                                    )}
                                                    <button
                                                        onClick={() => handleDelete(b)}
                                                        title="Delete"
                                                        style={{ ...actionBtnStyle, color: 'var(--accent-red, #c53030)' }}
                                                    >✕</button>
                                                </div>
                                            </td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    )}
                </div>
            )}

            {/* Restore Tab */}
            {activeTab === 'restore' && (
                <div style={{ maxWidth: 600 }}>
                    <h3 style={{ margin: '0 0 16px', color: 'var(--text-primary)', fontSize: '1rem' }}>
                        Restore from Server Backup
                    </h3>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', margin: '0 0 12px' }}>
                        Select a completed backup to restore. The system will enter maintenance mode during the restore.
                    </p>
                    {backupList.filter(b => b.status === 'completed').length === 0 ? (
                        <p style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>No completed backups available.</p>
                    ) : (
                        <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 24 }}>
                            <select
                                id="restore-select"
                                style={{
                                    flex: 1, padding: '8px 12px', borderRadius: 8,
                                    border: '1px solid var(--border-color)', background: 'var(--bg-primary)',
                                    color: 'var(--text-primary)', fontSize: '0.85rem',
                                }}
                                defaultValue=""
                            >
                                <option value="" disabled>Select a backup...</option>
                                {backupList.filter(b => b.status === 'completed').map(b => (
                                    <option key={b.id} value={b.id}>
                                        {TYPE_LABELS[b.type]} — {formatDate(b.created_at)} ({formatBytes(b.file_size)})
                                    </option>
                                ))}
                            </select>
                            <button
                                onClick={() => {
                                    const sel = document.getElementById('restore-select');
                                    if (!sel?.value) { toast.error('Please select a backup'); return; }
                                    const b = backupList.find(x => x.id === sel.value);
                                    if (b) handleRestore(b);
                                }}
                                disabled={restoring}
                                style={{
                                    padding: '8px 16px', borderRadius: 8, border: 'none', cursor: 'pointer',
                                    background: 'var(--accent-orange, #e67e22)', color: '#fff', fontWeight: 600,
                                    fontSize: '0.85rem', opacity: restoring ? 0.6 : 1, whiteSpace: 'nowrap',
                                }}
                            >
                                Restore
                            </button>
                        </div>
                    )}

                    <h3 style={{ margin: '24px 0 16px', color: 'var(--text-primary)', fontSize: '1rem', borderTop: '1px solid var(--border-color)', paddingTop: 20 }}>
                        Upload & Restore
                    </h3>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', margin: '0 0 12px' }}>
                        Upload a previously downloaded backup file (.db) from your local machine.
                    </p>
                    <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                        <input
                            ref={fileInputRef}
                            type="file"
                            accept=".db"
                            onChange={(e) => setUploadFile(e.target.files?.[0] || null)}
                            style={{ flex: 1, fontSize: '0.85rem' }}
                        />
                        <button
                            onClick={handleUploadRestore}
                            disabled={restoring || !uploadFile}
                            style={{
                                padding: '8px 16px', borderRadius: 8, border: 'none', cursor: 'pointer',
                                background: 'var(--accent-orange, #e67e22)', color: '#fff', fontWeight: 600,
                                fontSize: '0.85rem', opacity: (restoring || !uploadFile) ? 0.6 : 1, whiteSpace: 'nowrap',
                            }}
                        >
                            Upload & Restore
                        </button>
                    </div>
                </div>
            )}

            {/* Schedule Tab */}
            {activeTab === 'schedule' && (
                <div style={{ maxWidth: 500 }}>
                    <h3 style={{ margin: '0 0 16px', color: 'var(--text-primary)', fontSize: '1rem' }}>
                        Automatic Backup Schedule
                    </h3>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
                        <label style={{ display: 'flex', alignItems: 'center', gap: 10, cursor: 'pointer' }}>
                            <input
                                type="checkbox"
                                checked={scheduleForm.enabled}
                                onChange={(e) => setScheduleForm(f => ({ ...f, enabled: e.target.checked }))}
                                style={{ width: 18, height: 18 }}
                            />
                            <span style={{ fontSize: '0.9rem', color: 'var(--text-primary)', fontWeight: 500 }}>
                                Enable automatic backups
                            </span>
                        </label>

                        <div>
                            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: 4 }}>
                                Interval (hours)
                            </label>
                            <input
                                type="number"
                                min="1"
                                value={scheduleForm.interval_hours}
                                onChange={(e) => setScheduleForm(f => ({ ...f, interval_hours: parseInt(e.target.value) || 1 }))}
                                style={{
                                    padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border-color)',
                                    background: 'var(--bg-primary)', color: 'var(--text-primary)',
                                    fontSize: '0.85rem', width: 120,
                                }}
                            />
                            <span style={{ marginLeft: 8, fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                                ({scheduleForm.interval_hours >= 24
                                    ? `${(scheduleForm.interval_hours / 24).toFixed(1)} days`
                                    : `${scheduleForm.interval_hours} hours`})
                            </span>
                        </div>

                        <div>
                            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: 4 }}>
                                Retention (max automatic backups to keep)
                            </label>
                            <input
                                type="number"
                                min="1"
                                value={scheduleForm.retention_count}
                                onChange={(e) => setScheduleForm(f => ({ ...f, retention_count: parseInt(e.target.value) || 1 }))}
                                style={{
                                    padding: '8px 12px', borderRadius: 8, border: '1px solid var(--border-color)',
                                    background: 'var(--bg-primary)', color: 'var(--text-primary)',
                                    fontSize: '0.85rem', width: 120,
                                }}
                            />
                        </div>

                        <button
                            onClick={handleSaveSchedule}
                            disabled={savingSchedule}
                            style={{
                                padding: '8px 20px', borderRadius: 8, border: 'none', cursor: 'pointer',
                                background: 'var(--accent-indigo)', color: '#fff', fontWeight: 600,
                                fontSize: '0.85rem', width: 'fit-content', opacity: savingSchedule ? 0.6 : 1,
                            }}
                        >
                            {savingSchedule ? 'Saving...' : 'Save Schedule'}
                        </button>

                        {schedule && (
                            <div style={{
                                padding: 16, borderRadius: 8, background: 'var(--bg-secondary)',
                                fontSize: '0.82rem', color: 'var(--text-secondary)', marginTop: 8,
                            }}>
                                <div><strong>Status:</strong> {schedule.enabled ? '✅ Enabled' : '⏸ Disabled'}</div>
                                <div><strong>Last run:</strong> {formatDate(schedule.last_run_at)}</div>
                                <div><strong>Next scheduled:</strong> {schedule.enabled ? formatDate(schedule.next_run_at) : '—'}</div>
                            </div>
                        )}
                    </div>
                </div>
            )}

            {modal && (
                <Modal
                    title={modal.title}
                    message={modal.message}
                    type={modal.type}
                    confirmText={modal.confirmText}
                    confirmStyle={modal.confirmStyle}
                    onConfirm={modal.onConfirm}
                    onCancel={() => setModal(null)}
                />
            )}
        </div>
    );
}

export default function BackupsPage() {
    return (
        <div style={{ padding: '24px 32px', maxWidth: 1100, margin: '0 auto' }}>
            <BackupsSettings />
        </div>
    );
}

const actionBtnStyle = {
    padding: '4px 8px', borderRadius: 6, border: '1px solid var(--border-color)',
    cursor: 'pointer', background: 'var(--bg-primary)', fontSize: '0.8rem',
    color: 'var(--text-secondary)', lineHeight: 1,
};
