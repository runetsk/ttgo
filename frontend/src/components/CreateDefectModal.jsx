import React, { useState, useEffect } from 'react';
import { defectLinks, jira } from '../api';

/**
 * CreateDefectModal
 *
 * Modal for creating a new Jira issue linked to a failed test result (US1, FR-001).
 * Pre-fills summary from test name; description from error_message/stack_trace.
 * Uses JiraConfig defaults for project key and issue type (FR-001).
 *
 * Props:
 *   testCaseId   {string}   — test case to link the new issue to
 *   testName     {string}   — pre-fill for summary
 *   errorMessage {string}   — pre-fill for description (actual result)
 *   stackTrace   {string}   — appended to description
 *   onClose      {function} — called to close modal (no args)
 *   onCreated    {function} — called with new DefectLink after successful creation
 */
export default function CreateDefectModal({ testCaseId, testName, errorMessage, stackTrace, onClose, onCreated }) {
    const [summary, setSummary] = useState(`[Defect] ${testName || 'Test failure'}`);
    const [description, setDescription] = useState(() => {
        const actual = errorMessage || stackTrace || '';
        return actual ? `Actual result: ${actual}` : 'Actual result: (not recorded)';
    });
    const [projectKey, setProjectKey] = useState('');
    const [issueType, setIssueType] = useState('Bug');
    const [submitting, setSubmitting] = useState(false);
    const [configLoaded, setConfigLoaded] = useState(false);

    // Load JiraConfig defaults (FR-001: pre-fill project key + issue type).
    useEffect(() => {
        jira.getConfig()
            .then(cfg => {
                if (cfg?.default_project_key) setProjectKey(cfg.default_project_key);
                if (cfg?.default_issue_type) setIssueType(cfg.default_issue_type || 'Bug');
            })
            .catch(() => {})
            .finally(() => setConfigLoaded(true));
    }, []);

    const handleSubmit = (e) => {
        e.preventDefault();
        if (!summary.trim()) return;
        setSubmitting(true);
        defectLinks.createIssue(testCaseId, {
            summary: summary.trim(),
            description: description.trim(),
            project_key: projectKey.trim(),
            issue_type: issueType.trim() || 'Bug',
        })
            .then(link => {
                onCreated?.(link);
                onClose();
            })
            .catch(() => {})
            .finally(() => setSubmitting(false));
    };

    return (
        <div
            style={{
                position: 'fixed', inset: 0, zIndex: 1000,
                background: 'rgba(0,0,0,0.6)',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}
            onClick={e => { if (e.target === e.currentTarget) onClose(); }}
        >
            <div className="glass-panel" style={{ width: '100%', maxWidth: 520, padding: 24, borderRadius: 10, position: 'relative' }}>
                <h3 style={{ margin: '0 0 16px', fontSize: '1rem' }}>Create Jira Defect</h3>

                <form onSubmit={handleSubmit}>
                    <label style={labelStyle}>Summary *</label>
                    <input
                        className="modern-input"
                        style={{ width: '100%', marginBottom: 12 }}
                        value={summary}
                        onChange={e => setSummary(e.target.value)}
                        required
                        disabled={submitting}
                    />

                    <label style={labelStyle}>Description</label>
                    <textarea
                        className="modern-input"
                        style={{ width: '100%', marginBottom: 12, minHeight: 90, resize: 'vertical' }}
                        value={description}
                        onChange={e => setDescription(e.target.value)}
                        disabled={submitting}
                    />

                    <div style={{ display: 'flex', gap: 10, marginBottom: 12 }}>
                        <div style={{ flex: 1 }}>
                            <label style={labelStyle}>Project Key *</label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder="e.g. PROJ"
                                value={projectKey}
                                onChange={e => setProjectKey(e.target.value.toUpperCase())}
                                required
                                disabled={submitting || !configLoaded}
                            />
                        </div>
                        <div style={{ flex: 1 }}>
                            <label style={labelStyle}>Issue Type</label>
                            <input
                                className="modern-input"
                                style={{ width: '100%' }}
                                placeholder="Bug"
                                value={issueType}
                                onChange={e => setIssueType(e.target.value)}
                                disabled={submitting || !configLoaded}
                            />
                        </div>
                    </div>

                    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 8 }}>
                        <button
                            type="button"
                            className="action-btn"
                            onClick={onClose}
                            disabled={submitting}
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            className="primary-btn"
                            disabled={submitting || !summary.trim() || !projectKey.trim()}
                            style={{ opacity: submitting ? 0.6 : 1 }}
                        >
                            {submitting ? 'Creating…' : 'Create Defect'}
                        </button>
                    </div>
                </form>
            </div>
        </div>
    );
}

const labelStyle = {
    display: 'block',
    fontSize: '0.78rem',
    color: 'var(--text-secondary)',
    marginBottom: 4,
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
};
