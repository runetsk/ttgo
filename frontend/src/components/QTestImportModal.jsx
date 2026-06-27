import React, { useEffect, useMemo, useState } from 'react';
import { getFolderTree, qtest } from '../api';
import { toast } from '../toast';
import FolderTreeSelect from './FolderTreeSelect';
import ModalShell from './shared/ModalShell';
import { stripHtml } from '../utils/htmlUtils';

function flattenFolderTree(nodes, depth = 0) {
    if (!Array.isArray(nodes)) return [];
    return nodes.flatMap((node) => [
        { id: node.id, name: node.name, depth },
        ...flattenFolderTree(node.sub_folders || [], depth + 1),
    ]);
}

function flattenModules(nodes, depth = 0, parentPath = '') {
    if (!Array.isArray(nodes)) return [];
    return nodes.flatMap((node) => {
        const path = parentPath ? `${parentPath} / ${node.name}` : node.name;
        return [
            {
                id: node.id,
                name: node.name,
                depth,
                path: node.path || path,
            },
            ...flattenModules(node.children || [], depth + 1, path),
        ];
    });
}

function filterModulesFlat(modules, search) {
    if (!modules || !search) return [];
    const lower = search.toLowerCase();
    const results = [];
    const walk = (list) => {
        for (const module of list || []) {
            if (module.name.toLowerCase().includes(lower)) {
                results.push({
                    id: module.id,
                    name: module.name,
                    depth: 0,
                    hasChildren: false,
                    path: module.path,
                });
            }
            if (module.children) {
                walk(module.children);
            }
        }
    };
    walk(modules);
    return results;
}


export default function QTestImportModal({ initialFolderId, onClose, onImported }) {
    const [folders, setFolders] = useState([]);
    const [enabledProjects, setEnabledProjects] = useState([]);
    const [selectedProjectId, setSelectedProjectId] = useState('');
    const [modules, setModules] = useState([]);
    const [selectedModuleId, setSelectedModuleId] = useState('');
    const [moduleSearch, setModuleSearch] = useState('');
    const [expandedModuleIds, setExpandedModuleIds] = useState(new Set());
    const [remoteCases, setRemoteCases] = useState([]);
    const [selectedCaseIds, setSelectedCaseIds] = useState([]);
    const [targetFolderId, setTargetFolderId] = useState(initialFolderId || '');
    const [onConflict, setOnConflict] = useState('skip');
    const [includeSubmodules, setIncludeSubmodules] = useState(true);
    const [preserveHierarchy, setPreserveHierarchy] = useState(true);
    const [caseSearch, setCaseSearch] = useState('');
    const [loadingSetup, setLoadingSetup] = useState(true);
    const [loadingModules, setLoadingModules] = useState(false);
    const [loadingCases, setLoadingCases] = useState(false);
    const [importing, setImporting] = useState(false);

    useEffect(() => {
        let cancelled = false;
        setLoadingSetup(true);

        Promise.all([qtest.listEnabledProjects(), getFolderTree()])
            .then(([projects, tree]) => {
                if (cancelled) return;

                const nextProjects = projects || [];
                const flatFolders = flattenFolderTree(Array.isArray(tree) ? tree : []);

                setEnabledProjects(nextProjects);
                setFolders(flatFolders);

                if (!initialFolderId && flatFolders.length > 0) {
                    setTargetFolderId(flatFolders[0].id);
                }

                const defaultProject = nextProjects.find((project) => project.is_default) || nextProjects[0];
                setSelectedProjectId(defaultProject ? String(defaultProject.project_id) : '');
            })
            .catch(() => {
                if (!cancelled) {
                    toast.error('Failed to load QTest import options');
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoadingSetup(false);
                }
            });

        return () => {
            cancelled = true;
        };
    }, [initialFolderId]);

    useEffect(() => {
        if (!selectedProjectId) {
            setModules([]);
            setSelectedModuleId('');
            return;
        }

        let cancelled = false;
        setLoadingModules(true);
        setSelectedModuleId('');
        setModuleSearch('');
        setExpandedModuleIds(new Set());
        setRemoteCases([]);
        setSelectedCaseIds([]);

        qtest.listModules(parseInt(selectedProjectId, 10))
            .then((data) => {
                if (!cancelled) {
                    setModules(data || []);
                }
            })
            .catch(() => {
                if (!cancelled) {
                    toast.error('Failed to load QTest modules');
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoadingModules(false);
                }
            });

        return () => {
            cancelled = true;
        };
    }, [selectedProjectId]);

    useEffect(() => {
        if (!selectedProjectId || !selectedModuleId) {
            setRemoteCases([]);
            setSelectedCaseIds([]);
            return;
        }

        let cancelled = false;
        setLoadingCases(true);
        setSelectedCaseIds([]);

        qtest.listTestCases(parseInt(selectedProjectId, 10), parseInt(selectedModuleId, 10), includeSubmodules)
            .then((data) => {
                if (!cancelled) {
                    setRemoteCases(data || []);
                }
            })
            .catch(() => {
                if (!cancelled) {
                    toast.error('Failed to load QTest test cases');
                }
            })
            .finally(() => {
                if (!cancelled) {
                    setLoadingCases(false);
                }
            });

        return () => {
            cancelled = true;
        };
    }, [includeSubmodules, selectedProjectId, selectedModuleId]);

    const moduleOptions = useMemo(() => flattenModules(modules), [modules]);
    const selectedModule = moduleOptions.find((module) => String(module.id) === String(selectedModuleId)) || null;
    const isSearchingModules = moduleSearch.trim().length > 0;
    const displayModules = useMemo(() => {
        if (isSearchingModules) {
            return filterModulesFlat(moduleOptions, moduleSearch);
        }

        const result = [];
        const walk = (list, depth) => {
            if (!list) return;
            for (const module of list) {
                result.push({
                    id: module.id,
                    name: module.name,
                    depth,
                    hasChildren: !!(module.children && module.children.length > 0),
                    path: module.path,
                });
                if (module.children && expandedModuleIds.has(module.id)) {
                    walk(module.children, depth + 1);
                }
            }
        };

        walk(modules, 0);
        return result;
    }, [expandedModuleIds, isSearchingModules, moduleOptions, moduleSearch, modules]);
    const filteredCases = useMemo(() => {
        const search = caseSearch.trim().toLowerCase();
        if (!search) return remoteCases;
        return remoteCases.filter((testCase) => {
            const haystack = [
                testCase.pid,
                testCase.name,
                testCase.module_path,
                stripHtml(testCase.description),
            ].join(' ').toLowerCase();
            return haystack.includes(search);
        });
    }, [caseSearch, remoteCases]);

    const visibleCaseIDs = filteredCases.map((testCase) => String(testCase.id));
    const allVisibleSelected = visibleCaseIDs.length > 0 && visibleCaseIDs.every((id) => selectedCaseIds.includes(id));

    const toggleCase = (id) => {
        setSelectedCaseIds((current) => (
            current.includes(id)
                ? current.filter((existingID) => existingID !== id)
                : [...current, id]
        ));
    };

    const toggleVisibleCases = () => {
        if (allVisibleSelected) {
            setSelectedCaseIds((current) => current.filter((id) => !visibleCaseIDs.includes(id)));
            return;
        }
        setSelectedCaseIds((current) => [...new Set([...current, ...visibleCaseIDs])]);
    };

    const toggleModuleExpand = (id) => {
        setExpandedModuleIds((current) => {
            const next = new Set(current);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    const handleImport = () => {
        if (!selectedProjectId || !selectedModuleId || !targetFolderId || selectedCaseIds.length === 0) {
            return;
        }

        setImporting(true);

        qtest.importTestCases({
            project_id: parseInt(selectedProjectId, 10),
            module_id: parseInt(selectedModuleId, 10),
            module_path: selectedModule?.path || '',
            folder_id: targetFolderId,
            test_case_ids: selectedCaseIds.map((id) => parseInt(id, 10)).filter((id) => Number.isFinite(id)),
            on_conflict: onConflict,
            preserve_hierarchy: includeSubmodules && preserveHierarchy,
            recursive: includeSubmodules,
        })
            .then((result) => {
                toast.success(`Imported ${result.succeeded || 0}, skipped ${result.skipped || 0}, failed ${result.failed || 0}`);
                if (onImported) {
                    onImported(result);
                }
                onClose();
            })
            .catch((err) => {
                toast.error(err.response?.data?.error || 'Import failed');
            })
            .finally(() => setImporting(false));
    };

    return (
        <ModalShell
            title="Import from QTest"
            subtitle={selectedModule ? selectedModule.path : undefined}
            width={900}
            maxHeight="85vh"
            onClose={() => !importing && onClose()}
            footer={(
                <>
                    <button className="action-btn" onClick={onClose} disabled={importing}>
                        Cancel
                    </button>
                    <button
                        className="primary-btn"
                        onClick={handleImport}
                        disabled={importing || !selectedProjectId || !selectedModuleId || !targetFolderId || selectedCaseIds.length === 0}
                    >
                        {importing ? 'Importing...' : `Import ${selectedCaseIds.length} Test Case${selectedCaseIds.length === 1 ? '' : 's'}`}
                    </button>
                </>
            )}
        >
            {loadingSetup ? (
                <div style={{ padding: '32px 12px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                    Loading QTest import options...
                </div>
            ) : enabledProjects.length === 0 ? (
                <div style={{
                    padding: 16,
                    borderRadius: 10,
                    border: '1px solid rgba(251,191,36,0.3)',
                    background: 'rgba(251,191,36,0.08)',
                    color: 'var(--text-secondary)',
                }}>
                    No enabled QTest projects found. Configure them in Settings before importing.
                </div>
            ) : (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
                    <div style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
                        gap: 14,
                    }}>
                        <div>
                            <label style={{ display: 'block', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                                QTest Project
                            </label>
                            <select
                                className="modern-select"
                                value={selectedProjectId}
                                onChange={(event) => setSelectedProjectId(event.target.value)}
                                style={{ width: '100%' }}
                            >
                                {enabledProjects.map((project) => (
                                    <option key={project.project_id} value={String(project.project_id)}>
                                        {project.project_name}{project.is_default ? ' (Default)' : ''}
                                    </option>
                                ))}
                            </select>
                        </div>

                        <div>
                            <label style={{ display: 'block', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                                Target Folder
                            </label>
                            <FolderTreeSelect
                                folders={folders}
                                value={targetFolderId}
                                onChange={setTargetFolderId}
                                disabled={importing}
                            />
                        </div>

                        <div style={{ gridColumn: '1 / -1' }}>
                            <label style={{ display: 'block', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 6, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                                QTest Module
                            </label>
                            {loadingModules ? (
                                <div style={{
                                    padding: '18px 14px',
                                    border: '1px solid var(--border-color)',
                                    borderRadius: 10,
                                    background: 'var(--bg-secondary)',
                                    color: 'var(--text-secondary)',
                                }}>
                                    Loading modules...
                                </div>
                            ) : (
                                <div style={{
                                    border: '1px solid var(--border-color)',
                                    borderRadius: 12,
                                    overflow: 'hidden',
                                    background: 'var(--bg-secondary)',
                                }}>
                                    <div style={{
                                        padding: '10px 12px',
                                        borderBottom: '1px solid var(--border-color)',
                                        display: 'flex',
                                        alignItems: 'center',
                                        gap: 8,
                                    }}>
                                        <span style={{ color: 'var(--text-secondary)' }}>🔍</span>
                                        <input
                                            className="modern-input"
                                            value={moduleSearch}
                                            onChange={(event) => setModuleSearch(event.target.value)}
                                            placeholder="Search modules..."
                                            style={{ flex: 1 }}
                                        />
                                    </div>

                                    <div style={{ maxHeight: 240, overflowY: 'auto', padding: '6px 0' }}>
                                        {displayModules.length === 0 ? (
                                            <div style={{ padding: '22px 14px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                                                {moduleOptions.length === 0 ? 'No modules available for this project.' : 'No modules match your search.'}
                                            </div>
                                        ) : displayModules.map((module) => {
                                            const isSelected = String(module.id) === String(selectedModuleId);
                                            const isExpanded = expandedModuleIds.has(module.id);
                                            return (
                                                <div
                                                    key={module.id}
                                                    onClick={() => setSelectedModuleId(String(module.id))}
                                                    style={{
                                                        padding: '8px 12px',
                                                        paddingLeft: isSearchingModules ? 12 : 12 + module.depth * 20,
                                                        display: 'flex',
                                                        alignItems: 'center',
                                                        gap: 6,
                                                        cursor: 'pointer',
                                                        background: isSelected ? 'rgba(99,102,241,0.1)' : 'transparent',
                                                        color: isSelected ? 'var(--accent-indigo)' : 'var(--text-primary)',
                                                        fontWeight: isSelected ? 600 : 400,
                                                    }}
                                                >
                                                    {module.hasChildren && !isSearchingModules ? (
                                                        <button
                                                            type="button"
                                                            onClick={(event) => {
                                                                event.stopPropagation();
                                                                toggleModuleExpand(module.id);
                                                            }}
                                                            style={{
                                                                width: 18,
                                                                height: 18,
                                                                border: 'none',
                                                                background: 'transparent',
                                                                color: 'var(--text-secondary)',
                                                                cursor: 'pointer',
                                                                padding: 0,
                                                                transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                                                            }}
                                                        >
                                                            ▶
                                                        </button>
                                                    ) : (
                                                        <span style={{ width: 18, display: 'inline-block' }} />
                                                    )}
                                                    <span style={{ opacity: 0.75 }}>📁</span>
                                                    <span style={{ flex: 1 }}>{module.name}</span>
                                                    {isSelected && <span>✓</span>}
                                                </div>
                                            );
                                        })}
                                    </div>

                                    {selectedModule && (
                                        <div style={{
                                            padding: '10px 12px',
                                            borderTop: '1px solid var(--border-color)',
                                            color: 'var(--accent-indigo)',
                                            fontSize: '0.82rem',
                                        }}>
                                            Selected: <span style={{ fontWeight: 600 }}>{selectedModule.path}</span>
                                        </div>
                                    )}
                                </div>
                            )}
                        </div>
                    </div>

                    <div>
                        <label style={{ display: 'block', fontSize: '0.78rem', fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 8, textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                            Conflict Handling
                        </label>
                        <div style={{ display: 'flex', gap: 10 }}>
                            {[
                                { value: 'skip', label: 'Skip Existing', hint: 'Leave matching TTGO cases unchanged' },
                                { value: 'update', label: 'Update Existing', hint: 'Overwrite matching TTGO cases with qTest data' },
                            ].map((option) => (
                                <button
                                    key={option.value}
                                    type="button"
                                    onClick={() => setOnConflict(option.value)}
                                    style={{
                                        flex: 1,
                                        textAlign: 'left',
                                        padding: '12px 14px',
                                        borderRadius: 10,
                                        border: `1px solid ${onConflict === option.value ? 'var(--accent-indigo)' : 'var(--border-color)'}`,
                                        background: onConflict === option.value ? 'rgba(99,102,241,0.1)' : 'var(--bg-secondary)',
                                        color: 'var(--text-primary)',
                                        cursor: 'pointer',
                                    }}
                                >
                                    <div style={{ fontSize: '0.9rem', fontWeight: 600 }}>{option.label}</div>
                                    <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 2 }}>{option.hint}</div>
                                </button>
                            ))}
                        </div>
                    </div>

                    <label style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 10,
                        padding: '12px 14px',
                        borderRadius: 10,
                        background: 'rgba(99,102,241,0.06)',
                        border: '1px solid rgba(99,102,241,0.14)',
                        cursor: 'pointer',
                    }}>
                        <input
                            type="checkbox"
                            checked={includeSubmodules}
                            onChange={(event) => setIncludeSubmodules(event.target.checked)}
                            style={{ marginTop: 2 }}
                        />
                        <div>
                            <div style={{ fontSize: '0.88rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                                Include submodules
                            </div>
                            <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                                Import the selected qTest module and everything under it.
                            </div>
                        </div>
                    </label>

                    <label style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 10,
                        padding: '12px 14px',
                        borderRadius: 10,
                        background: includeSubmodules ? 'rgba(16,185,129,0.06)' : 'rgba(148,163,184,0.06)',
                        border: includeSubmodules ? '1px solid rgba(16,185,129,0.16)' : '1px solid rgba(148,163,184,0.14)',
                        cursor: includeSubmodules ? 'pointer' : 'not-allowed',
                        opacity: includeSubmodules ? 1 : 0.7,
                    }}>
                        <input
                            type="checkbox"
                            checked={includeSubmodules && preserveHierarchy}
                            onChange={(event) => setPreserveHierarchy(event.target.checked)}
                            disabled={!includeSubmodules}
                            style={{ marginTop: 2 }}
                        />
                        <div>
                            <div style={{ fontSize: '0.88rem', fontWeight: 600, color: 'var(--text-primary)' }}>
                                Mirror qTest folder hierarchy
                            </div>
                            <div style={{ fontSize: '0.78rem', color: 'var(--text-secondary)', marginTop: 2 }}>
                                Create TTGO subfolders that match the selected qTest module tree. Turn this off to import all selected cases into the target folder only.
                            </div>
                        </div>
                    </label>

                    <div style={{
                        border: '1px solid var(--border-color)',
                        borderRadius: 12,
                        overflow: 'hidden',
                        background: 'var(--bg-secondary)',
                    }}>
                        <div style={{
                            padding: '12px 14px',
                            borderBottom: '1px solid var(--border-color)',
                            display: 'flex',
                            gap: 10,
                            alignItems: 'center',
                            flexWrap: 'wrap',
                        }}>
                            <div style={{ fontWeight: 600, flex: 1, minWidth: 180 }}>
                                Remote Test Cases
                            </div>
                            <input
                                className="modern-input"
                                placeholder="Search test cases..."
                                value={caseSearch}
                                onChange={(event) => setCaseSearch(event.target.value)}
                                disabled={!selectedModuleId || loadingCases}
                                style={{ width: 220 }}
                            />
                            <button
                                type="button"
                                className="action-btn"
                                onClick={toggleVisibleCases}
                                disabled={visibleCaseIDs.length === 0 || loadingCases}
                            >
                                {allVisibleSelected ? 'Clear Visible' : 'Select Visible'}
                            </button>
                        </div>

                        {!selectedModuleId ? (
                            <div style={{ padding: '36px 16px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                                Choose a QTest module to load its test cases.
                            </div>
                        ) : loadingCases ? (
                            <div style={{ padding: '36px 16px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                                Loading test cases...
                            </div>
                        ) : filteredCases.length === 0 ? (
                            <div style={{ padding: '36px 16px', textAlign: 'center', color: 'var(--text-secondary)' }}>
                                {remoteCases.length === 0 ? 'No test cases found in this module.' : 'No test cases match your search.'}
                            </div>
                        ) : (
                            <div style={{ maxHeight: 380, overflowY: 'auto' }}>
                                {filteredCases.map((testCase) => {
                                    const isSelected = selectedCaseIds.includes(String(testCase.id));
                                    return (
                                        <label
                                            key={testCase.id}
                                            style={{
                                                display: 'flex',
                                                gap: 12,
                                                alignItems: 'flex-start',
                                                padding: '12px 14px',
                                                borderTop: '1px solid rgba(255,255,255,0.04)',
                                                background: isSelected ? 'rgba(99,102,241,0.08)' : 'transparent',
                                                cursor: 'pointer',
                                            }}
                                        >
                                            <input
                                                type="checkbox"
                                                checked={isSelected}
                                                onChange={() => toggleCase(String(testCase.id))}
                                                style={{ marginTop: 2 }}
                                            />
                                            <div style={{ flex: 1, minWidth: 0 }}>
                                                <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                                                    <span style={{ fontWeight: 600 }}>{testCase.name}</span>
                                                    <span style={{ fontSize: '0.72rem', color: 'var(--accent-indigo)', background: 'rgba(99,102,241,0.12)', padding: '2px 6px', borderRadius: 999 }}>
                                                        {testCase.pid || `#${testCase.id}`}
                                                    </span>
                                                    {testCase.module_path && testCase.module_path !== selectedModule?.path && (
                                                        <span style={{ fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
                                                            {testCase.module_path}
                                                        </span>
                                                    )}
                                                    <span style={{ fontSize: '0.72rem', color: 'var(--text-secondary)' }}>
                                                        {testCase.steps?.length || 0} step{(testCase.steps?.length || 0) === 1 ? '' : 's'}
                                                    </span>
                                                </div>
                                                {testCase.description && (
                                                    <div style={{ fontSize: '0.82rem', color: 'var(--text-secondary)', marginTop: 4 }}>
                                                        {stripHtml(testCase.description).slice(0, 180) || 'No description'}
                                                    </div>
                                                )}
                                            </div>
                                        </label>
                                    );
                                })}
                            </div>
                        )}
                    </div>
                </div>
            )}
        </ModalShell>
    );
}
