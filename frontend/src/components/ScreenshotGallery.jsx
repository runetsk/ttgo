import React, { useState, useEffect, useCallback } from 'react';
import ModalShell from './shared/ModalShell';

export default function ScreenshotGallery({ screenshots, initialIndex = 0, onClose }) {
    const [index, setIndex] = useState(initialIndex);

    const goNext = useCallback(() => setIndex(i => Math.min(i + 1, screenshots.length - 1)), [screenshots.length]);
    const goPrev = useCallback(() => setIndex(i => Math.max(i - 1, 0)), []);

    useEffect(() => {
        const handleKey = (e) => {
            if (e.key === 'ArrowRight') goNext();
            else if (e.key === 'ArrowLeft') goPrev();
            else if (e.key === 'Escape') onClose();
        };
        window.addEventListener('keydown', handleKey);
        return () => window.removeEventListener('keydown', handleKey);
    }, [goNext, goPrev, onClose]);

    const url = screenshots[index];
    const caption = `Screenshot ${index + 1} of ${screenshots.length}`;
    const frameStyle = {
        display: 'inline-flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: 12,
        background: 'linear-gradient(180deg, rgba(255,255,255,0.04), rgba(255,255,255,0.02))',
        border: '1px solid var(--border-color)',
        borderRadius: 10,
        boxShadow: 'inset 0 0 0 1px rgba(255,255,255,0.04), 0 14px 30px rgba(0,0,0,0.28)',
    };

    return (
        <ModalShell
            title="Screenshots"
            subtitle={caption}
            width={900}
            maxHeight="90vh"
            onClose={onClose}
            footer={
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
                    <button
                        onClick={goPrev}
                        disabled={index === 0}
                        style={{
                            padding: '6px 16px',
                            background: index === 0 ? 'var(--bg-tertiary)' : 'var(--bg-secondary)',
                            color: index === 0 ? 'var(--text-secondary)' : 'var(--text-primary)',
                            border: '1px solid var(--border-color)',
                            borderRadius: 6,
                            cursor: index === 0 ? 'default' : 'pointer',
                        }}
                    >
                        Previous
                    </button>
                    <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>{caption}</span>
                    <button
                        onClick={goNext}
                        disabled={index === screenshots.length - 1}
                        style={{
                            padding: '6px 16px',
                            background: index === screenshots.length - 1 ? 'var(--bg-tertiary)' : 'var(--bg-secondary)',
                            color: index === screenshots.length - 1 ? 'var(--text-secondary)' : 'var(--text-primary)',
                            border: '1px solid var(--border-color)',
                            borderRadius: 6,
                            cursor: index === screenshots.length - 1 ? 'default' : 'pointer',
                        }}
                    >
                        Next
                    </button>
                </div>
            }
        >
            <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 300 }}>
                <div style={frameStyle}>
                    <img
                        src={url}
                        alt={caption}
                        style={{
                            display: 'block',
                            maxWidth: '100%',
                            maxHeight: '70vh',
                            objectFit: 'contain',
                            borderRadius: 6,
                            border: '1px solid rgba(255,255,255,0.08)',
                            boxShadow: '0 0 0 1px rgba(0,0,0,0.28)',
                            background: 'var(--bg-primary)',
                        }}
                    />
                </div>
            </div>
        </ModalShell>
    );
}
