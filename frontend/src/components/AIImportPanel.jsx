import React, { useState, useRef } from 'react';
import { useAIGeneration } from '../contexts/AIGenerationContext';

const SUPPORTED_EXTENSIONS = ['.txt', '.md', '.csv', '.json'];
const MAX_FILE_SIZE = 1 * 1024 * 1024; // 1 MB

const FORMAT_OPTIONS = [
    { value: '', label: 'Auto-detect' },
    { value: 'json', label: 'JSON' },
    { value: 'csv', label: 'CSV' },
    { value: 'markdown_table', label: 'Markdown Table' },
    { value: 'numbered_list', label: 'Numbered List' },
];

const FORMAT_LABELS = {
    json: 'JSON',
    csv: 'CSV',
    markdown_table: 'Markdown Table',
    numbered_list: 'Numbered List',
    ai: 'AI-Parsed',
};

export default function AIImportPanel({ onParsed }) {
    const ai = useAIGeneration();
    const [rawContent, setRawContent] = useState('');
    const [formatHint, setFormatHint] = useState('');
    const [fileName, setFileName] = useState('');
    const [fileSize, setFileSize] = useState(0);
    const [fileError, setFileError] = useState('');
    const [dragOver, setDragOver] = useState(false);
    const fileInputRef = useRef(null);

    const canParse = rawContent.trim().length > 0 && !ai.importParsing;
    const charCount = rawContent.length;

    const handleParse = async () => {
        if (!canParse) return;
        try {
            await ai.parseImport(rawContent, formatHint, ai.selectedFolderId || '');
            onParsed?.();
        } catch {
            // Error already set in context
        }
    };

    const handleFileSelect = (file) => {
        setFileError('');
        if (!file) return;

        const ext = '.' + file.name.split('.').pop().toLowerCase();
        if (!SUPPORTED_EXTENSIONS.includes(ext)) {
            setFileError(`Unsupported file type "${ext}". Supported: ${SUPPORTED_EXTENSIONS.join(', ')}`);
            return;
        }
        if (file.size > MAX_FILE_SIZE) {
            setFileError(`File too large (${(file.size / 1024 / 1024).toFixed(1)} MB). Maximum: 1 MB.`);
            return;
        }

        const reader = new FileReader();
        reader.onload = (e) => {
            setRawContent(e.target.result);
            setFileName(file.name);
            setFileSize(file.size);
        };
        reader.onerror = () => setFileError('Failed to read file');
        reader.readAsText(file);
    };

    const clearFile = () => {
        setFileName('');
        setFileSize(0);
        setFileError('');
        if (fileInputRef.current) fileInputRef.current.value = '';
    };

    const handleDrop = (e) => {
        e.preventDefault();
        setDragOver(false);
        const file = e.dataTransfer?.files?.[0];
        if (file) handleFileSelect(file);
    };

    const handleKeyDown = (e) => {
        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && canParse) {
            e.preventDefault();
            handleParse();
        }
    };

    return (
        <div style={styles.root}>
            {/* Header */}
            <div style={styles.header}>
                <div style={styles.headerTop}>
                    <h3 style={styles.title}>Import AI-Generated Test Cases</h3>
                    {ai.importFormat && (
                        <span style={{
                            ...styles.detectedBadge,
                            ...(ai.importFormat === 'ai' ? {
                                background: 'rgba(168,85,247,0.1)',
                                color: '#a855f7',
                                border: '1px solid rgba(168,85,247,0.2)',
                            } : {}),
                        }}>
                            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="20 6 9 17 4 12"/>
                            </svg>
                            {ai.importFormat === 'ai' ? 'Parsed by AI' : `Detected: ${FORMAT_LABELS[ai.importFormat] || ai.importFormat}`}
                        </span>
                    )}
                </div>
                <p style={styles.subtitle}>
                    Paste output from ChatGPT, Gemini, Claude, or any LLM — or upload a file.
                </p>
            </div>

            {/* File metadata bar */}
            {fileName && (
                <div style={styles.fileMeta}>
                    <span style={styles.fileIcon}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>
                        </svg>
                    </span>
                    <span style={styles.fileName}>{fileName}</span>
                    <span style={styles.fileSize}>({(fileSize / 1024).toFixed(1)} KB)</span>
                    <button onClick={clearFile} style={styles.clearFileBtn} className="import-clear-file" title="Clear file">
                        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
                            <line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
                        </svg>
                    </button>
                </div>
            )}

            {/* Textarea + drop zone */}
            <div
                style={{ ...styles.textareaWrap, ...(dragOver ? styles.textareaWrapDragOver : {}) }}
                onDragOver={(e) => { e.preventDefault(); setDragOver(true); }}
                onDragLeave={() => setDragOver(false)}
                onDrop={handleDrop}
            >
                <textarea
                    value={rawContent}
                    onChange={(e) => setRawContent(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={`Paste AI-generated test cases here...\n\nSupported formats:\n• JSON array: [{"name":"...", "steps":[...]}]\n• Numbered list: 1. Test name\\n   - Step action → Expected result\n• Markdown table: | Name | Action | Expected Result |\n• CSV: name,action,expected_result`}
                    style={styles.textarea}
                    rows={12}
                />
                {!rawContent && !dragOver && (
                    <div style={styles.dropHint}>
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" style={{ opacity: 0.4 }}>
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                        </svg>
                        <span>or drag & drop a file here</span>
                    </div>
                )}
                {dragOver && (
                    <div style={styles.dropActiveOverlay}>
                        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                        </svg>
                        <span>Drop file to import</span>
                    </div>
                )}
                {rawContent && (
                    <div style={styles.charCount}>{charCount.toLocaleString()} chars</div>
                )}
            </div>

            {/* Controls bar */}
            <div style={styles.controlsBar}>
                <div style={styles.leftControls}>
                    <input
                        ref={fileInputRef}
                        type="file"
                        accept={SUPPORTED_EXTENSIONS.join(',')}
                        onChange={(e) => handleFileSelect(e.target.files?.[0])}
                        style={{ display: 'none' }}
                    />
                    <button
                        onClick={() => fileInputRef.current?.click()}
                        style={styles.uploadBtn}
                        className="import-upload-btn"
                    >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>
                        </svg>
                        Upload File
                    </button>

                    <div style={styles.formatWrap}>
                        <label style={styles.formatLabel}>Format</label>
                        <select
                            value={formatHint}
                            onChange={(e) => setFormatHint(e.target.value)}
                            style={styles.formatSelect}
                            className="modern-select"
                        >
                            {FORMAT_OPTIONS.map(opt => (
                                <option key={opt.value} value={opt.value}>{opt.label}</option>
                            ))}
                        </select>
                    </div>
                </div>

                <button
                    onClick={handleParse}
                    disabled={!canParse}
                    className="import-parse-btn"
                    style={{ ...styles.parseBtn, ...(canParse ? {} : styles.parseBtnDisabled) }}
                >
                    {ai.importParsing ? (
                        <>
                            <span style={styles.spinner} />
                            Parsing…
                        </>
                    ) : (
                        <>
                            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                                <polyline points="4 7 4 4 20 4 20 7"/><line x1="9" y1="20" x2="15" y2="20"/><line x1="12" y1="4" x2="12" y2="20"/>
                            </svg>
                            Parse
                            {canParse && <span style={styles.parseShortcut}>⌘↵</span>}
                        </>
                    )}
                </button>
            </div>

            {/* Errors */}
            {fileError && (
                <div style={styles.error}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
                        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <span>{fileError}</span>
                </div>
            )}
            {ai.importError && (
                <div style={styles.error}>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0 }}>
                        <circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/>
                    </svg>
                    <span>{ai.importError}</span>
                </div>
            )}

            <style>{`
                @keyframes spin { to { transform: rotate(360deg); } }
                .import-upload-btn:hover {
                    border-color: rgba(99,102,241,0.4) !important;
                    background: rgba(99,102,241,0.06) !important;
                }
                .import-parse-btn:not(:disabled):hover {
                    filter: brightness(1.1);
                    transform: translateY(-1px);
                    box-shadow: 0 6px 20px rgba(99,102,241,0.35) !important;
                }
                .import-parse-btn:not(:disabled):active {
                    transform: translateY(0);
                }
                .import-clear-file:hover {
                    color: var(--text-primary) !important;
                    background: rgba(255,255,255,0.08) !important;
                }
            `}</style>
        </div>
    );
}

const styles = {
    root: {
        display: 'flex', flexDirection: 'column', gap: 14,
        padding: '20px 0', maxWidth: 900,
    },
    header: { marginBottom: 2 },
    headerTop: {
        display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap',
    },
    title: {
        margin: 0, fontSize: '1.1rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.01em',
    },
    subtitle: {
        margin: '4px 0 0', fontSize: '0.84rem',
        color: 'var(--text-secondary)', lineHeight: 1.5,
    },
    detectedBadge: {
        display: 'inline-flex', alignItems: 'center', gap: 4,
        padding: '3px 10px', borderRadius: 20, fontSize: '0.72rem', fontWeight: 600,
        background: 'rgba(34,197,94,0.1)', color: '#22c55e',
        border: '1px solid rgba(34,197,94,0.2)',
    },
    fileMeta: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '8px 12px', borderRadius: 8,
        background: 'rgba(99,102,241,0.06)',
        border: '1px solid rgba(99,102,241,0.15)',
        fontSize: '0.82rem',
    },
    fileIcon: { display: 'flex', color: '#818cf8' },
    fileName: { fontWeight: 600, color: 'var(--text-primary)' },
    fileSize: { color: 'var(--text-secondary)', fontSize: '0.78rem' },
    clearFileBtn: {
        marginLeft: 'auto', background: 'none', border: 'none',
        color: 'var(--text-secondary)', cursor: 'pointer',
        padding: '4px 6px', borderRadius: 4, display: 'flex',
        transition: 'all 0.15s',
    },
    textareaWrap: {
        position: 'relative', borderRadius: 10,
        border: '1px solid var(--border-color, var(--border-primary))',
        background: 'var(--bg-primary)',
        transition: 'border-color 0.2s, box-shadow 0.2s',
        overflow: 'hidden',
    },
    textareaWrapDragOver: {
        borderColor: '#6366f1',
        borderStyle: 'dashed',
        boxShadow: '0 0 0 3px rgba(99,102,241,0.12), inset 0 0 40px rgba(99,102,241,0.04)',
    },
    textarea: {
        width: '100%', boxSizing: 'border-box', padding: '14px 16px',
        background: 'transparent', color: 'var(--text-primary)',
        border: 'none', resize: 'vertical',
        fontSize: '0.84rem', fontFamily: "'SF Mono', 'Fira Code', 'Cascadia Code', monospace",
        lineHeight: 1.6, outline: 'none', minHeight: 220,
    },
    dropHint: {
        position: 'absolute', bottom: 14, left: 0, right: 0,
        display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
        pointerEvents: 'none', fontSize: '0.78rem',
        color: 'var(--text-secondary)', opacity: 0.6,
    },
    dropActiveOverlay: {
        position: 'absolute', inset: 0,
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 8,
        background: 'rgba(99,102,241,0.06)',
        color: '#818cf8', fontSize: '0.9rem', fontWeight: 600,
        pointerEvents: 'none',
    },
    charCount: {
        position: 'absolute', bottom: 8, right: 12,
        fontSize: '0.68rem', color: 'var(--text-secondary)', opacity: 0.5,
        fontFamily: 'monospace', pointerEvents: 'none',
    },
    controlsBar: {
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        gap: 12, flexWrap: 'wrap',
    },
    leftControls: {
        display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap',
    },
    uploadBtn: {
        display: 'flex', alignItems: 'center', gap: 6,
        padding: '8px 14px', borderRadius: 8, fontSize: '0.82rem', fontWeight: 500,
        background: 'var(--bg-secondary, rgba(255,255,255,0.06))',
        color: 'var(--text-primary)',
        border: '1px solid var(--border-color, var(--border-primary))',
        cursor: 'pointer', fontFamily: 'inherit',
        transition: 'all 0.15s',
    },
    formatWrap: {
        display: 'flex', alignItems: 'center', gap: 6,
    },
    formatLabel: {
        fontSize: '0.72rem', fontWeight: 600, color: 'var(--text-secondary)',
        textTransform: 'uppercase', letterSpacing: '0.04em',
    },
    formatSelect: {
        padding: '6px 10px', borderRadius: 6, fontSize: '0.82rem',
        background: 'var(--bg-secondary)', color: 'var(--text-primary)',
        border: '1px solid var(--border-color, var(--border-primary))',
        cursor: 'pointer', fontFamily: 'inherit',
    },
    parseBtn: {
        display: 'flex', alignItems: 'center', gap: 7,
        padding: '9px 22px', borderRadius: 9, fontSize: '0.88rem',
        background: 'linear-gradient(135deg, #4338ca 0%, #6366f1 100%)',
        color: '#fff', border: 'none',
        cursor: 'pointer', fontWeight: 700, fontFamily: 'inherit',
        boxShadow: '0 3px 14px rgba(99,102,241,0.3)',
        transition: 'all 0.18s cubic-bezier(0.16, 1, 0.3, 1)',
        letterSpacing: '0.01em',
    },
    parseBtnDisabled: {
        opacity: 0.4, cursor: 'not-allowed',
        boxShadow: 'none',
    },
    parseShortcut: {
        fontSize: '0.68rem', opacity: 0.55,
        padding: '1px 5px', borderRadius: 3,
        background: 'rgba(255,255,255,0.15)',
        fontFamily: 'system-ui',
    },
    spinner: {
        display: 'inline-block', width: 14, height: 14,
        border: '2px solid rgba(255,255,255,0.3)', borderTopColor: '#fff',
        borderRadius: '50%', animation: 'spin 0.6s linear infinite',
    },
    error: {
        display: 'flex', alignItems: 'center', gap: 8,
        padding: '10px 14px', borderRadius: 8, fontSize: '0.82rem',
        background: 'rgba(239,68,68,0.07)',
        color: '#f87171',
        border: '1px solid rgba(239,68,68,0.2)',
        lineHeight: 1.4,
    },
};
