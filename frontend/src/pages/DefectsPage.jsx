import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { createTestRun, defects as defectsApi } from '../api';
import DefectModal from '../components/DefectModal';
import Modal from '../components/Modal';
import BulkBar from './defects/BulkBar';
import DefectRow from './defects/DefectRow';
import FilterBar from './defects/FilterBar';
import TriageStrip from './defects/TriageStrip';
import { applyBulkResult, bulkLockFor, buildRetestRun, isBulkLocked, isEditSnapshotStale, rememberBulkTouched, BULK_LOCK_MESSAGE } from '../utils/defectActions';
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
    // The two dimensions only a tile can set (utils/defectQueue's TRIAGE_TILES):
    // "not closed" and "nothing has happened in a week". They have no control in the
    // filter bar, so the two controls that NARROW by hand — the status tabs and the
    // severity chips — clear them again rather than leaving an invisible filter on the
    // table (clearTileDimensions below). Search and Sort deliberately do not: neither
    // one contradicts what a tile set, and dropping a tile's queue because someone
    // typed in the search box would be its own surprise.
    const [openOnly, setOpenOnly] = useState(false);
    const [staleOnly, setStaleOnly] = useState(false);

    const [selected, setSelected] = useState(() => new Set());
    const [expandedId, setExpandedId] = useState(null);
    const [focusId, setFocusId] = useState(null);
    const [modal, setModal] = useState(null); // { mode:'create'|'edit', defect } | null
    const [pendingDelete, setPendingDelete] = useState(null); // defect awaiting confirmation | null
    // An inline message pinned to ONE row's expand panel: a Retest that could not be started, or
    // a write this page refused because a bulk apply is mid-flight on that defect.
    const [notice, setNotice] = useState(null); // { id, message } | null
    const [focusMissing, setFocusMissing] = useState(false); // ?focus named a defect that is gone
    // Reported by BulkBar as it resizes; drives the scroll clearance under the bar.
    const [barHeight, setBarHeight] = useState(0);
    // Also reported by BulkBar: the ids one of its applies CAPTURED at click time, held as
    // a Set for as long as that call is in flight and null the rest of the time. Two
    // different freezes come off it, and the difference is the whole point (bulkLockFor):
    //
    //   · the checkboxes and select-all freeze TABLE-WIDE, because what must hold still
    //     there is the id set itself — ticking any row moves the selection the in-flight
    //     call captured, which loses what the user built when the success handler clears
    //     and lands the failure message over a set nothing was tried on;
    //   · each row's OWN write paths freeze only if that row is in the captured list,
    //     because those are the only defects being written to.
    //
    // The second one must never be read off `selected`. No filter control is locked while
    // an apply runs and every one of them clears the selection on purpose
    // (afterFilterChange), so one re-sort mid-flight emptied it and handed back the modal,
    // Delete and Retest on rows the request was still writing to.
    const [bulkLock, setBulkLock] = useState(null);
    const handleBulkInFlight = useCallback((ids) => setBulkLock(bulkLockFor(ids)), []);
    const selectionLocked = bulkLock !== null;

    // The same lock, asked as a question at the point of a write rather than read as a boolean
    // at a button. Disabling the buttons is still done — it is the honest thing to show — but it
    // is no longer what makes the register safe: a dialog opened BEFORE an apply started was
    // opened while nothing was locked, and its Save/Delete never pass any disabled control. So
    // every per-defect write below asks this immediately before issuing its request. See
    // utils/defectActions isBulkLocked for why five rounds of path-guarding kept losing.
    const isLocked = useCallback((id) => isBulkLocked(bulkLock, id), [bulkLock]);

    // Which ids a bulk apply has REWRITTEN under the edit dialog that is open RIGHT NOW. A ref,
    // not state: nothing on screen depends on it, and it is only ever read inside a click handler
    // at the moment of a write. It is what makes the edit dialog's guard outlive the request —
    // the lock releases when the apply lands, the dialog's pre-bulk snapshot does not stop being
    // stale.
    //
    // Emptied whenever a dialog opens OR closes, and only the open dialog's own defect is ever
    // put in it (rememberBulkTouched), so it holds at most one id. An id can only matter while a
    // dialog holding a snapshot of it is up: with nothing open there is no stale snapshot to
    // refuse, and the next dialog is seeded from the row the apply produced. Recording every
    // applied id — the first shape of this — was correct but unbounded, because only opening a
    // dialog cleared it and a bulk-only session never does.
    const bulkTouchedRef = useRef(new Set());
    // The defect the open edit dialog is holding a snapshot of, or null. A ref rather than a read
    // of `modal` because BulkBar captures its onApplied at click time: a callback closed over
    // `modal` would report the dialog as it stood when the apply STARTED, not when it landed.
    const editingIdRef = useRef(null);

    // Every dialog open and close goes through these two, so the staleness record cannot drift
    // from what is on screen. Opening is what takes a fresh snapshot; closing is what throws the
    // last one away.
    const openModal = useCallback((next) => {
        bulkTouchedRef.current = new Set();
        editingIdRef.current = next?.mode === 'edit' ? (next.defect?.id || null) : null;
        setModal(next);
    }, []);
    const closeModal = useCallback(() => {
        bulkTouchedRef.current = new Set();
        editingIdRef.current = null;
        setModal(null);
    }, []);

    // What DefectModal asks immediately before it PATCHes. Wider than isLocked on purpose; see
    // utils/defectActions isEditSnapshotStale for why the in-flight window alone was not enough.
    const isEditStale = useCallback(
        (id) => isEditSnapshotStale(bulkLock, bulkTouchedRef.current, id),
        [bulkLock],
    );

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
                if (!target) return;
                if (loaded.some(d => d.id === target)) {
                    // Land on the unfiltered view: a linked defect can be in any
                    // queue, and the caller asked for that row specifically.
                    setStatus('all');
                    setSeverities([]);
                    setQuery('');
                    setExpandedId(target);
                    setFocusId(target);
                    return;
                }
                // A deep link into a defect that has since been deleted otherwise
                // renders as an ordinary unfiltered load, with nothing to say the
                // link failed — and bugHref (utils/bugs.js) makes those links
                // load-bearing from every test case's linked-bugs panel.
                setFocusMissing(true);
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

    // Scoped out while a dialog is open: the shortcut is document-wide, so without
    // this it steals focus out of the edit modal or the delete confirmation while
    // someone is typing in one of their fields.
    const dialogOpen = Boolean(modal) || Boolean(pendingDelete);
    useEffect(() => {
        if (dialogOpen) return undefined;
        const onKey = (event) => {
            if (event.key !== 'k' && event.key !== 'K') return;
            if (!event.metaKey && !event.ctrlKey) return;
            event.preventDefault();
            searchRef.current?.focus();
            searchRef.current?.select();
        };
        document.addEventListener('keydown', onKey);
        return () => document.removeEventListener('keydown', onKey);
    }, [dialogOpen]);

    const counts = useMemo(() => queueCounts(rows, now), [rows, now]);

    // The whole filter state as one object: it is what filterDefects takes and what
    // a tile's aria-pressed is compared against, so the two can never drift.
    const view = useMemo(
        () => ({ query, status, severities, openOnly, stale: staleOnly, now }),
        [query, status, severities, openOnly, staleOnly, now],
    );
    const visible = useMemo(() => sortDefects(filterDefects(rows, view), sort), [rows, view, sort]);

    // Every filter control funnels through here. The selection is dropped because a
    // bulk action must never land on rows the user has just filtered out of sight,
    // and the ?focus landing highlight goes with it — the user has started working.
    const afterFilterChange = useCallback(() => {
        setSelected(prev => (prev.size === 0 ? prev : new Set()));
        setFocusId(null);
        // The dead-deep-link notice is a landing message too, so it retires with the
        // highlight rather than following the user around the queue.
        setFocusMissing(false);
    }, []);

    // Clearing the two tile-only dimensions is what keeps them honest: they have no
    // control in the filter bar, so the moment someone filters by hand they must go,
    // or the table stays narrower than every count on the page claims.
    const clearTileDimensions = () => { setOpenOnly(false); setStaleOnly(false); };

    const changeQuery = (value) => { setQuery(value); afterFilterChange(); };
    const changeStatus = (value) => { setStatus(value); clearTileDimensions(); afterFilterChange(); };
    const changeSort = (value) => { setSort(value); afterFilterChange(); };

    const toggleSeverity = (severity) => {
        setSeverities(prev => (prev.includes(severity)
            ? prev.filter(s => s !== severity)
            : [...prev, severity]));
        clearTileDimensions();
        afterFilterChange();
    };

    // A tile sets every dimension its preset names (utils/defectQueue's
    // TRIAGE_TILES), including the two the filter bar cannot reach, so the table
    // lands on exactly the rows the tile counted. A typed query is left alone on
    // purpose: it is visible in the box, and silently discarding what someone typed
    // is worse than showing them a narrower queue than the tile's count promised.
    //
    // A pressed tile RELEASES instead — the tiles carry aria-pressed, so re-clicking
    // one has to mean "off". It is also the only honest exit from the Stale tile:
    // its preset sets nothing but the two invisible dimensions, so unlike Critical
    // (which leaves its severity chip pressed) nothing in the filter bar reflects
    // it, and the Reset button only exists inside the empty state.
    const pickTile = (preset, pressed) => {
        if (pressed) {
            setStatus('all');
            setSeverities([]);
            clearTileDimensions();
            afterFilterChange();
            return;
        }
        setStatus(preset.status);
        setSeverities(preset.severities);
        setOpenOnly(preset.openOnly === true);
        setStaleOnly(preset.stale === true);
        if (preset.sort) setSort(preset.sort);
        afterFilterChange();
    };

    const resetFilters = () => {
        setQuery('');
        setStatus('all');
        setSeverities([]);
        setSort(DEFAULT_SORT);
        clearTileDimensions();
        afterFilterChange();
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
        setNotice(null);
        setFocusId(null);
    }, []);

    // bulk-update hands back fully enriched rows (assignee_name AND linked_test_count), so
    // the response replaces the row outright — and an id that was sent but does NOT come
    // back is a defect someone else deleted since this page loaded, so its row goes with it
    // rather than sitting there unchanged after a "successful" apply. Both halves live in
    // applyBulkResult (utils/defectActions), where they are unit-tested.
    //
    // Nothing else has to be swept up behind a dropped row: its expand panel is rendered by the
    // row itself and goes when the row does. A dialog CAN be open while an apply is in flight —
    // an overlay is not inertness, and one opened before the apply started was never refused —
    // but it cannot write to a locked row (isLocked below), so a dialog left standing over a
    // dropped row can only be cancelled.
    const handleBulkApplied = useCallback((updated, requestedIds) => {
        // Remembered BEFORE the rows change, and never unset here: if this apply rewrote the
        // defect an edit dialog is standing open on, that dialog's snapshot is now wrong — and
        // stays wrong after the request has landed. Only the open dialog's id is recorded, and
        // only while it IS open; see rememberBulkTouched for why the wider version grew forever.
        rememberBulkTouched(bulkTouchedRef.current, editingIdRef.current, requestedIds);
        setRows(prev => applyBulkResult(prev, updated, requestedIds));
        // Cleared even on success: a closed defect usually leaves the queue it was
        // ticked in, and a selection of rows nobody can see is the hazard this page
        // clears selections to avoid in the first place. This is only ever the
        // selection the call was fired on — the bar locks it for the round-trip —
        // so nothing built in the meantime is thrown away here.
        setSelected(prev => (prev.size === 0 ? prev : new Set()));
    }, []);

    // PATCH /defects/{id} returns GetDefect, which resolves assignee_name but not
    // linked_test_count — carry the count over rather than blanking the Impact cell.
    const handleSaved = (saved) => {
        if (modal?.mode === 'edit') {
            setRows(prev => prev.map(r => (r.id === saved.id ? { ...saved, linked_test_count: r.linked_test_count } : r)));
            // An edit can push its own defect out of the queue on screen — closing it
            // from a tab that is not Closed, renaming it out of the search term, dropping
            // its severity below the pressed chip. A filter change drops the selection for
            // exactly this reason (afterFilterChange), so a row that has just left the view
            // leaves the selection with it: the bulk bar must never be able to write to a
            // defect nobody can see. Only this row can have moved, so only this id is checked.
            if (filterDefects([saved], view).length === 0) {
                setSelected(prev => {
                    if (!prev.has(saved.id)) return prev;
                    const next = new Set(prev);
                    next.delete(saved.id);
                    return next;
                });
            }
            return;
        }
        setRows(prev => [saved, ...prev]);
        // A new defect lands in whatever queue it belongs to, which is often not the
        // one on screen — creating from the Closed tab used to close the modal and
        // show nothing, which reads as "the save failed" and invites a duplicate.
        // Drop back to the unfiltered view instead, and highlight the new row.
        if (filterDefects([saved], view).length === 0) {
            setQuery('');
            setStatus('all');
            setSeverities([]);
            clearTileDimensions();
        }
        setSelected(prev => (prev.size === 0 ? prev : new Set()));
        setFocusId(saved.id);
    };

    // Delete is confirmed through components/Modal, not window.confirm: the native dialog
    // is unstyled, unthemed and unreachable from an e2e run, and it was the last one on
    // this page. The defect is named in the body so a mis-aimed Delete is visible before
    // it fires.
    const confirmDelete = () => {
        const defect = pendingDelete;
        setPendingDelete(null);
        if (!defect) return;
        // Guarded at the call, exactly like DefectModal.submit and for the same reason: this
        // confirmation can have been opened before the apply started, when the Delete button
        // that opens it was still live. DELETE would destroy the row the bulk call is mid-write
        // on. The refusal lands in that row's expand panel, which is the only place Delete can
        // be reached from and is therefore on screen.
        if (isLocked(defect.id)) {
            setNotice({ id: defect.id, message: BULK_LOCK_MESSAGE });
            return;
        }
        setNotice(null);
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

    // useCallback, like every other handler handed to DefectRow: the row is memoised,
    // and a fresh closure per render would defeat that on every keystroke.
    const handleRetest = useCallback((defect, tests) => {
        // Guarded at the call like the two writes above, though this one has no dialog of its
        // own to be reached from: its button lives in the row and is disabled while the row is
        // locked. It is guarded anyway because the point of this round is that a disabled
        // button is not a guarantee, and because Retest NAVIGATES — unmounting the page before
        // the in-flight apply can report at all.
        if (isLocked(defect?.id)) {
            setNotice({ id: defect.id, message: BULK_LOCK_MESSAGE });
            return;
        }
        const body = buildRetestRun(defect, tests);
        if (!body) return;
        setNotice(null);
        createTestRun(null, body.name, null, body.test_case_ids)
            .then(run => navigate(`/runs/run/${run.id}`))
            .catch(() => setNotice({
                id: defect.id,
                // POST /runs rejects the whole body when any id is unknown
                // (store.ErrUnknownTestCases), so one deleted test case fails the run.
                message: 'Could not start the retest run — one of the linked tests may no longer exist.',
            }));
    }, [isLocked, navigate]);

    // `target` comes from the row list as it stands now, which already includes every apply that
    // has landed — so the fresh snapshot openModal records is by definition not stale.
    const openDetail = useCallback((target) => {
        openModal({ mode: 'edit', defect: target });
    }, [openModal]);

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
                        onClick={() => openModal({ mode: 'create', defect: null })}
                        data-testid="defects-new"
                    >
                        + New defect
                    </button>
                </div>
            </div>

            {focusMissing && (
                <div className="defects-notice" role="status" data-testid="defects-focus-missing">
                    That defect no longer exists — it may have been deleted. The whole register is shown instead.
                </div>
            )}

            <TriageStrip counts={counts} view={view} onPick={pickTile} />

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
                onHeightChange={setBarHeight}
                onInFlightChange={handleBulkInFlight}
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
                                    disabled={visible.length === 0 || selectionLocked}
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
                                selectionLocked={selectionLocked}
                                // From the captured ids, never from `selected` — see bulkLock
                                // above. A boolean per row keeps DefectRow's memo intact.
                                actionsLocked={selectionLocked && bulkLock.has(defect.id)}
                                expanded={expandedId === defect.id}
                                focused={focusId === defect.id}
                                notice={notice?.id === defect.id ? notice.message : ''}
                                onToggleSelect={toggleSelect}
                                onToggleExpand={toggleExpand}
                                onOpenDetail={openDetail}
                                onRetest={handleRetest}
                                onDelete={setPendingDelete}
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

                {/* The bulk bar floats over the bottom of the queue, so while it is up
                    the scroller carries its height as extra scroll range — without it the
                    last row sits behind the bar with nowhere left to scroll. The height is
                    whatever the bar just measured (BulkBar's ResizeObserver), because the
                    bar wraps: a narrow viewport with an inline failure message on it is
                    several times taller than the one-row bar, and a constant that clears
                    one row leaves the last defect stranded behind the other. Until a
                    measurement arrives the stylesheet's fallback stands. */}
                {selected.size > 0 && (
                    <div
                        className="defects-bulk-spacer"
                        aria-hidden="true"
                        style={barHeight > 0 ? { '--defects-bulkbar-h': `${barHeight}px` } : undefined}
                    />
                )}
            </div>

            {/* Opened by "+ New defect" and by a row expand's "Open detail". */}
            {modal && (
                <DefectModal
                    key={modal.defect?.id || 'create'}
                    mode={modal.mode}
                    defect={modal.defect}
                    // Not a disabled prop: the modal asks this at the moment it saves, because
                    // it can have been opened before the apply that now owns this defect — and
                    // it stays true after that apply lands, for as long as this dialog holds the
                    // snapshot the apply invalidated.
                    isSnapshotStale={isEditStale}
                    onClose={closeModal}
                    onSaved={handleSaved}
                />
            )}

            {pendingDelete && (
                <Modal
                    type="confirm"
                    title="Delete Defect"
                    message={`Delete "${pendingDelete.title}"? Its links to test results and test cases go with it. This cannot be undone.`}
                    confirmText="Delete"
                    confirmStyle="danger"
                    onConfirm={confirmDelete}
                    onCancel={() => setPendingDelete(null)}
                />
            )}
        </div>
    );
}
