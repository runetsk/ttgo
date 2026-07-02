import React, { useState, useEffect } from 'react';
import { useAuth } from '../contexts/AuthContext';
import {
    listRunComments, addRunComment,
    listResultComments, addResultComment,
    updateComment, deleteComment
} from '../api';

const CommentsPanel = ({ targetType, runId, resultId, compact, onCountChange }) => {
    const { user } = useAuth();
    const [comments, setComments] = useState([]);
    const [newContent, setNewContent] = useState('');
    const [editingId, setEditingId] = useState(null);
    const [editContent, setEditContent] = useState('');
    const [loading, setLoading] = useState(true);
    // "Now" for relative-time labels, captured when comments are (re)loaded
    // rather than read inline during render (Date.now() must not be called
    // in render). Refreshed on every fetch, matching how often the comment
    // list itself — and therefore the labels — actually change.
    const [now, setNow] = useState(() => Date.now());

    const fetchComments = () => {
        setLoading(true);
        setComments([]);
        const fetcher = targetType === 'run'
            ? listRunComments(runId)
            : listResultComments(runId, resultId);
        fetcher.then(data => {
            const list = data || [];
            setComments(list);
            setNow(Date.now());
            if (onCountChange) onCountChange(list.length);
        }).catch(() => {})
          .finally(() => setLoading(false));
    };

    useEffect(() => {
        // eslint-disable-next-line react-hooks/set-state-in-effect -- pre-existing pattern (fetchComments synchronously sets loading/comments state), unmasked by the purity fix above; out of scope for this task (owned by the set-state-in-effect cleanup)
        fetchComments();
    }, [targetType, runId, resultId]);

    const handleAdd = () => {
        const trimmed = newContent.trim();
        if (!trimmed || trimmed.length > 2000) return;
        const adder = targetType === 'run'
            ? addRunComment(runId, trimmed)
            : addResultComment(runId, resultId, trimmed);
        adder.then(() => { setNewContent(''); fetchComments(); }).catch(() => {});
    };

    const handleUpdate = (id) => {
        const trimmed = editContent.trim();
        if (!trimmed || trimmed.length > 2000) return;
        updateComment(id, trimmed).then(() => { setEditingId(null); fetchComments(); }).catch(() => {});
    };

    const handleDelete = (id) => {
        if (!window.confirm('Delete this comment?')) return;
        deleteComment(id).then(() => fetchComments()).catch(() => {});
    };

    const canModify = (comment) =>
        user && (user.role === 'admin' || user.id === comment.user_id);

    const relativeTime = (isoStr, nowMs) => {
        const diff = nowMs - new Date(isoStr).getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'just now';
        if (mins < 60) return `${mins}m ago`;
        const hrs = Math.floor(mins / 60);
        if (hrs < 24) return `${hrs}h ago`;
        return `${Math.floor(hrs / 24)}d ago`;
    };

    const avatarColors = ['rgba(99,102,241,0.2)', 'rgba(34,197,94,0.2)', 'rgba(245,158,11,0.2)', 'rgba(239,68,68,0.2)', 'rgba(14,184,166,0.2)'];
    const textColors = ['#818cf8', '#22c55e', '#f59e0b', '#ef4444', '#14b8a6'];
    const colorIndex = (name) => {
        let hash = 0;
        for (let i = 0; i < (name || '').length; i++) hash = name.charCodeAt(i) + ((hash << 5) - hash);
        return Math.abs(hash) % avatarColors.length;
    };

    if (loading) return <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', padding: '4px 0' }}>Loading comments...</div>;

    return (
        <div>
            {comments.map(c => (
                <div key={c.id} style={{ display: 'flex', gap: 8, marginBottom: 10 }}>
                    <div style={{
                        width: compact ? 20 : 24, height: compact ? 20 : 24, borderRadius: '50%',
                        background: avatarColors[colorIndex(c.user_display_name)],
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontSize: compact ? '0.6rem' : '0.65rem', color: textColors[colorIndex(c.user_display_name)],
                        flexShrink: 0, marginTop: 2,
                    }}>
                        {(c.user_display_name || 'A')[0].toUpperCase()}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginBottom: 2 }}>
                            <span style={{ fontSize: compact ? '0.68rem' : '0.72rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                                {c.user_display_name || 'API'}
                            </span>
                            <span style={{ fontSize: compact ? '0.62rem' : '0.65rem', color: 'var(--text-secondary)', opacity: 0.6 }}>
                                {relativeTime(c.created_at, now)}
                            </span>
                            {canModify(c) && editingId !== c.id && (
                                <span style={{ marginLeft: 'auto', display: 'flex', gap: 4 }}>
                                    <button onClick={() => { setEditingId(c.id); setEditContent(c.content); }}
                                        style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '0.65rem', color: 'var(--text-secondary)', padding: 0 }}>
                                        ✏️
                                    </button>
                                    <button onClick={() => handleDelete(c.id)}
                                        style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '0.65rem', color: 'var(--text-secondary)', padding: 0 }}>
                                        🗑️
                                    </button>
                                </span>
                            )}
                        </div>
                        {editingId === c.id ? (
                            <div style={{ display: 'flex', gap: 6, alignItems: 'flex-start' }}>
                                <textarea value={editContent} onChange={e => setEditContent(e.target.value)}
                                    style={{
                                        flex: 1, background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)',
                                        borderRadius: 4, padding: '6px 8px', color: 'var(--text-primary)',
                                        fontSize: '0.75rem', fontFamily: 'inherit', resize: 'vertical', minHeight: 36,
                                    }}
                                />
                                <button onClick={() => handleUpdate(c.id)}
                                    style={{ background: 'var(--accent-indigo)', color: 'white', border: 'none', borderRadius: 4, padding: '6px 10px', fontSize: '0.7rem', fontWeight: 600, cursor: 'pointer' }}>
                                    Save
                                </button>
                                <button onClick={() => setEditingId(null)}
                                    style={{ background: 'none', border: '1px solid var(--border-color)', borderRadius: 4, padding: '6px 10px', fontSize: '0.7rem', color: 'var(--text-secondary)', cursor: 'pointer' }}>
                                    Cancel
                                </button>
                            </div>
                        ) : (
                            <div style={{ fontSize: compact ? '0.72rem' : '0.78rem', color: 'var(--text-secondary)', lineHeight: 1.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
                                {c.content}
                            </div>
                        )}
                    </div>
                </div>
            ))}

            {comments.length === 0 && (
                <div style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', opacity: 0.6, fontStyle: 'italic', marginBottom: 8 }}>
                    No comments yet.
                </div>
            )}

            {/* Add comment input */}
            <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start', paddingTop: comments.length > 0 ? 8 : 0, borderTop: comments.length > 0 ? '1px solid var(--border-color)' : 'none' }}>
                {compact ? (
                    <input
                        value={newContent} onChange={e => setNewContent(e.target.value)}
                        onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleAdd(); } }}
                        placeholder="Add a comment..."
                        style={{
                            flex: 1, background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)',
                            borderRadius: 4, padding: '6px 8px', color: 'var(--text-primary)',
                            fontSize: '0.72rem', fontFamily: 'inherit', outline: 'none',
                        }}
                    />
                ) : (
                    <textarea
                        value={newContent} onChange={e => setNewContent(e.target.value)}
                        placeholder="Add a comment..."
                        rows={2}
                        style={{
                            flex: 1, background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)',
                            borderRadius: 6, padding: '8px 10px', color: 'var(--text-primary)',
                            fontSize: '0.78rem', fontFamily: 'inherit', resize: 'vertical',
                            minHeight: 36, maxHeight: 120, outline: 'none',
                        }}
                    />
                )}
                <button onClick={handleAdd} disabled={!newContent.trim()}
                    style={{
                        background: newContent.trim() ? 'var(--accent-indigo)' : 'var(--bg-tertiary)',
                        color: newContent.trim() ? 'white' : 'var(--text-secondary)',
                        border: 'none', borderRadius: compact ? 4 : 6,
                        padding: compact ? '6px 10px' : '8px 14px',
                        fontSize: compact ? '0.72rem' : '0.78rem', fontWeight: 600, cursor: 'pointer',
                        whiteSpace: 'nowrap', opacity: newContent.trim() ? 1 : 0.5,
                    }}>
                    Post
                </button>
            </div>
            {newContent.length > 1800 && (
                <div style={{ fontSize: '0.65rem', color: newContent.length > 2000 ? 'var(--accent-red)' : 'var(--warning-color)', marginTop: 4 }}>
                    {newContent.length}/2000
                </div>
            )}
        </div>
    );
};

export default CommentsPanel;
