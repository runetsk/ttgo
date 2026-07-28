import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { createTestRun, defects as defectsApi } from '../api';
import DefectModal from '../components/DefectModal';
import BulkBar from './defects/BulkBar';
import DefectRow from './defects/DefectRow';
import FilterBar from './defects/FilterBar';
import TriageStrip from './defects/TriageStrip';
import { buildRetestRun, nextSelectionOnFilterChange } from '../utils/defectActions';
import { filterDefects, queueCounts, sortDefects } from '../utils/defectQueue';

// The Defects register, as a triage queue rather than a record table.
//
// The whole defect list is fetched ONCE, on mount; filtering, sorting, search and
// every count are derived from it in memory (utils/defectQueue). That is strictly
// less work than the page did before — it already loaded everything, then refetched
// on every filter change and every keystroke — and it is what lets the tiles keep
// counting the full backlog while the table shows one slice of it.
//
// Scaling ceiling: this holds while a workspace has a few thousand defects. Past
// roughly 5k, move to a paginated server-side list plus a separate stats endpoint
// for the tiles and tab counts, and the derivations here become the server's job.

const DEFAULT_SORT = 'priority';

// The ⌘K hint is only honest if the shortcut works, so it is wired below — and
// labelled for the platform, since Ctrl is what actually fires it off a Mac.
const IS_MAC = typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform || '');
const SEARCH_HINT = IS_MAC ? '⌘K' : 'Ctrl K';

export default function DefectsPage() {
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();

    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [loadFailed, setLoadFailed] = useState(false);

    const [query, setQuery] = useState('');
    const [status, setStatus] = useState('all');
    const [severities, setSeverities] = useState([]);
    const [sort, setSort] = useState(DEFAULT_SORT);

    const [selected, setSelected] = useState(() => new Set());
    const [expandedId, setExpandedId] = useState(null);
    const [focusId, setFocusId] = useState(null);
    const [modal, setModal] = useState(null); // { mode:'create'|'edit', defect } | null
    const [retestError, setRetestError] = useState(null); // { id, message } | null

    const searchRef = useRef(null);

    // ?focus=<id> is a landing instruction, read once at mount — not a filter this
    // page keeps in sync. It is also deliberately LEFT in the URL: stripping it is
    // what an existing e2e assertion races against
    // (e2e/tests/test_cases/linked_bugs_requirements.spec.js), and a param that
    // survives a reload is the more useful behaviour anyway. The landing highlight
    // is cleared by the first interaction instead, in afterFilterChange.
    const pendingFocus = useRef(searchParams.get('focus'));

    // Pinned at mount so ages and the stale amber cannot drift mid-scan; a remount
    // re-reads the clock.
    const [now] = useState(() => Date.now());

    useEffect(() => {
        let cancelled = false;
        // No params: the register filters client-side. (defects.list still takes
        // them for DefectLinkPanel, which searches server-side.)
        defectsApi.list()
            .then(list => {
                if (cancelled) return;
                const loaded = list || [];
                setRows(loaded);
                setLoading(false);

                const target = pendingFocus.current;
                pendingFocus.current = null;
                if (target && loaded.some(d => d.id === target)) {
                    // Land on the unfiltered view: a linked defect can be in any
                    // queue, and the caller asked for that row specifically.
                    setStatus('all');
                    setSeverities([]);
                    setQuery('');
                    setExpandedId(target);
                    setFocusId(target);
                }
            })
            .catch(() => {
                if (cancelled) return;
                setLoading(false);
                setLoadFailed(true);
            });
        return () => { cancelled = true; };
    }, []);

    // Runs after the commit that rendered the focused row, so the element exists —
    // no timer needed the way the old page needed one.
    useEffect(() => {
        if (!focusId) return;
        document.getElementById(`defect-row-${focusId}`)?.scrollIntoView({ block: 'center' });
    }, [focusId]);

    useEffect(() => {
        const onKey = (event) => {
            if (event.key !== 'k' && event.key !== 'K') return;
            if (!event.metaKey && !event.ctrlKey) return;
            event.preventDefault();
            searchRef.current?.focus();
            searchRef.current?.select();
        };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, []);

    const counts = useMemo(() => queueCounts(rows, now), [rows, now]);
    const visible = useMemo(
        () => sortDefects(filterDefects(rows, { query, status, severities }), sort),
        [rows, query, status, severities, sort],
    );

    // Every filter control funnels through here. The selection is dropped because a
    // bulk action must never land on rows the user has just filtered out of sight,
    // and the ?focus landing highlight goes with it — the user has started working.
    const afterFilterChange = useCallback((changed) => {
        setSelected(prev => nextSelectionOnFilterChange(prev, changed));
        setFocusId(null);
    }, []);

    const changeQuery = (value) => { setQuery(value); afterFilterChange('query'); };
    const changeStatus = (value) => { setStatus(value); afterFilterChange('status'); };
    const changeSort = (value) => { setSort(value); afterFilterChange('sort'); };

    const toggleSeverity = (severity) => {
        setSeverities(prev => (prev.includes(severity)
            ? prev.filter(s => s !== severity)
            : [...prev, severity]));
        afterFilterChange('severities');
    };

    // A tile sets only the dimensions its preset names (utils/defectQueue's
    // TRIAGE_TILES). A typed query is left alone on purpose: it is visible in the
    // box, and silently discarding what someone typed is worse than showing them a
    // narrower queue than the tile's count promised.
    const pickTile = (preset) => {
        setStatus(preset.status);
        setSeverities(preset.severities);
        if (preset.sort) setSort(preset.sort);
        afterFilterChange('tile');
    };

    const resetFilters = () => {
        setQuery('');
        setStatus('all');
        setSeverities([]);
        setSort(DEFAULT_SORT);
        afterFilterChange('reset');
    };

    const toggleSelect = useCallback((id) => {
        setSelected(prev => {
            const next = new Set(prev);
            if (!next.delete(id)) next.add(id);
            return next;
        });
        setFocusId(null);
    }, []);

    const allVisibleSelected = visible.length > 0 && visible.every(d => selected.has(d.id));

    // Scoped to what is on screen: "select all" can only honestly mean the queue
    // being looked at. (The bulk endpoint caps a call at 500 ids; past that BulkBar
    // surfaces the rejection inline rather than half-applying anything.)
    const toggleSelectAll = () => {
        setSelected(allVisibleSelected ? new Set() : new Set(visible.map(d => d.id)));
        setFocusId(null);
    };

    const toggleExpand = useCallback((defect) => {
        setExpandedId(prev => (prev === defect.id ? null : defect.id));
        setRetestError(null);
        setFocusId(null);
    }, []);

    // bulk-update hands back fully enriched rows (assignee_name AND
    // linked_test_count), so the response replaces the row outright.
    const handleBulkApplied = useCallback((updated) => {
        if (updated.length > 0) {
            const byID = new Map(updated.map(d => [d.id, d]));
            setRows(prev => prev.map(r => byID.get(r.id) || r));
        }
        // Cleared even on success: a closed defect usually leaves the queue it was
        // ticked in, and a selection of rows nobody can see is the hazard this page
        // clears selections to avoid in the first place.
        setSelected(new Set());
    }, []);

    // PATCH /defects/{id} returns GetDefect, which resolves assignee_name but not
    // linked_test_count — carry the count over rather than blanking the Impact cell.
    const handleSaved = (saved) => {
        setRows(prev => (modal?.mode === 'edit'
            ? prev.map(r => (r.id === saved.id ? { ...saved, linked_test_count: r.linked_test_count } : r))
            : [saved, ...prev]));
    };

    const handleDelete = (defect) => {
        if (!window.confirm(`Delete defect "${defect.title}"?`)) return;
        defectsApi.remove(defect.id).then(() => {
            setRows(prev => prev.filter(r => r.id !== defect.id));
            setSelected(prev => {
                if (!prev.has(defect.id)) return prev;
                const next = new Set(prev);
                next.delete(defect.id);
                return next;
            });
            setExpandedId(prev => (prev === defect.id ? null : prev));
        }).catch(() => { /* api.js has already toasted */ });
    };

    const handleRetest = (defect, tests) => {
        const body = buildRetestRun(defect, tests);
        if (!body) return;
        setRetestError(null);
        createTestRun(null, body.name, null, body.test_case_ids)
            .then(run => navigate(`/runs/run/${run.id}`))
            .catch(() => setRetestError({
                id: defect.id,
                // POST /runs rejects the whole body when any id is unknown
                // (store.ErrUnknownTestCases), so one deleted test case fails the run.
                message: 'Could not start the retest run — one of the linked tests may no longer exist.',
            }));
    };

    return (
        <div className="defects-page">
            <div className="defects-titlebar">
                <h2 className="defects-title">Defects</h2>
                <span className="defects-title-meta">
                    {counts.open} open · {counts.needsTriage} need triage
                </span>
                <div className="defects-titlebar-actions">
                    <div className="defects-search">
                        <svg
                            className="defects-search-icon"
                            width="13"
                            height="13"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            aria-hidden="true"
                        >
                            <circle cx="11" cy="11" r="7" />
                            <line x1="21" y1="21" x2="16.65" y2="16.65" />
                        </svg>
                        <input
                            ref={searchRef}
                            type="search"
                            className="defects-search-input"
                            placeholder="Search title or key"
                            aria-label="Search defects by title or external key"
                            value={query}
                            onChange={event => changeQuery(event.target.value)}
                        />
                        <span className="defects-search-hint" aria-hidden="true">{SEARCH_HINT}</span>
                    </div>
                    <button
                        type="button"
                        className="primary-btn"
                        onClick={() => setModal({ mode: 'create', defect: null })}
                        data-testid="defects-new"
                    >
                        + New defect
                    </button>
                </div>
            </div>

            <TriageStrip
                counts={counts}
                status={status}
                severities={severities}
                onPick={pickTile}
            />

            <FilterBar
                counts={counts}
                status={status}
                onStatusChange={changeStatus}
                severities={severities}
                onToggleSeverity={toggleSeverity}
                sort={sort}
                onSortChange={changeSort}
            />

            {/* Self-guards on an empty selection, so there is never a "0 selected" bar. */}
            <BulkBar
                selectedIds={selected}
                onApplied={handleBulkApplied}
                onClear={() => setSelected(new Set())}
            />

            <div className="defects-table-wrap">
                <table className="defects-table">
                    <thead>
                        <tr>
                            <th className="defects-col-select">
                                <input
                                    type="checkbox"
                                    className="defects-checkbox"
                                    aria-label="Select every defect in this queue"
                                    checked={allVisibleSelected}
                                    disabled={visible.length === 0}
                                    onChange={toggleSelectAll}
                                    data-testid="defects-select-all"
                                />
                            </th>
                            <th>Defect</th>
                            <th className="defects-col-status">Status</th>
                            <th className="defects-col-owner">Owner</th>
                            <th className="defects-col-impact">Impact</th>
                            <th className="defects-col-age">Age</th>
                        </tr>
                    </thead>
                    <tbody>
                        {visible.map(defect => (
                            <DefectRow
                                key={defect.id}
                                defect={defect}
                                now={now}
                                selected={selected.has(defect.id)}
                                expanded={expandedId === defect.id}
                                focused={focusId === defect.id}
                                retestError={retestError?.id === defect.id ? retestError.message : ''}
                                onToggleSelect={toggleSelect}
                                onToggleExpand={toggleExpand}
                                onOpenDetail={target => setModal({ mode: 'edit', defect: target })}
                                onRetest={handleRetest}
                                onDelete={handleDelete}
                            />
                        ))}
                    </tbody>
                </table>

                {/* Only ever on first paint: after that the table is patched in place
                    from mutation responses and never blanks to "Loading…" again. */}
                {loading && (
                    <div className="defects-empty">
                        <span className="defects-empty-text">Loading defects…</span>
                    </div>
                )}

                {!loading && loadFailed && (
                    <div className="defects-empty">
                        <span className="defects-empty-title">Couldn&apos;t load the defects</span>
                        <span className="defects-empty-text">Reload the page to try again.</span>
                    </div>
                )}

                {!loading && !loadFailed && visible.length === 0 && (
                    <div className="defects-empty" data-testid="defects-empty">
                        <div className="defects-empty-icon">
                            <svg
                                width="18"
                                height="18"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                strokeWidth="2.2"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                aria-hidden="true"
                            >
                                <polyline points="20 6 9 17 4 12" />
                            </svg>
                        </div>
                        <span className="defects-empty-title">Nothing in this queue</span>
                        <span className="defects-empty-text">
                            {rows.length === 0 ? 'No defects have been reported yet.' : 'No defects match these filters.'}
                        </span>
                        {rows.length > 0 && (
                            <button
                                type="button"
                                className="defects-ghost-btn"
                                onClick={resetFilters}
                                data-testid="defects-reset-filters"
                            >
                                Reset filters
                            </button>
                        )}
                    </div>
                )}
            </div>

            {modal && (
                <DefectModal
                    key={modal.defect?.id || 'create'}
                    mode={modal.mode}
                    defect={modal.defect}
                    onClose={() => setModal(null)}
                    onSaved={handleSaved}
                />
            )}
        </div>
    );
}
