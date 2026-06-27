import React from 'react';

export default function SourceBadge({ sourceType, sourceUrl, label }) {
    if (!sourceType) return null;
    const displayLabel = label || (sourceType === 'jira' ? 'Jira' : sourceType === 'confluence' ? 'Confluence' : sourceType);
    const color = sourceType === 'jira' ? 'var(--accent-blue, #60a5fa)' : 'var(--accent-teal, #2dd4bf)';
    return (
        <a
            href={sourceUrl || '#'}
            target="_blank"
            rel="noopener noreferrer"
            style={{
                display: 'inline-flex', alignItems: 'center', gap: 3,
                fontSize: '0.7rem', fontWeight: 600, padding: '1px 6px', borderRadius: 4,
                background: `${color}18`, color, border: `1px solid ${color}40`,
                textDecoration: 'none', whiteSpace: 'nowrap', marginLeft: 6,
            }}
            title={`Imported from ${displayLabel} — open source`}
            onClick={e => { if (!sourceUrl) e.preventDefault(); }}
        >
            🔗 {displayLabel}
        </a>
    );
}
