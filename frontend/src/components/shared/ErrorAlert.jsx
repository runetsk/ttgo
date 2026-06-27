import React from 'react';

export default function ErrorAlert({ message }) {
    if (!message) return null;
    return (
        <div style={{
            background: 'rgba(239,68,68,0.1)',
            color: 'var(--accent-red, #f87171)',
            padding: '8px 12px',
            borderRadius: 6,
            marginBottom: 14,
            fontSize: '0.875rem',
        }}>
            {message}
        </div>
    );
}
