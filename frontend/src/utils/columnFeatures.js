/**
 * Filters column definitions down to those whose feature flag is enabled.
 * A def with no `feature` field is always kept. `flags` maps feature name → boolean.
 *   activeColumns(defs, { qtest: false, ai: true })
 */
export function activeColumns(defs, flags = {}) {
    return defs.filter(c => !c.feature || flags[c.feature]);
}
