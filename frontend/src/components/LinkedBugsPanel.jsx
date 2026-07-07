import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { testCaseDefects } from '../api';
import { activeBugs, bugHref, severityChipStyle } from '../utils/bugs';

// Read-only panel listing the test case's linked ACTIVE (open) bugs, each
// navigable. Renders nothing when there are no active bugs.
export default function LinkedBugsPanel({ testCaseId }) {
    const [bugs, setBugs] = useState([]);

    useEffect(() => {
        if (!testCaseId) return;
        let cancelled = false;
        testCaseDefects.list(testCaseId)
            .then(data => { if (!cancelled) setBugs(Array.isArray(data) ? data : []); })
            .catch(() => { if (!cancelled) setBugs([]); });
        return () => { cancelled = true; };
    }, [testCaseId]);

    const active = activeBugs(bugs);
    if (active.length === 0) return null;

    const rowStyle = {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '6px 10px', borderRadius: 6,
        background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.25)',
        textDecoration: 'none', color: 'var(--text-primary)', fontSize: '0.85rem',
    };

    return (
        <div style={{ marginTop: 24 }} data-testid="linked-bugs-panel">
            <h4 style={{ margin: '0 0 10px', color: 'var(--text-secondary)', fontSize: '0.85rem', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                Active Bugs <span style={{ opacity: 0.6 }}>({active.length})</span>
            </h4>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                {active.map(d => {
                    const { href, external } = bugHref(d);
                    const inner = (
                        <>
                            <span style={severityChipStyle(d.severity)}>{d.severity}</span>
                            <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{d.title}</span>
                            {external && (
                                <span style={{ color: 'var(--text-secondary)', fontSize: '0.8rem', flexShrink: 0 }}>
                                    {d.external_key || 'link'} ↗
                                </span>
                            )}
                        </>
                    );
                    return external ? (
                        <a key={d.id} href={href} target="_blank" rel="noopener noreferrer"
                           data-testid={`linked-bug-${d.id}`} style={rowStyle}>
                            {inner}
                        </a>
                    ) : (
                        <Link key={d.id} to={href} data-testid={`linked-bug-${d.id}`} style={rowStyle}>
                            {inner}
                        </Link>
                    );
                })}
            </div>
        </div>
    );
}
