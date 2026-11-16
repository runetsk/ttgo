// Pure query-building for the server-side library grid (GET /api/tests/page).
// No React, no network. The grid holds filters as local-calendar day strings and
// column state; this maps them to the endpoint's query parameters, including the
// local-calendar → UTC conversion the server needs for date ranges.

// localRangeToUTC converts a {from, to} pair of local-calendar YYYY-MM-DD day
// strings (inclusive on both ends, as the old client-side filter treated them) to
// UTC instants for a lower-inclusive / upper-exclusive comparison: from → local
// midnight of the from day; to → local midnight of the day AFTER the to day. Using
// the Date(y, m, d) constructor applies the correct local offset for that specific
// date, so DST transitions are handled.
export function localRangeToUTC(range) {
    const out = {};
    if (range?.from) {
        const [y, m, d] = range.from.split('-').map(Number);
        out.from = new Date(y, m - 1, d, 0, 0, 0, 0).toISOString();
    }
    if (range?.to) {
        const [y, m, d] = range.to.split('-').map(Number);
        out.to = new Date(y, m - 1, d + 1, 0, 0, 0, 0).toISOString();
    }
    return out;
}

// buildTestPageParams maps the grid's live filter/sort/page state to the endpoint's
// query parameters. Empty filters are omitted so the server applies no constraint.
export function buildTestPageParams({ folderIds, search, columnFilters, sort, pageSize, page }) {
    const cf = columnFilters || {};
    const params = {
        folder_ids: (folderIds || []).join(','),
        sort: sort?.key || 'name',
        dir: sort?.direction === 'desc' ? 'desc' : 'asc',
        limit: pageSize,
        offset: Math.max(0, (page - 1) * pageSize),
    };
    if (search) params.search = search;
    if (cf.id) params.id = cf.id;
    if (cf.name) params.name = cf.name;
    if (cf.description) params.description = cf.description;
    if (cf.steps_count) params.steps_count = cf.steps_count;
    if (cf.open_defects) params.open_defects = cf.open_defects;
    if (cf.reverification_flagged) params.reverification = cf.reverification_flagged;
    if (cf.linked_requirements) params.linked_requirement = cf.linked_requirements;
    if (cf.categories && cf.categories.length) params.category_ids = cf.categories.join(',');

    const created = localRangeToUTC(cf.created_at);
    if (created.from) params.created_from = created.from;
    if (created.to) params.created_to = created.to;
    const updated = localRangeToUTC(cf.updated_at);
    if (updated.from) params.updated_from = updated.from;
    if (updated.to) params.updated_to = updated.to;

    return params;
}
