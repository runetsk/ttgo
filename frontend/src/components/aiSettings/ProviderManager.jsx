import React, { useState, useEffect, useCallback } from 'react';
import { aiGeneration } from '../../api';
import { toast } from '../../toast';
import { s } from './styles';
import ProviderModal from './ProviderModal';
import DeleteModal from './DeleteModal';
import ProviderCard from './ProviderCard';

/* ── LLM Providers Section (list, add/edit/delete/test, modals) ── */
export default function ProviderManager({ isAdmin }) {
    const [providers, setProviders]         = useState([]);
    const [loading, setLoading]             = useState(true);
    const [modal, setModal]                 = useState(null);
    const [testingId, setTestingId]         = useState(null);
    const [testResults, setTestResults]     = useState({});
    const [confirmDelete, setConfirmDelete] = useState(null);

    const loadProviders = useCallback(() => {
        setLoading(true);
        aiGeneration.listProviders()
            .then(data => setProviders(data || []))
            .catch(() => toast.error('Failed to load providers'))
            .finally(() => setLoading(false));
    }, []);

    useEffect(() => {
        loadProviders();
    }, [loadProviders]);

    const handleTestConnection = async (provider) => {
        setTestingId(provider.id);
        setTestResults(prev => ({ ...prev, [provider.id]: null }));
        try {
            const result = await aiGeneration.testConnection(provider.id);
            setTestResults(prev => ({ ...prev, [provider.id]: result }));
        } catch (err) {
            setTestResults(prev => ({ ...prev, [provider.id]: { success: false, error: err?.response?.data?.error || err.message } }));
        } finally {
            setTestingId(null);
        }
    };

    const handleSetDefault = async (provider) => {
        try {
            await aiGeneration.setDefault(provider.id);
            toast.success(`${provider.label} set as default`);
            loadProviders();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to set default');
        }
    };

    const handleDeleteExecute = async () => {
        if (!confirmDelete) return;
        setConfirmDelete(null);
        try {
            await aiGeneration.deleteProvider(confirmDelete.id);
            toast.success('Provider deleted');
            loadProviders();
        } catch (err) {
            toast.error(err?.response?.data?.error || 'Failed to delete provider');
        }
    };

    return (
        <>
            {/* ── LLM Providers Section ── */}
            <section style={s.section}>
                <div style={s.sectionHead}>
                    <div style={s.sectionHeadLeft}>
                        <span style={s.sectionDot} />
                        <h4 style={s.sectionTitle}>LLM Providers</h4>
                        {providers.length > 0 && (
                            <span style={s.sectionCount}>{providers.length}</span>
                        )}
                    </div>
                    {isAdmin && (
                        <button className="primary-btn" style={s.addBtn} onClick={() => setModal({ type: 'add' })}>
                            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
                                <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
                            </svg>
                            Add Provider
                        </button>
                    )}
                </div>

                {loading ? (
                    <div style={s.loadingState}>
                        <span style={s.loadingSpinner} />
                        Loading providers…
                    </div>
                ) : providers.length === 0 ? (
                    <div style={s.emptyState}>
                        <div style={s.emptyIcon}>
                            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="12" cy="12" r="3"/><path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83"/>
                            </svg>
                        </div>
                        <p style={s.emptyTitle}>No providers configured</p>
                        <p style={s.emptyDesc}>
                            {isAdmin
                                ? 'Add an LLM provider to enable AI-powered test case generation.'
                                : 'An admin needs to configure an LLM provider before you can generate test cases.'}
                        </p>
                        {isAdmin && (
                            <button className="primary-btn" style={{ marginTop: 4 }} onClick={() => setModal({ type: 'add' })}>
                                Add Your First Provider
                            </button>
                        )}
                    </div>
                ) : (
                    <div style={s.providerList}>
                        {providers.map(p => (
                            <ProviderCard
                                key={p.id}
                                provider={p}
                                isAdmin={isAdmin}
                                testingId={testingId}
                                testResult={testResults[p.id]}
                                onTest={handleTestConnection}
                                onSetDefault={handleSetDefault}
                                onEdit={(prov) => setModal({ type: 'edit', provider: prov })}
                                onDelete={(prov) => setConfirmDelete(prov)}
                            />
                        ))}
                    </div>
                )}
            </section>

            {/* ── Modals ── */}
            {modal && (
                <ProviderModal
                    provider={modal.type === 'edit' ? modal.provider : null}
                    onClose={() => setModal(null)}
                    onSaved={() => { setModal(null); loadProviders(); }}
                />
            )}
            {confirmDelete && (
                <DeleteModal
                    provider={confirmDelete}
                    onCancel={() => setConfirmDelete(null)}
                    onConfirm={handleDeleteExecute}
                />
            )}
        </>
    );
}
