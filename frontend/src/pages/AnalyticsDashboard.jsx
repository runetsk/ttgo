import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
    getAnalyticsSummary,
    getAnalyticsTrend,
    getAnalyticsFlaky,
    getAnalyticsMostFailed,
    getAnalyticsDuration,
    getAnalyticsDurationTop,
    getAnalyticsComponentHealth,
    getAnalyticsGrowth,
    getAnalyticsPassingRate,
    getAnalyticsUniqueBugs,
    getAnalyticsActivity,
} from '../api';
import AnalyticsFilters from '../components/analytics/AnalyticsFilters';
import CollapsibleSection from '../components/analytics/CollapsibleSection';
import SummaryCards from '../components/analytics/SummaryCards';
import PassingRateSummary from '../components/analytics/PassingRateSummary';
import TrendChart from '../components/analytics/TrendChart';
import MostFailedTable from '../components/analytics/MostFailedTable';
import FlakyTestsTable from '../components/analytics/FlakyTestsTable';
import DurationChart from '../components/analytics/DurationChart';
import TimeConsumingList from '../components/analytics/TimeConsumingList';
import ComponentHealth from '../components/analytics/ComponentHealth';
import GrowthChart from '../components/analytics/GrowthChart';
import PassingRatePerFolder from '../components/analytics/PassingRatePerFolder';
import UniqueBugsTable from '../components/analytics/UniqueBugsTable';
import RunComparison from '../components/analytics/RunComparison';
import ActivityPanel from '../components/analytics/ActivityPanel';
import { formatDate } from '../components/analytics/utils';

function defaultFilters() {
    const now = new Date();
    const start = new Date();
    start.setDate(now.getDate() - 30);
    return {
        startDate: formatDate(start),
        endDate: formatDate(now),
        folderId: '',
    };
}

function toParams(filters) {
    const p = {};
    if (filters.startDate) p.start_date = filters.startDate;
    if (filters.endDate) p.end_date = filters.endDate;
    if (filters.folderId) p.folder_id = filters.folderId;
    return p;
}

export default function AnalyticsDashboard() {
    const [filters, setFilters] = useState(defaultFilters);

    // P1 section data (loaded on mount)
    const [summary, setSummary] = useState(null);
    const [trend, setTrend] = useState(null);
    const [mostFailed, setMostFailed] = useState(null);
    const [flaky, setFlaky] = useState(null);

    // P2/P3 section data (lazy loaded)
    const [duration, setDuration] = useState(null);
    const [durationTop, setDurationTop] = useState(null);
    const [componentHealth, setComponentHealth] = useState(null);
    const [growth, setGrowth] = useState(null);
    const [passingRate, setPassingRate] = useState(null);
    const [uniqueBugs, setUniqueBugs] = useState(null);
    const [activity, setActivity] = useState(null);

    // Loading/error states
    const [p1Loading, setP1Loading] = useState(true);
    const [p1Error, setP1Error] = useState(null);
    const [lazyErrors, setLazyErrors] = useState({});

    // Track which lazy sections have been requested
    const lazyLoaded = useRef({
        duration: false,
        componentHealth: false,
        growth: false,
        passingRate: false,
        uniqueBugs: false,
        activity: false,
    });

    // AbortController ref for request cancellation (T025)
    const abortRef = useRef(null);

    const fetchP1 = useCallback((f, signal) => {
        const params = toParams(f);
        setP1Loading(true);
        setP1Error(null);

        Promise.all([
            getAnalyticsSummary(params, signal),
            getAnalyticsTrend(params, signal),
            getAnalyticsMostFailed(params, signal),
            getAnalyticsFlaky(params, signal),
        ])
            .then(([s, t, mf, fl]) => {
                setSummary(s);
                setTrend(t);
                setMostFailed(mf);
                setFlaky(fl);
                setP1Loading(false);
            })
            .catch(err => {
                if (err?.name === 'CanceledError' || err?.code === 'ERR_CANCELED') return;
                setP1Error(err?.message || 'Failed to load analytics');
                setP1Loading(false);
            });
    }, []);

    const fetchLazySection = useCallback((section, f, signal) => {
        const params = toParams(f);
        const handleErr = (err) => {
            if (err?.name === 'CanceledError' || err?.code === 'ERR_CANCELED') return;
            setLazyErrors(prev => ({ ...prev, [section]: err?.message || 'Failed to load' }));
        };
        setLazyErrors(prev => ({ ...prev, [section]: null }));
        switch (section) {
            case 'duration':
                Promise.all([
                    getAnalyticsDuration(params, signal),
                    getAnalyticsDurationTop(params, signal),
                ]).then(([d, dt]) => {
                    setDuration(d);
                    setDurationTop(dt);
                }).catch(handleErr);
                break;
            case 'componentHealth':
                getAnalyticsComponentHealth(params, signal)
                    .then(setComponentHealth)
                    .catch(handleErr);
                break;
            case 'growth':
                getAnalyticsGrowth(params, signal)
                    .then(setGrowth)
                    .catch(handleErr);
                break;
            case 'passingRate':
                getAnalyticsPassingRate(params, signal)
                    .then(setPassingRate)
                    .catch(handleErr);
                break;
            case 'uniqueBugs':
                getAnalyticsUniqueBugs(params, signal)
                    .then(setUniqueBugs)
                    .catch(handleErr);
                break;
            case 'activity':
                getAnalyticsActivity(params, signal)
                    .then(setActivity)
                    .catch(handleErr);
                break;
        }
    }, []);

    // Initial load + filter changes with AbortController cancellation (T025)
    useEffect(() => {
        if (abortRef.current) abortRef.current.abort();
        const controller = new AbortController();
        abortRef.current = controller;

        // eslint-disable-next-line react-hooks/set-state-in-effect -- async load result: fetchP1 fetches the P1 analytics sections (with AbortController cancellation) on mount/filter change
        fetchP1(filters, controller.signal);

        // Re-fetch any lazy sections that were already opened
        Object.entries(lazyLoaded.current).forEach(([section, loaded]) => {
            if (loaded) fetchLazySection(section, filters, controller.signal);
        });

        return () => controller.abort();
    }, [filters, fetchP1, fetchLazySection]);

    const handleFilterChange = useCallback((newFilters) => {
        setFilters(newFilters);
    }, []);

    const handleFirstExpand = useCallback((section) => {
        lazyLoaded.current[section] = true;
        fetchLazySection(section, filters, abortRef.current?.signal);
    }, [filters, fetchLazySection]);

    // Trend time range change handler
    const handleTimeRangeChange = useCallback((days) => {
        const now = new Date();
        const start = new Date();
        start.setDate(now.getDate() - days);
        setFilters(prev => ({
            ...prev,
            startDate: formatDate(start),
            endDate: formatDate(now),
        }));
    }, []);

    // Passing rate exclude skipped toggle — re-fetch with param
    const handleExcludeSkippedChange = useCallback((excludeSkipped) => {
        const params = { ...toParams(filters), exclude_skipped: excludeSkipped };
        getAnalyticsPassingRate(params)
            .then(setPassingRate)
            .catch(() => {});
    }, [filters]);

    return (
        <div style={{ padding: 24 }}>
            <h2 style={{ marginBottom: 20 }}>Analytics Dashboard</h2>

            <AnalyticsFilters value={filters} onChange={handleFilterChange} />

            {p1Error && (
                <div style={{ padding: 16, color: 'var(--accent-red)', marginBottom: 16 }}>
                    Error: {p1Error}
                </div>
            )}

            {/* P1: Summary (expanded) — T014 */}
            <CollapsibleSection
                title="Executive Summary"
                description="High-level overview of total runs, pass/fail/skip counts, and overall passing rate for the selected date range."
                defaultExpanded={true}
            >
                {p1Loading ? (
                    <div className="analytics-loading">Loading summary...</div>
                ) : (
                    <>
                        <SummaryCards data={summary} />
                        <PassingRateSummary data={summary} />
                    </>
                )}
            </CollapsibleSection>

            {/* P1: Trends (expanded) — T017 */}
            <CollapsibleSection
                title="Run Statistics Trend"
                description="Daily time-series chart showing passed, failed, and skipped test counts over time. Use the time range buttons to zoom in or out."
                defaultExpanded={true}
            >
                {p1Loading ? (
                    <div className="analytics-loading">Loading trends...</div>
                ) : (
                    <TrendChart data={trend} onTimeRangeChange={handleTimeRangeChange} />
                )}
            </CollapsibleSection>

            {/* P1: Most Failed & Flaky (expanded) — T024 */}
            <CollapsibleSection
                title="Most Failed & Flaky Tests"
                description="Top tests ranked by failure count and flakiness score. Flaky tests are detected by status-switch frequency — how often a test alternates between pass and fail across consecutive runs."
                defaultExpanded={true}
            >
                {p1Loading ? (
                    <div className="analytics-loading">Loading test analysis...</div>
                ) : (
                    <div className="analytics-two-col">
                        <div className="analytics-chart-container">
                            <h4 style={{ margin: '0 0 12px', color: 'var(--text-primary)' }}>
                                Most Failed ({mostFailed?.total || 0})
                            </h4>
                            <MostFailedTable data={mostFailed} />
                        </div>
                        <div className="analytics-chart-container">
                            <h4 style={{ margin: '0 0 12px', color: 'var(--text-primary)' }}>
                                Flaky Tests ({flaky?.total || 0})
                            </h4>
                            <FlakyTestsTable data={flaky} />
                        </div>
                    </div>
                )}
            </CollapsibleSection>

            {/* P2: Duration (collapsed, lazy) — T031 */}
            <CollapsibleSection
                title="Execution Duration"
                description="Average and total execution time trends per day, plus a ranked list of the slowest individual test cases. Helps identify performance regressions."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('duration')}
            >
                {lazyErrors.duration ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.duration}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('duration', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : duration ? (
                    <>
                        <DurationChart data={duration} />
                        <TimeConsumingList data={durationTop} />
                    </>
                ) : (
                    <div className="analytics-loading">Loading duration data...</div>
                )}
            </CollapsibleSection>

            {/* P2: Component Health (collapsed, lazy) — T035 */}
            <CollapsibleSection
                title="Component Health"
                description="Passing rate breakdown by folder/component. Components below the threshold are flagged as unhealthy. Shows pass, fail, and skip distribution per component."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('componentHealth')}
            >
                {lazyErrors.componentHealth ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.componentHealth}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('componentHealth', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : componentHealth ? (
                    <ComponentHealth
                        data={componentHealth}
                        threshold={componentHealth?.threshold}
                    />
                ) : (
                    <div className="analytics-loading">Loading component health...</div>
                )}
            </CollapsibleSection>

            {/* P3: Growth (collapsed, lazy) — T039 */}
            <CollapsibleSection
                title="Test Case Growth"
                description="Cumulative count of test cases over time, showing how the test library is growing. The delta indicates how many new tests were added each day."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('growth')}
            >
                {lazyErrors.growth ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.growth}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('growth', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : growth ? (
                    <GrowthChart data={growth} />
                ) : (
                    <div className="analytics-loading">Loading growth data...</div>
                )}
            </CollapsibleSection>

            {/* P3: Passing Rate Per Folder (collapsed, lazy) — T043 */}
            <CollapsibleSection
                title="Passing Rate Per Folder"
                description="Per-folder pass rate with bar visualization. Toggle 'Exclude Skipped' to calculate rates based only on passed and failed results."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('passingRate')}
            >
                {lazyErrors.passingRate ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.passingRate}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('passingRate', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : passingRate ? (
                    <PassingRatePerFolder
                        data={passingRate}
                        onExcludeSkippedChange={handleExcludeSkippedChange}
                    />
                ) : (
                    <div className="analytics-loading">Loading passing rate data...</div>
                )}
            </CollapsibleSection>

            {/* P3: Unique Bugs (collapsed, lazy) */}
            <CollapsibleSection
                title="Unique Bugs"
                description="Defects linked to test cases. Shows status, severity, and how many test cases reference each bug."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('uniqueBugs')}
            >
                {lazyErrors.uniqueBugs ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.uniqueBugs}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('uniqueBugs', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : uniqueBugs ? (
                    <UniqueBugsTable data={uniqueBugs} />
                ) : (
                    <div className="analytics-loading">Loading bugs data...</div>
                )}
            </CollapsibleSection>

            {/* P3: Run Comparison (collapsed, self-contained) */}
            <CollapsibleSection
                title="Run Comparison"
                description="Side-by-side comparison of two test runs. Select any two runs to see differences in pass/fail/skip counts and identify regressions or improvements."
                defaultExpanded={false}
            >
                <RunComparison />
            </CollapsibleSection>

            {/* P3: Activity Panel (collapsed, lazy) */}
            <CollapsibleSection
                title="Project Activity"
                description="Recent actions in the project — test case creation, updates, run submissions, and more. Shows who did what and when."
                defaultExpanded={false}
                onFirstExpand={() => handleFirstExpand('activity')}
            >
                {lazyErrors.activity ? (
                    <div className="analytics-empty">
                        <div className="analytics-empty-text">Error: {lazyErrors.activity}</div>
                        <button className="action-btn" onClick={() => fetchLazySection('activity', filters, abortRef.current?.signal)} type="button">Retry</button>
                    </div>
                ) : activity ? (
                    <ActivityPanel data={activity} />
                ) : (
                    <div className="analytics-loading">Loading activity...</div>
                )}
            </CollapsibleSection>
        </div>
    );
}
