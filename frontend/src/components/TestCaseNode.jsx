import React, { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';

const TestCaseNode = ({ testCase, depth = 0, activeTestId, selectedTestIds = [], onSelectTest, onDeleteTests }) => {
    const isActive = activeTestId === testCase.id;
    const isSelected = selectedTestIds.includes(testCase.id);
    const openCount = testCase.open_defect_count || 0;
    const closedCount = testCase.closed_defect_count || 0;
    const hasDefects = openCount > 0 || closedCount > 0;
    const passRate = typeof testCase.pass_rate === 'number' ? testCase.pass_rate : null;
    const dotColor = passRate === null
        ? 'rgba(255,255,255,0.25)'
        : passRate > 0.85 ? '#22c55e'
        : passRate > 0.7 ? '#f59e0b'
        : '#ef4444';

    const [contextMenu, setContextMenu] = useState(null);
    const menuRef = useRef(null);

    useEffect(() => {
        if (!contextMenu) return;
        const handleClickOutside = (e) => {
            if (menuRef.current && !menuRef.current.contains(e.target)) setContextMenu(null);
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [contextMenu]);

    const handleClick = (e) => {
        if (!onSelectTest) return;
        if (e.metaKey || e.ctrlKey) {
            e.preventDefault(); e.stopPropagation();
            onSelectTest(testCase, { type: 'multi' });
        } else if (e.shiftKey) {
            e.preventDefault(); e.stopPropagation();
            onSelectTest(testCase, { type: 'range' });
            // Shift-click selects text by default; clear it.
            if (window.getSelection) window.getSelection().removeAllRanges();
        }
        // Plain click: let the Link navigate. Selection is unchanged.
    };

    const handleContextMenu = (e) => {
        e.preventDefault();
        e.stopPropagation();
        // Right-clicking an unselected test makes it the sole selection.
        if (!isSelected && onSelectTest) onSelectTest(testCase, { type: 'single' });
        setContextMenu({ x: e.clientX, y: e.clientY });
    };

    const handleDragStart = (e) => {
        e.stopPropagation();
        if (isSelected && selectedTestIds.length > 1) {
            e.dataTransfer.setData('testIds', JSON.stringify(selectedTestIds));
        }
        e.dataTransfer.setData('testId', testCase.id);
        e.dataTransfer.effectAllowed = 'move';
    };

    const handleDelete = () => {
        setContextMenu(null);
        if (!onDeleteTests) return;
        // If the right-clicked test is part of a multi-selection, delete the whole set.
        const ids = isSelected && selectedTestIds.length > 1 ? selectedTestIds : [testCase.id];
        onDeleteTests(ids);
    };

    const className = `test-case-node ${isActive ? 'active' : ''} ${isSelected ? 'selected' : ''}`;
    const isMulti = isSelected && selectedTestIds.length > 1;

    return (
        <>
            <Link
                to={`/library/tests/${testCase.id}`}
                className={className}
                data-testid={`test-case-${testCase.id}`}
                draggable
                onClick={handleClick}
                onContextMenu={handleContextMenu}
                onDragStart={handleDragStart}
                style={{ paddingLeft: 8 + depth * 14 + 14 }}
            >
                <span style={{
                    width: 5, height: 5, borderRadius: '50%',
                    background: dotColor,
                    flexShrink: 0,
                }} />
                <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', flex: 1 }}>
                    {testCase.name}
                </span>
                {/* Defect count chip (008-jira-integration, FR-015) */}
                {hasDefects && (
                    <span
                        title={`${openCount} open, ${closedCount} resolved defect${openCount + closedCount !== 1 ? 's' : ''}`}
                        style={{
                            fontSize: '0.68em', fontWeight: 700, flexShrink: 0,
                            padding: '0 5px', borderRadius: 99, lineHeight: '16px',
                            background: openCount > 0 ? 'rgba(239,68,68,0.18)' : 'rgba(52,211,153,0.15)',
                            color: openCount > 0 ? '#f87171' : '#34d399',
                            border: `1px solid ${openCount > 0 ? 'rgba(239,68,68,0.35)' : 'rgba(52,211,153,0.35)'}`,
                        }}
                    >
                        {openCount > 0 ? `${openCount}🐛` : `✓${closedCount}`}
                    </span>
                )}
            </Link>
            {contextMenu && (
                <div className="context-menu" ref={menuRef} style={{ top: contextMenu.y, left: contextMenu.x }} onClick={(e) => e.stopPropagation()}>
                    <div style={{ padding: '8px 12px', fontSize: '0.7em', color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.05em', borderBottom: '1px solid var(--border-color)', marginBottom: 4 }}>
                        {isMulti ? `${selectedTestIds.length} Tests` : testCase.name}
                    </div>
                    <div className="context-menu-item danger" onClick={handleDelete} data-testid="context-menu-delete-test">
                        <span>🗑️</span> {isMulti ? 'Delete Selected' : 'Delete Test'}
                    </div>
                </div>
            )}
        </>
    );
};
export default TestCaseNode;
