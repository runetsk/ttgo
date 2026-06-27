import React, { useState } from 'react';

// ─── Category ordering ───────────────────────────────────────────────────────

const CATEGORY_ORDER = [
    'Functional', 'Negative', 'Boundary', 'Edge Case',
    'Security', 'Performance', 'API', 'Mobile/Responsive', 'Accessibility',
];
const OTHER_CATEGORY = 'Other';

// ─── Grouping logic ──────────────────────────────────────────────────────────

function groupDraftsByCategory(drafts) {
    const groups = new Map();
    const orderLower = CATEGORY_ORDER.map(c => c.toLowerCase());

    for (const draft of drafts) {
        const raw = (draft.category || '').trim();
        const lowerRaw = raw.toLowerCase();
        const knownIdx = orderLower.indexOf(lowerRaw);
        const cat = knownIdx !== -1 ? CATEGORY_ORDER[knownIdx] : (raw || OTHER_CATEGORY);

        if (!groups.has(cat)) groups.set(cat, []);
        groups.get(cat).push(draft);
    }

    const ordered = [];
    for (const cat of CATEGORY_ORDER) {
        if (groups.has(cat)) {
            ordered.push({ category: cat, drafts: groups.get(cat) });
            groups.delete(cat);
        }
    }
    const remaining = [...groups.entries()]
        .filter(([cat]) => cat !== OTHER_CATEGORY)
        .sort(([a], [b]) => a.localeCompare(b));
    for (const [cat, draftList] of remaining) {
        ordered.push({ category: cat, drafts: draftList });
    }
    if (groups.has(OTHER_CATEGORY)) {
        ordered.push({ category: OTHER_CATEGORY, drafts: groups.get(OTHER_CATEGORY) });
    }
    return ordered;
}

// ─── Panel ───────────────────────────────────────────────────────────────────

export default function AIGenReviewPanel({
    drafts,
    acceptedIds,
    onAccept,
    onDiscard,
    onAcceptAll,
    onDiscardAll,
    onAcceptGroup,
    onDiscardGroup,
    onEdit,
}) {
    const pendingDrafts = drafts.filter(d => !acceptedIds.has(d.temp_id));
    const [collapsedGroups, setCollapsedGroups] = useState(new Set());

    if (pendingDrafts.length === 0) {
        return (
            <div style={{
                textAlign: 'center', padding: '32px 0',
                color: 'var(--text-secondary)', fontSize: '0.9rem',
            }}>
                ✓ All drafts have been processed.
            </div>
        );
    }

    const groups = groupDraftsByCategory(pendingDrafts);

    const toggleGroup = (category) => {
        setCollapsedGroups(prev => {
            const next = new Set(prev);
            next.has(category) ? next.delete(category) : next.add(category);
            return next;
        });
    };

    const collapseAll = () => setCollapsedGroups(new Set(groups.map(g => g.category)));
    const expandAll   = () => setCollapsedGroups(new Set());
    const allCollapsed = groups.every(g => collapsedGroups.has(g.category));

    return (
        <div>
            {/* ── Global header ── */}
            <div style={{
                display: 'flex', alignItems: 'center', gap: 8,
                marginBottom: 14, flexWrap: 'wrap',
            }}>
                <span style={{
                    color: 'var(--text-secondary)', fontSize: '0.82rem',
                    marginRight: 'auto', display: 'flex', alignItems: 'center', gap: 6,
                }}>
                    <span style={{
                        background: 'var(--bg-tertiary)',
                        border: '1px solid var(--border-color)',
                        borderRadius: 20, padding: '1px 10px',
                        fontSize: '0.78rem', fontWeight: 700,
                        color: 'var(--text-primary)',
                    }}>
                        {pendingDrafts.length}
                    </span>
                    drafts in
                    <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
                        {groups.length} group{groups.length !== 1 ? 's' : ''}
                    </span>
                </span>

                <button
                    className="action-btn"
                    style={{ fontSize: '0.75rem', padding: '3px 10px', opacity: 0.85 }}
                    onClick={allCollapsed ? expandAll : collapseAll}
                >
                    {allCollapsed ? '↕ Expand all' : '↕ Collapse all'}
                </button>
                <div style={{ width: 1, height: 16, background: 'var(--border-color)', margin: '0 2px' }} />
                <button
                    className="action-btn"
                    style={{ fontSize: '0.78rem', padding: '4px 12px', color: 'var(--accent-red)' }}
                    onClick={onDiscardAll}
                >
                    Discard All
                </button>
                <button
                    className="primary-btn"
                    style={{ fontSize: '0.78rem', padding: '4px 14px' }}
                    onClick={onAcceptAll}
                >
                    Accept All
                </button>
            </div>

            {/* ── Groups ── */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {groups.map(group => (
                    <CategoryGroup
                        key={group.category}
                        category={group.category}
                        drafts={group.drafts}
                        collapsed={collapsedGroups.has(group.category)}
                        onToggle={() => toggleGroup(group.category)}
                        onAccept={onAccept}
                        onDiscard={onDiscard}
                        onAcceptGroup={onAcceptGroup}
                        onDiscardGroup={onDiscardGroup}
                        onEdit={onEdit}
                        allPending={pendingDrafts}
                    />
                ))}
            </div>
        </div>
    );
}

// ─── Category group ───────────────────────────────────────────────────────────

function CategoryGroup({
    category, drafts, collapsed, onToggle,
    onAccept, onDiscard, onAcceptGroup, onDiscardGroup, onEdit, allPending,
}) {
    const [hovered, setHovered] = useState(false);

    const handleAcceptGroup = (e) => {
        e.stopPropagation();
        onAcceptGroup ? onAcceptGroup(drafts) : drafts.forEach(d => onAccept(d));
    };
    const handleDiscardGroup = (e) => {
        e.stopPropagation();
        onDiscardGroup ? onDiscardGroup(drafts.map(d => d.temp_id)) : drafts.forEach(d => onDiscard(d.temp_id));
    };

    return (
        <div style={{
            borderRadius: 8,
            border: '1px solid var(--border-color)',
            overflow: 'hidden',
        }}>
            {/* ── Header ── */}
            <div
                onMouseEnter={() => setHovered(true)}
                onMouseLeave={() => setHovered(false)}
                onClick={onToggle}
                style={{
                    display: 'flex', alignItems: 'center', gap: 10,
                    padding: '8px 12px',
                    background: hovered ? 'var(--bg-tertiary)' : 'var(--bg-secondary)',
                    borderBottom: collapsed ? 'none' : '1px solid var(--border-color)',
                    cursor: 'pointer', userSelect: 'none',
                    transition: 'background 0.15s',
                }}
            >
                {/* Chevron */}
                <span style={{
                    fontSize: '0.65rem', color: 'var(--text-secondary)',
                    transition: 'transform 0.2s',
                    transform: collapsed ? 'rotate(-90deg)' : 'rotate(0deg)',
                    flexShrink: 0, display: 'inline-block',
                }}>
                    ▾
                </span>

                {/* Category label */}
                <span style={{
                    fontSize: '0.82rem', fontWeight: 600,
                    color: 'var(--text-primary)',
                    flexShrink: 0,
                }}>
                    {category}
                </span>

                {/* Count badge */}
                <span style={{
                    background: 'var(--bg-tertiary)',
                    border: '1px solid var(--border-color)',
                    color: 'var(--text-secondary)',
                    fontSize: '0.7rem', fontWeight: 600,
                    padding: '1px 7px', borderRadius: 10,
                    flexShrink: 0,
                }}>
                    {drafts.length}
                </span>

                {/* Collapsed preview pills */}
                {collapsed && (
                    <div style={{
                        display: 'flex', gap: 5, flex: 1, overflow: 'hidden',
                        alignItems: 'center',
                    }}>
                        {drafts.slice(0, 3).map(d => (
                            <span key={d.temp_id} style={{
                                fontSize: '0.72rem', color: 'var(--text-secondary)',
                                background: 'var(--bg-tertiary)',
                                border: '1px solid var(--border-color)',
                                borderRadius: 4, padding: '1px 7px',
                                maxWidth: 160, overflow: 'hidden',
                                textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                            }}>
                                {d.name || 'Untitled'}
                            </span>
                        ))}
                        {drafts.length > 3 && (
                            <span style={{ fontSize: '0.72rem', color: 'var(--text-secondary)', flexShrink: 0 }}>
                                +{drafts.length - 3} more
                            </span>
                        )}
                    </div>
                )}

                {/* Group actions */}
                <div
                    style={{ marginLeft: 'auto', display: 'flex', gap: 6, flexShrink: 0 }}
                    onClick={e => e.stopPropagation()}
                >
                    <button
                        className="action-btn"
                        style={{ fontSize: '0.72rem', padding: '3px 10px', color: 'var(--accent-red)' }}
                        onClick={handleDiscardGroup}
                    >
                        Discard
                    </button>
                    <button
                        className="primary-btn"
                        style={{ fontSize: '0.72rem', padding: '3px 10px' }}
                        onClick={handleAcceptGroup}
                    >
                        Accept
                    </button>
                </div>
            </div>

            {/* ── Draft cards ── */}
            {!collapsed && (
                <div>
                    {drafts.map((draft, idx) => {
                        const globalIdx = allPending.indexOf(draft) + 1;
                        return (
                            <DraftCard
                                key={draft.temp_id}
                                draft={draft}
                                index={globalIdx}
                                isLast={idx === drafts.length - 1}
                                onAccept={() => onAccept(draft)}
                                onDiscard={() => onDiscard(draft.temp_id)}
                                onEdit={(changes) => onEdit(draft.temp_id, changes)}
                            />
                        );
                    })}
                </div>
            )}
        </div>
    );
}

// ─── Draft card ───────────────────────────────────────────────────────────────

function DraftCard({ draft, index, isLast, onAccept, onDiscard, onEdit }) {
    const [expanded, setExpanded] = useState(true);
    const [editingName, setEditingName] = useState(false);
    const [editingDesc, setEditingDesc] = useState(false);
    const [name, setName] = useState(draft.name);
    const [desc, setDesc] = useState(draft.description);
    const [steps, setSteps] = useState(draft.steps || []);

    const commitName = () => { setEditingName(false); onEdit({ name, description: desc, steps }); };
    const commitDesc = () => { setEditingDesc(false); onEdit({ name, description: desc, steps }); };
    const updateStep = (idx, field, value) => {
        const updated = steps.map((s, i) => i === idx ? { ...s, [field]: value } : s);
        setSteps(updated);
        onEdit({ name, description: desc, steps: updated });
    };

    return (
        <div style={{
            padding: '12px 14px 12px 16px',
            borderBottom: isLast ? 'none' : '1px solid var(--border-color)',
        }}>
            {/* ── Card header ── */}
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
                {/* Index badge */}
                <span style={{
                    flexShrink: 0, width: 22, height: 22,
                    borderRadius: '50%',
                    background: 'var(--bg-tertiary)',
                    border: '1px solid var(--border-color)',
                    color: 'var(--text-secondary)',
                    fontSize: '0.68rem', fontWeight: 700,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    marginTop: 2,
                }}>
                    {index}
                </span>

                {/* Name */}
                <div style={{ flex: 1, minWidth: 0 }}>
                    {editingName ? (
                        <input
                            className="modern-input"
                            style={{ width: '100%', fontSize: '0.9rem', fontWeight: 600 }}
                            value={name}
                            onChange={e => setName(e.target.value)}
                            onBlur={commitName}
                            onKeyDown={e => e.key === 'Enter' && commitName()}
                            autoFocus
                        />
                    ) : (
                        <div
                            style={{
                                fontWeight: 600, fontSize: '0.9rem',
                                cursor: 'text', wordBreak: 'break-word',
                                display: 'flex', alignItems: 'baseline', gap: 5,
                            }}
                            onClick={() => setEditingName(true)}
                            title="Click to edit"
                        >
                            <span>{name || <span style={{ color: 'var(--text-secondary)', fontStyle: 'italic' }}>Untitled</span>}</span>
                            <span style={{ fontSize: '0.7rem', color: 'var(--text-secondary)', opacity: 0.7, flexShrink: 0 }}>✎</span>
                        </div>
                    )}
                </div>

                {/* Actions */}
                <div style={{ display: 'flex', gap: 5, flexShrink: 0, alignItems: 'center' }}>
                    <button
                        style={{
                            background: 'none', border: 'none', cursor: 'pointer',
                            color: 'var(--text-secondary)', padding: '2px 5px',
                            fontSize: '0.68rem', borderRadius: 4,
                            opacity: 0.7, transition: 'opacity 0.15s',
                        }}
                        onMouseEnter={e => e.currentTarget.style.opacity = '1'}
                        onMouseLeave={e => e.currentTarget.style.opacity = '0.7'}
                        onClick={() => setExpanded(e => !e)}
                        title={expanded ? 'Collapse' : 'Expand'}
                    >
                        {expanded ? '▲' : '▼'}
                    </button>
                    <button
                        className="action-btn"
                        style={{ fontSize: '0.73rem', padding: '3px 10px', color: 'var(--accent-red)' }}
                        onClick={onDiscard}
                    >
                        Discard
                    </button>
                    <button
                        className="primary-btn"
                        style={{ fontSize: '0.73rem', padding: '3px 10px' }}
                        onClick={onAccept}
                    >
                        Accept
                    </button>
                </div>
            </div>

            {/* ── Expanded body ── */}
            {expanded && (
                <div style={{ paddingLeft: 32, marginTop: 10 }}>
                    {/* Description */}
                    <div style={{ marginBottom: steps.length > 0 ? 10 : 0 }}>
                        <div style={sectionLabel}>Description</div>
                        {editingDesc ? (
                            <textarea
                                className="modern-input"
                                style={{ width: '100%', minHeight: 56, resize: 'vertical', fontSize: '0.85rem' }}
                                value={desc}
                                onChange={e => setDesc(e.target.value)}
                                onBlur={commitDesc}
                                autoFocus
                            />
                        ) : (
                            <div
                                style={{
                                    fontSize: '0.85rem', color: 'var(--text-primary)',
                                    cursor: 'text', lineHeight: 1.6,
                                    padding: '4px 6px', borderRadius: 5,
                                    border: '1px solid transparent',
                                    transition: 'border-color 0.15s, background 0.15s',
                                }}
                                onMouseEnter={e => {
                                    e.currentTarget.style.borderColor = 'var(--border-color)';
                                    e.currentTarget.style.background = 'var(--bg-tertiary)';
                                }}
                                onMouseLeave={e => {
                                    e.currentTarget.style.borderColor = 'transparent';
                                    e.currentTarget.style.background = 'transparent';
                                }}
                                onClick={() => setEditingDesc(true)}
                                title="Click to edit"
                            >
                                {desc
                                    ? <>{desc} <span style={{ fontSize: '0.7rem', opacity: 0.7 }}>✎</span></>
                                    : <span style={{ fontStyle: 'italic', opacity: 0.6 }}>No description — click to add</span>
                                }
                            </div>
                        )}
                    </div>

                    {/* Steps */}
                    {steps.length > 0 && (
                        <div>
                            <div style={sectionLabel}>
                                Steps
                                <span style={{
                                    marginLeft: 5, background: 'var(--bg-tertiary)',
                                    border: '1px solid var(--border-color)',
                                    borderRadius: 8, padding: '0px 5px',
                                    fontSize: '0.68rem', fontWeight: 600,
                                    color: 'var(--text-secondary)',
                                }}>
                                    {steps.length}
                                </span>
                            </div>
                            <div style={{
                                border: '1px solid var(--border-color)',
                                borderRadius: 6,
                                overflow: 'hidden',
                            }}>
                                {/* Column headers — shown once */}
                                <div style={{
                                    display: 'grid',
                                    gridTemplateColumns: '24px 1fr 1fr',
                                    gap: 10,
                                    padding: '5px 10px',
                                    background: 'var(--bg-tertiary)',
                                    borderBottom: '1px solid var(--border-color)',
                                }}>
                                    <span style={stepColHeader}>#</span>
                                    <span style={stepColHeader}>Action</span>
                                    <span style={stepColHeader}>Expected Result</span>
                                </div>
                                {/* Step rows */}
                                {steps.map((step, si) => (
                                    <StepRow
                                        key={si}
                                        step={step}
                                        index={si + 1}
                                        isLast={si === steps.length - 1}
                                        onChange={(field, val) => updateStep(si, field, val)}
                                    />
                                ))}
                            </div>
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}

// ─── Step row ─────────────────────────────────────────────────────────────────

function StepRow({ step, index, isLast, onChange }) {
    const [editingAction, setEditingAction] = useState(false);
    const [editingExpected, setEditingExpected] = useState(false);
    const [action, setAction] = useState(step.action);
    const [expected, setExpected] = useState(step.expected_result);

    return (
        <div style={{
            display: 'grid',
            gridTemplateColumns: '24px 1fr 1fr',
            gap: 10,
            alignItems: 'start',
            padding: '7px 10px',
            borderBottom: isLast ? 'none' : '1px solid var(--border-color)',
        }}>
            <span style={{
                fontSize: '0.72rem', fontWeight: 700,
                color: 'var(--text-secondary)', paddingTop: 3,
                textAlign: 'center',
            }}>
                {index}
            </span>

            <InlineCell
                value={action}
                editing={editingAction}
                onStartEdit={() => setEditingAction(true)}
                onStopEdit={() => setEditingAction(false)}
                onChange={v => { setAction(v); onChange('action', v); }}
            />
            <InlineCell
                value={expected}
                editing={editingExpected}
                onStartEdit={() => setEditingExpected(true)}
                onStopEdit={() => setEditingExpected(false)}
                onChange={v => { setExpected(v); onChange('expected_result', v); }}
            />
        </div>
    );
}

function InlineCell({ value, editing, onStartEdit, onStopEdit, onChange }) {
    return editing ? (
        <textarea
            className="modern-input"
            style={{ width: '100%', fontSize: '0.84rem', minHeight: 40, resize: 'vertical' }}
            value={value}
            onChange={e => onChange(e.target.value)}
            onBlur={onStopEdit}
            autoFocus
        />
    ) : (
        <div
            style={{
                fontSize: '0.84rem', cursor: 'text',
                lineHeight: 1.55, color: 'var(--text-primary)',
                padding: '2px 4px', borderRadius: 4,
                border: '1px solid transparent',
                transition: 'border-color 0.15s, background 0.15s',
            }}
            onMouseEnter={e => {
                e.currentTarget.style.borderColor = 'var(--border-color)';
                e.currentTarget.style.background = 'var(--bg-secondary)';
            }}
            onMouseLeave={e => {
                e.currentTarget.style.borderColor = 'transparent';
                e.currentTarget.style.background = 'transparent';
            }}
            onClick={onStartEdit}
            title="Click to edit"
        >
            {value || <span style={{ fontStyle: 'italic', opacity: 0.6 }}>Empty</span>}
        </div>
    );
}

// ─── Shared styles ────────────────────────────────────────────────────────────

const stepColHeader = {
    fontSize: '0.68rem', fontWeight: 700,
    color: 'var(--text-secondary)',
    textTransform: 'uppercase',
    letterSpacing: '0.04em',
};

const sectionLabel = {
    display: 'flex', alignItems: 'center',
    marginBottom: 5,
    fontSize: '0.7rem',
    color: 'var(--text-secondary)',
    fontWeight: 700,
    textTransform: 'uppercase',
    letterSpacing: '0.05em',
};
