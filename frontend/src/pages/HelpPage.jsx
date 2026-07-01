import React, { useState, useRef } from 'react';

/* ── Section config with icons ── */
const SECTIONS = [
    { id: 'overview', label: 'Overview', icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0h4' },
    { id: 'test-cases', label: 'Test Cases', icon: 'M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4' },
    { id: 'test-runs', label: 'Test Runs', icon: 'M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664zM21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
    { id: 'categories', label: 'Categories', icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10' },
    { id: 'requirements', label: 'Requirements', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
    { id: 'ai-generation', label: 'AI Generation', icon: 'M13 10V3L4 14h7v7l9-11h-7z' },
    { id: 'ai-import', label: 'AI Import', icon: 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12' },
    { id: 'analytics', label: 'Analytics', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
    { id: 'traceability', label: 'Traceability', icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1' },
    { id: 'backups', label: 'Backups', icon: 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4' },
    { id: 'cli', label: 'CLI', icon: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
    { id: 'settings', label: 'Settings', icon: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
    { id: 'shortcuts', label: 'Tips & Shortcuts', icon: 'M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z' },
];

function SectionIcon({ d, size = 16, color = 'currentColor' }) {
    return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
            <path d={d}/>
        </svg>
    );
}

export default function HelpPage() {
    const [activeSection, setActiveSection] = useState('overview');
    const [search, setSearch] = useState('');
    const [hoveredNav, setHoveredNav] = useState(null);
    const contentRef = useRef(null);

    const switchSection = (id) => {
        setActiveSection(id);
        setSearch('');
        contentRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
    };

    return (
        <div style={styles.page}>
            {/* ── Sidebar ── */}
            <nav style={styles.sidebar}>
                <div style={styles.sidebarTop}>
                    <div style={styles.sidebarBrand}>
                        <div style={styles.brandIcon}>
                            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                                <circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><line x1="12" y1="17" x2="12.01" y2="17"/>
                            </svg>
                        </div>
                        <span style={styles.brandText}>User Guide</span>
                    </div>

                    {/* Search */}
                    <div style={styles.searchWrap}>
                        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={styles.searchIcon}>
                            <circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/>
                        </svg>
                        <input
                            value={search}
                            onChange={e => setSearch(e.target.value)}
                            placeholder="Search docs..."
                            style={styles.searchInput}
                        />
                        {search && (
                            <button onClick={() => setSearch('')} style={styles.searchClear}>
                                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                            </button>
                        )}
                    </div>
                </div>

                <div style={styles.navList}>
                    {SECTIONS.map(sec => {
                        const isActive = activeSection === sec.id;
                        const isHovered = hoveredNav === sec.id;
                        return (
                            <button
                                key={sec.id}
                                onClick={() => switchSection(sec.id)}
                                onMouseEnter={() => setHoveredNav(sec.id)}
                                onMouseLeave={() => setHoveredNav(null)}
                                style={{
                                    ...styles.navItem,
                                    ...(isActive ? styles.navItemActive : {}),
                                    ...(!isActive && isHovered ? styles.navItemHover : {}),
                                }}
                            >
                                <SectionIcon d={sec.icon} size={15} color={isActive ? '#fff' : 'var(--text-secondary)'} />
                                <span>{sec.label}</span>
                            </button>
                        );
                    })}
                </div>

                <div style={styles.sidebarFooter}>
                    <span style={styles.footerText}>TTGO v1.0</span>
                </div>
            </nav>

            {/* ── Content ── */}
            <div style={styles.content} ref={contentRef}>
                <div style={styles.contentInner}>
                    {activeSection === 'overview' && <OverviewSection onNavigate={switchSection} />}
                    {activeSection === 'test-cases' && <TestCasesSection />}
                    {activeSection === 'test-runs' && <TestRunsSection />}
                    {activeSection === 'categories' && <CategoriesSection />}
                    {activeSection === 'requirements' && <RequirementsSection />}
                    {activeSection === 'ai-generation' && <AIGenerationSection />}
                    {activeSection === 'ai-import' && <AIImportSection />}
                    {activeSection === 'analytics' && <AnalyticsSection />}
                    {activeSection === 'traceability' && <TraceabilitySection />}
                    {activeSection === 'backups' && <BackupsSection />}
                    {activeSection === 'cli' && <CLISection />}
                    {activeSection === 'settings' && <SettingsSection />}
                    {activeSection === 'shortcuts' && <ShortcutsSection />}
                </div>
            </div>

            <style>{`
                .help-nav-item:hover { background: var(--bg-tertiary) !important; }
                .help-card { transition: border-color 0.15s ease, box-shadow 0.15s ease; }
                .help-card:hover { border-color: rgba(99,102,241,0.25) !important; box-shadow: 0 4px 16px rgba(0,0,0,0.15) !important; }
                .help-feature-card { transition: transform 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease; }
                .help-feature-card:hover { transform: translateY(-2px); border-color: rgba(99,102,241,0.3) !important; box-shadow: 0 8px 24px rgba(0,0,0,0.2) !important; cursor: pointer; }
                .help-step-number { transition: background 0.15s ease; }
                .help-kbd { font-family: monospace; font-size: 0.72rem; padding: 2px 6px; border-radius: 4px; background: var(--bg-tertiary); border: 1px solid var(--border-color); color: var(--text-primary); }
            `}</style>
        </div>
    );
}

/* ── SECTION COMPONENTS ── */

function OverviewSection({ onNavigate }) {
    return (
        <div>
            {/* Hero */}
            <div style={styles.hero}>
                <div style={styles.heroGlow} />
                <div style={styles.heroContent}>
                    <div style={styles.heroBadge}>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 2L2 7l10 5 10-5-10-5z"/><path d="M2 17l10 5 10-5"/><path d="M2 12l10 5 10-5"/>
                        </svg>
                        Documentation
                    </div>
                    <h1 style={styles.heroTitle}>Welcome to TTGO</h1>
                    <p style={styles.heroSubtitle}>
                        Modern test case management for QA teams. Organize, execute, and track
                        your testing with AI-powered generation and full traceability.
                    </p>
                </div>
            </div>

            {/* Quick links grid */}
            <div style={styles.quickGrid}>
                {[
                    { id: 'test-cases', icon: SECTIONS[1].icon, title: 'Test Cases', desc: 'Create, organize, and version test cases in folders' },
                    { id: 'test-runs', icon: SECTIONS[2].icon, title: 'Test Runs', desc: 'Execute tests and record pass/fail results' },
                    { id: 'ai-generation', icon: SECTIONS[5].icon, title: 'AI Generation', desc: 'Generate test cases from requirements using LLMs' },
                    { id: 'ai-import', icon: SECTIONS[6].icon, title: 'AI Import', desc: 'Import AI output from ChatGPT, Gemini, Claude' },
                    { id: 'requirements', icon: SECTIONS[4].icon, title: 'Requirements', desc: 'Link tests to requirements for traceability' },
                    { id: 'analytics', icon: SECTIONS[7].icon, title: 'Analytics', desc: 'Dashboards, trends, and flaky test detection' },
                ].map(item => (
                    <button
                        key={item.id}
                        onClick={() => onNavigate(item.id)}
                        className="help-feature-card"
                        style={styles.quickCard}
                    >
                        <div style={styles.quickCardIcon}>
                            <SectionIcon d={item.icon} size={20} color="var(--accent-indigo, #6366f1)" />
                        </div>
                        <div style={styles.quickCardText}>
                            <div style={styles.quickCardTitle}>{item.title}</div>
                            <div style={styles.quickCardDesc}>{item.desc}</div>
                        </div>
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--text-secondary)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, opacity: 0.5 }}>
                            <polyline points="9 18 15 12 9 6"/>
                        </svg>
                    </button>
                ))}
            </div>

            {/* Core concepts */}
            <SectionHeader>Core Concepts</SectionHeader>
            <Card>
                <DL items={[
                    ['Test Cases', 'Individual test scenarios organized in folders. Each has a name, description, priority, and steps with actions and expected results.'],
                    ['Folders', 'Hierarchical containers for organizing test cases. Support drag-and-drop reordering and unlimited nesting.'],
                    ['Test Runs', 'Execution sessions where test cases are run and results (Pass, Fail, Blocked, Skipped) are recorded.'],
                    ['Categories', 'Reusable collections of test cases for regression, smoke testing, or feature validation.'],
                    ['Requirements', 'Business or functional requirements linked to test cases for full coverage traceability.'],
                ]} />
            </Card>

            {/* Quick start */}
            <SectionHeader>Quick Start</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Create folders', desc: 'Organize your test cases (Tests tab > right-click sidebar > New Folder)' },
                    { title: 'Add test cases', desc: 'Manually, via AI generation, or by importing from an external LLM' },
                    { title: 'Create a category', desc: 'Group related test cases for regression or feature testing' },
                    { title: 'Run tests', desc: 'Start a test run from a category and record pass/fail results' },
                    { title: 'Track coverage', desc: 'Use analytics and the traceability matrix to monitor progress' },
                ]} />
            </Card>
        </div>
    );
}

function TestCasesSection() {
    return (
        <div>
            <PageHeader title="Test Cases" desc="The core building blocks of your testing effort. Each test case lives inside a folder and contains structured steps." />

            <SectionHeader>Creating Test Cases</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Navigate to a folder', desc: 'Select a folder in the sidebar to target your new test case' },
                    { title: 'Click "+ New Test Case"', desc: 'Open the creation form from the toolbar' },
                    { title: 'Fill in details', desc: 'Name, description, and priority' },
                    { title: 'Add steps', desc: 'Define actions and expected results for each step' },
                    { title: 'Save', desc: 'Your test case is created with an automatic version snapshot' },
                ]} />
                <Tip>You can also generate test cases using AI or import them from external LLMs.</Tip>
            </Card>

            <SectionHeader>Editing & Versioning</SectionHeader>
            <Card>
                <P>Click any test case in the grid to open its detail view. All edits are tracked automatically.</P>
                <DL items={[
                    ['Inline Editing', 'Click any field in the detail view to edit directly. Changes save automatically.'],
                    ['Rich Text', 'Description fields support bold, italic, links, and lists.'],
                    ['Version History', 'Open the History sidebar to see all past versions with diffs.'],
                ]} />
            </Card>

            <SectionHeader>Folders & Grid</SectionHeader>
            <Card>
                <UL items={[
                    'Right-click in the sidebar to create, rename, or delete folders.',
                    'Drag and drop folders to reorder or nest them.',
                    'Drag test cases between folders to move them.',
                    'Sort by any column, search/filter, and select multiple for bulk operations.',
                    'Customize visible columns via the column selector button.',
                ]} />
            </Card>
        </div>
    );
}

function TestRunsSection() {
    return (
        <div>
            <PageHeader title="Test Runs" desc="Execution sessions where you step through test cases and record outcomes." />

            <SectionHeader>Creating a Run</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Go to the Runs tab', desc: 'Navigate to the Runs page from the top nav' },
                    { title: 'Click "+ New Run"', desc: 'Open the creation dialog' },
                    { title: 'Configure', desc: 'Name the run and select a category or individual test cases' },
                    { title: 'Create', desc: 'Start the run and begin execution' },
                ]} />
            </Card>

            <SectionHeader>Executing Tests</SectionHeader>
            <Card>
                <P>Open a run to see its test cases. For each one, expand the steps, execute, and set the status.</P>
                <Tip>When a test fails, you can create a defect directly from the run detail and link it to Jira.</Tip>
            </Card>

            <SectionHeader>Run Statuses</SectionHeader>
            <Card>
                <div style={styles.statusGrid}>
                    {[
                        { name: 'Pass', color: '#22c55e', bg: 'rgba(34,197,94,0.1)', desc: 'Actual results match expected' },
                        { name: 'Fail', color: '#ef4444', bg: 'rgba(239,68,68,0.1)', desc: 'Defect found — results differ' },
                        { name: 'Blocked', color: '#eab308', bg: 'rgba(234,179,8,0.1)', desc: 'External dependency blocking' },
                        { name: 'Skipped', color: '#8b5cf6', bg: 'rgba(139,92,246,0.1)', desc: 'Intentionally not executed' },
                        { name: 'Not Run', color: 'var(--text-secondary)', bg: 'var(--bg-tertiary)', desc: 'Default — not yet executed' },
                    ].map(s => (
                        <div key={s.name} style={{ ...styles.statusItem, background: s.bg, borderColor: s.color + '33' }}>
                            <div style={{ ...styles.statusDot, background: s.color }} />
                            <div>
                                <div style={{ ...styles.statusName, color: s.color }}>{s.name}</div>
                                <div style={styles.statusDesc}>{s.desc}</div>
                            </div>
                        </div>
                    ))}
                </div>
            </Card>
        </div>
    );
}

function CategoriesSection() {
    return (
        <div>
            <PageHeader title="Categories" desc="Reusable groupings of test cases. Use them as templates for test runs." />
            <Card>
                <UL items={[
                    'Navigate to the Categories tab to see all categories.',
                    'Click "+ New Category" to create one.',
                    'Add test cases from any folder into the category.',
                    'Reorder test cases within the category via drag-and-drop.',
                    'Use categories when creating test runs to auto-populate with the category\'s test cases.',
                ]} />
            </Card>
        </div>
    );
}

function RequirementsSection() {
    return (
        <div>
            <PageHeader title="Requirements" desc="Business or functional specs that link to test cases for full traceability." />

            <SectionHeader>Creating Requirements</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Go to the Reqs tab', desc: 'Open the Requirements page' },
                    { title: 'Click "+ New Requirement"', desc: 'Open the creation form' },
                    { title: 'Fill in details', desc: 'Identifier (e.g. REQ-001), title, description, priority, status' },
                    { title: 'Save', desc: 'Your requirement is ready for linking' },
                ]} />
            </Card>

            <SectionHeader>Linking to Test Cases</SectionHeader>
            <Card>
                <UL items={[
                    'From a requirement detail page, use the "Link Test Cases" panel.',
                    'From a test case detail, use the requirement linking panel.',
                    'When generating or importing AI test cases, select a requirement to auto-link.',
                    'View all links in the Traceability Matrix.',
                ]} />
            </Card>

            <SectionHeader>External Import</SectionHeader>
            <Card>
                <UL items={[
                    'Import from Jira — pull issues as requirements and keep them synced.',
                    'Import from Confluence — extract requirements from pages.',
                    'Configure connections in Settings > Integrations.',
                ]} />
            </Card>
        </div>
    );
}

function AIGenerationSection() {
    return (
        <div>
            <PageHeader title="AI Test Generation" desc="Generate structured test cases from requirements using LLM providers." />

            <SectionHeader>Setup</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Open Settings > AI Configuration', desc: 'Navigate to the AI settings panel' },
                    { title: 'Select your LLM provider', desc: 'OpenAI, Anthropic, Google Gemini, etc.' },
                    { title: 'Enter your API key', desc: 'Paste the key from your provider dashboard' },
                    { title: 'Save', desc: 'Choose model and adjust settings as needed' },
                ]} />
            </Card>

            <SectionHeader>Generating Tests</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Navigate to AI Gen', desc: 'The landing page shows two pathways: Generate and Import. Choose Generate.' },
                    { title: 'Select a requirement', desc: 'Pick from the requirement list or create a new one directly' },
                    { title: 'Configure options', desc: 'Choose destination folder, LLM provider, count, and detail level' },
                    { title: 'Click "Generate"', desc: 'Wait for the AI to produce test case drafts' },
                    { title: 'Review & edit', desc: 'Modify names, steps, and descriptions as needed' },
                    { title: 'Accept', desc: 'Selected drafts are created in the target folder with version history' },
                ]} />
                <Tip>Select/deselect individual drafts before accepting. Only selected ones are created.</Tip>
            </Card>
        </div>
    );
}

function AIImportSection() {
    return (
        <div>
            <PageHeader title="AI Import" desc="Paste or upload AI-generated test cases from ChatGPT, Gemini, Claude, or any LLM." />

            <SectionHeader>Supported Formats</SectionHeader>
            <Card>
                <div style={styles.formatGrid}>
                    {[
                        { name: 'JSON', tag: '.json', desc: 'Array of objects with name, description, steps. Supports wrappers and markdown fences.' },
                        { name: 'Markdown Table', tag: '.md', desc: 'Pipe-delimited tables with Name, Action, Expected Result headers.' },
                        { name: 'Numbered List', tag: '.txt', desc: 'Numbered items as test cases, sub-items as steps with arrow separators.' },
                        { name: 'CSV', tag: '.csv', desc: 'Comma-separated values with header row. Multi-row test cases supported.' },
                    ].map(f => (
                        <div key={f.name} style={styles.formatCard}>
                            <div style={styles.formatHeader}>
                                <span style={styles.formatName}>{f.name}</span>
                                <code style={styles.formatTag}>{f.tag}</code>
                            </div>
                            <div style={styles.formatDesc}>{f.desc}</div>
                        </div>
                    ))}
                </div>
            </Card>

            <SectionHeader>Import via Paste</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Open AI Gen > Import', desc: 'Click "Start Import" on the landing page, or switch to the Import tab' },
                    { title: 'Paste content', desc: 'Paste your AI-generated output into the text area' },
                    { title: 'Parse', desc: 'Click Parse — format is auto-detected (or override manually)' },
                    { title: 'Review', desc: 'Edit names, steps, and descriptions on the review cards' },
                    { title: 'Accept', desc: 'Choose a folder, optionally link a requirement, and accept' },
                ]} />
            </Card>

            <SectionHeader>Import via File</SectionHeader>
            <Card>
                <UL items={[
                    'Click "Upload File" or drag and drop onto the text area.',
                    'Supported types: .txt, .md, .csv, .json (max 1 MB).',
                    'File content loads into the text area for parsing.',
                ]} />
            </Card>

            <SectionHeader>Review Panel</SectionHeader>
            <Card>
                <UL items={[
                    'Edit test case names, descriptions, and step details inline.',
                    'Yellow "Duplicate" badges warn on name matches (non-blocking).',
                    'Orange badges indicate missing steps or expected results.',
                    'Unparseable content shown in a collapsible section.',
                    'Maximum 50 test cases per import session.',
                ]} />
                <Tip>If a requirement is selected on the Generate tab, it auto-populates in the Import requirement dropdown.</Tip>
            </Card>
        </div>
    );
}

function AnalyticsSection() {
    return (
        <div>
            <PageHeader title="Analytics" desc="Visibility into test execution trends, pass/fail rates, and quality metrics." />
            <Card>
                <UL items={[
                    'Pass/Fail/Blocked/Skipped breakdown by run.',
                    'Trend charts showing execution results over time.',
                    'Flaky test detection — identifies tests that alternate between pass and fail.',
                    'Run comparison — compare results across different test runs.',
                    'Filter by date range, folder, or category.',
                ]} />
            </Card>
        </div>
    );
}

function TraceabilitySection() {
    return (
        <div>
            <PageHeader title="Traceability Matrix" desc="Cross-reference view of requirements and their linked test cases." />
            <Card>
                <UL items={[
                    'Each row represents a requirement.',
                    'Columns show linked test cases with their latest execution status.',
                    'Color coding: green (passed), red (failed), yellow (blocked), gray (not run).',
                    'Click any cell to navigate to the test case or requirement detail.',
                    'Use filters to focus on specific requirements or coverage states.',
                ]} />
            </Card>
        </div>
    );
}

function BackupsSection() {
    return (
        <div>
            <PageHeader title="Database Backups" desc="Create, restore, and schedule backups of your TTGO database. Admin-only feature." />

            <SectionHeader>Creating a Backup</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Navigate to Admin > Backups', desc: 'Open the Backups page from the admin section in the navigation bar' },
                    { title: 'Click "Create Backup"', desc: 'A manual backup starts immediately — the database is checkpointed and copied' },
                    { title: 'Wait for completion', desc: 'The backup appears in the list with status, file size, and timestamp' },
                ]} />
                <Tip>Backups capture the entire SQLite database file. A disk space check runs before each backup to prevent failures.</Tip>
            </Card>

            <SectionHeader>Managing Backups</SectionHeader>
            <Card>
                <UL items={[
                    'View all backups (manual and automatic) in the Backups tab with status, size, and creator.',
                    'Download any backup to your local machine for off-site storage.',
                    'Delete old backups you no longer need.',
                    'Each backup is validated with SQLite header and table existence checks.',
                ]} />
            </Card>

            <SectionHeader>Restoring from a Backup</SectionHeader>
            <Card>
                <P>Restore replaces the current database with a backup copy. The system enters maintenance mode during restore, blocking all other API requests.</P>
                <Steps items={[
                    { title: 'Go to the Restore tab', desc: 'Choose between restoring from a server-stored backup or uploading a file' },
                    { title: 'Select or upload', desc: 'Pick a backup from the list, or upload a .db file from your machine' },
                    { title: 'Confirm restore', desc: 'Type "CONFIRM RESTORE" in the confirmation dialog to proceed' },
                    { title: 'Wait for completion', desc: 'A safety backup is created automatically before the restore begins' },
                ]} />
                <Tip>A pre-restore safety backup is always created so you can roll back if needed. Uploaded files are validated before restore.</Tip>
            </Card>

            <SectionHeader>Scheduled Backups</SectionHeader>
            <Card>
                <P>Configure automatic backups on a recurring schedule with a retention policy.</P>
                <DL items={[
                    ['Enable/Disable', 'Toggle scheduled backups on or off from the Schedule tab.'],
                    ['Interval', 'Set how often backups run (in hours, e.g. every 24 hours).'],
                    ['Retention', 'Set how many automatic backups to keep. Oldest are deleted when the limit is exceeded.'],
                ]} />
                <Tip>The scheduler checks every minute and runs a backup when the interval has elapsed since the last run.</Tip>
            </Card>

            <SectionHeader>Maintenance Mode</SectionHeader>
            <Card>
                <UL items={[
                    'During a restore, the system enters maintenance mode automatically.',
                    'All non-backup API requests are blocked with a maintenance message.',
                    'A full-screen overlay appears in the UI during maintenance.',
                    'Maintenance mode is lifted automatically once the restore completes.',
                ]} />
            </Card>
        </div>
    );
}

function CLISection() {
    return (
        <div>
            <PageHeader title="CLI" desc="Manage tests, runs, analytics, and more from the command line with the ttgo CLI." />

            <SectionHeader>Installation</SectionHeader>
            <Card>
                <Steps items={[
                    { title: 'Build the CLI', desc: 'Run go build -o ttgo ./cmd/ttgo/ from the backend directory' },
                    { title: 'Configure server URL', desc: 'ttgo config set-server http://localhost:8080' },
                    { title: 'Set your API token', desc: 'ttgo config set-token <token> — create a token in Settings > API Tokens' },
                    { title: 'Verify', desc: 'Run ttgo config show to confirm your configuration' },
                ]} />
            </Card>

            <SectionHeader>Available Commands</SectionHeader>
            <Card>
                <DL items={[
                    ['ttgo tests', 'List, create, update, delete, view versions, restore, and check executions for test cases.'],
                    ['ttgo folders', 'View folder tree, create, rename, move, and delete folders.'],
                    ['ttgo runs', 'Manage test runs — create, complete, reopen, copy, delete. Use ttgo runs results to add and update results.'],
                    ['ttgo categories', 'List, create, delete categories and assign test cases to them.'],
                    ['ttgo requirements', 'Manage requirements — CRUD, link/unlink test cases, import from Jira, bulk import.'],
                    ['ttgo analytics', 'Summary, trends, flaky tests, most-failed, duration stats, component health, run comparisons, and more.'],
                    ['ttgo ai', 'Configure AI providers, generate test cases from requirements, accept drafts, manage prompt templates.'],
                    ['ttgo defects', 'List, link, unlink defects and create Jira issues from test failures.'],
                    ['ttgo backups', 'Create, restore, delete backups and configure scheduled backup settings.'],
                    ['ttgo webhooks', 'List, create, and delete webhook subscriptions.'],
                    ['ttgo users', 'Admin user management — list, create, update, delete, restore.'],
                    ['ttgo search', 'Full-text search across tests, requirements, and runs.'],
                ]} />
            </Card>

            <SectionHeader>Output Formats</SectionHeader>
            <Card>
                <UL items={[
                    'Use --output table (default) for human-readable output.',
                    'Use --output json (-o json) for machine-readable output, ideal for scripting and piping to jq.',
                    'Use --output plain for minimal output without borders.',
                    'Override server and token per-command with --server and --token flags.',
                ]} />
            </Card>

            <SectionHeader>Example Workflows</SectionHeader>
            <Card>
                <P>Run smoke tests and report results:</P>
                <UL items={[
                    'ttgo categories list — find the smoke category ID.',
                    'ttgo runs create --name "Smoke Run" --category <id> — create a run.',
                    'ttgo runs results add <run-id> --test <test-id> --status PASS — record results.',
                    'ttgo runs complete <run-id> — mark the run as finished.',
                ]} />
                <Tip>Combine --output json with jq for powerful scripting: ttgo runs get &lt;id&gt; -o json | jq '.run_results[] | select(.status == "FAIL")'</Tip>
            </Card>

            <SectionHeader>Claude Code Skill</SectionHeader>
            <Card>
                <P>
                    TTGO ships with a built-in Claude Code skill that lets you operate the CLI using natural language.
                    When working in this repo with Claude Code, just describe what you need and it will translate your
                    request into the appropriate ttgo commands automatically.
                </P>
                <DL items={[
                    ['How it works', 'The skill (.claude/skills/SKILL.md) teaches Claude Code the full ttgo command surface. It reads JSON output, chains multi-step workflows, and reports results back in plain language.'],
                    ['Setup', 'The skill is loaded automatically when Claude Code is running in this project. Make sure the CLI is configured (ttgo config set-server and ttgo config set-token) before use.'],
                    ['Multi-step workflows', 'Ask for complex operations like "run smoke tests and report failures" or "import a Jira requirement and generate test cases for it" — the skill handles the full sequence.'],
                ]} />
                <Tip>The skill always uses --output json for reliable parsing. You can ask follow-up questions about the results and Claude Code will query for more detail.</Tip>
            </Card>

            <SectionHeader>Example Prompts</SectionHeader>
            <Card>
                <UL items={[
                    '"List all test cases in the Login folder"',
                    '"Create a new test run from the Regression category"',
                    '"Show me the flaky tests from the last 30 days"',
                    '"Import REQ-42 from Jira and generate test cases for it"',
                    '"Compare the results of the last two runs"',
                    '"Create a backup and show the current backup schedule"',
                ]} />
            </Card>
        </div>
    );
}

function SettingsSection() {
    return (
        <div>
            <PageHeader title="Settings & Integrations" desc="Configure TTGO's integrations, AI capabilities, and access controls." />
            <Card>
                <DL items={[
                    ['General', 'Application configuration, data management, seed demo data.'],
                    ['AI Configuration', 'LLM providers, API keys, models, prompt templates.'],
                    ['Jira Integration', 'Issue tracking, defect linking, requirement import.'],
                    ['Confluence', 'Import requirements from Confluence pages.'],
                    ['Tokens', 'API tokens for programmatic access and CI/CD integration.'],
                    ['Webhooks', 'Event notifications (run completed, defect created, etc.).'],
                ]} />
            </Card>
        </div>
    );
}

function ShortcutsSection() {
    return (
        <div>
            <PageHeader title="Tips & Shortcuts" desc="Productivity tips and workflow recommendations." />

            <SectionHeader>Productivity</SectionHeader>
            <Card>
                <UL items={[
                    'Use sidebar zoom buttons (A-/A+) to adjust folder tree font size.',
                    'Collapse the sidebar for more grid space.',
                    'Customize grid columns with the column selector.',
                    'Drag and drop to reorganize folders and test cases.',
                    'Bulk select in the grid for batch operations.',
                    'Toggle dark/light theme from the header.',
                ]} />
            </Card>

            <SectionHeader>AI Workflow</SectionHeader>
            <Card>
                <UL items={[
                    'Select a requirement first — it auto-populates in AI Gen and Import.',
                    'Use format override if auto-detection picks wrong format.',
                    'Review and edit all parsed test cases before accepting.',
                    'Check the traceability matrix after importing to verify coverage.',
                ]} />
            </Card>

            <SectionHeader>Integrations</SectionHeader>
            <Card>
                <UL items={[
                    'Set up Jira to create defects directly from failed test runs.',
                    'Use Confluence import for automatic requirement extraction.',
                    'Use API tokens for CI/CD — trigger runs and report results programmatically.',
                ]} />
            </Card>
        </div>
    );
}

/* ── REUSABLE PRIMITIVES ── */

function PageHeader({ title, desc }) {
    return (
        <div style={styles.pageHeader}>
            <h2 style={styles.pageTitle}>{title}</h2>
            <p style={styles.pageDesc}>{desc}</p>
            <div style={styles.pageHeaderBar} />
        </div>
    );
}

function SectionHeader({ children }) {
    return <h3 style={styles.sectionHeader}>{children}</h3>;
}

function P({ children }) {
    return <p style={styles.paragraph}>{children}</p>;
}

function Card({ children }) {
    return (
        <div className="help-card" style={styles.card}>
            {children}
        </div>
    );
}

function Tip({ children }) {
    return (
        <div style={styles.tip}>
            <div style={styles.tipIconWrap}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
                </svg>
            </div>
            <div>
                <div style={styles.tipLabel}>Tip</div>
                <div style={styles.tipText}>{children}</div>
            </div>
        </div>
    );
}

function Steps({ items }) {
    return (
        <div style={styles.steps}>
            {items.map((item, i) => (
                <div key={i} style={styles.stepRow}>
                    <div className="help-step-number" style={styles.stepNumber}>{i + 1}</div>
                    <div style={styles.stepContent}>
                        <div style={styles.stepTitle}>{item.title}</div>
                        <div style={styles.stepDesc}>{item.desc}</div>
                    </div>
                </div>
            ))}
        </div>
    );
}

function DL({ items }) {
    return (
        <div style={styles.dl}>
            {items.map(([term, desc], i) => (
                <div key={i} style={styles.dlItem}>
                    <div style={styles.dt}>{term}</div>
                    <div style={styles.dd}>{desc}</div>
                </div>
            ))}
        </div>
    );
}

function UL({ items }) {
    return (
        <div style={styles.ul}>
            {items.map((item, i) => (
                <div key={i} style={styles.ulItem}>
                    <div style={styles.ulDot} />
                    <div style={styles.ulText}>{item}</div>
                </div>
            ))}
        </div>
    );
}

/* ── STYLES ── */

const styles = {
    page: {
        display: 'flex',
        minHeight: 'calc(100vh - 56px)',
        width: '100%',
    },

    /* Sidebar */
    sidebar: {
        width: 230,
        minWidth: 230,
        display: 'flex',
        flexDirection: 'column',
        borderRight: '1px solid var(--border-color)',
        background: 'var(--bg-secondary)',
        position: 'sticky',
        top: 56,
        height: 'calc(100vh - 56px)',
        overflowY: 'auto',
    },
    sidebarTop: {
        padding: '16px 14px 12px',
        borderBottom: '1px solid var(--border-color)',
    },
    sidebarBrand: {
        display: 'flex', alignItems: 'center', gap: 9, marginBottom: 14,
    },
    brandIcon: {
        width: 28, height: 28, borderRadius: 7,
        background: 'linear-gradient(135deg, #6366f1, #4f46e5)',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        boxShadow: '0 2px 8px rgba(99,102,241,0.35)',
    },
    brandText: {
        fontSize: '0.92rem', fontWeight: 700, color: 'var(--text-primary)',
        letterSpacing: '-0.01em',
    },
    searchWrap: {
        position: 'relative', display: 'flex', alignItems: 'center',
    },
    searchIcon: {
        position: 'absolute', left: 9, pointerEvents: 'none', opacity: 0.6,
    },
    searchInput: {
        width: '100%', padding: '7px 8px 7px 30px', borderRadius: 6,
        border: '1px solid var(--border-color)', background: 'var(--bg-tertiary)',
        color: 'var(--text-primary)', fontSize: '0.8rem', outline: 'none',
        fontFamily: 'inherit', transition: 'border-color 0.15s',
    },
    searchClear: {
        position: 'absolute', right: 6, background: 'none', border: 'none',
        color: 'var(--text-secondary)', cursor: 'pointer', display: 'flex',
        padding: 3,
    },
    navList: {
        flex: 1, padding: '8px 10px', display: 'flex', flexDirection: 'column', gap: 1,
    },
    navItem: {
        display: 'flex', alignItems: 'center', gap: 9,
        width: '100%', textAlign: 'left', padding: '8px 10px',
        borderRadius: 7, border: 'none', background: 'transparent',
        color: 'var(--text-secondary)', fontSize: '0.82rem', fontWeight: 450,
        cursor: 'pointer', transition: 'all 0.12s ease', fontFamily: 'inherit',
    },
    navItemActive: {
        background: 'linear-gradient(135deg, #6366f1 0%, #4f46e5 100%)',
        color: '#fff', fontWeight: 600,
        boxShadow: '0 2px 10px rgba(99,102,241,0.35)',
    },
    navItemHover: {
        background: 'var(--bg-tertiary)',
        color: 'var(--text-primary)',
    },
    sidebarFooter: {
        padding: '12px 14px', borderTop: '1px solid var(--border-color)',
    },
    footerText: {
        fontSize: '0.7rem', color: 'var(--text-secondary)', opacity: 0.6,
    },

    /* Content */
    content: {
        flex: 1, overflowY: 'auto', height: 'calc(100vh - 56px)',
    },
    contentInner: {
        maxWidth: 780, padding: '32px 44px 80px', margin: '0 auto',
    },

    /* Hero */
    hero: {
        position: 'relative', padding: '36px 32px 32px', borderRadius: 14,
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        marginBottom: 28, overflow: 'hidden',
    },
    heroGlow: {
        position: 'absolute', top: -40, right: -40, width: 180, height: 180,
        borderRadius: '50%', background: 'radial-gradient(circle, rgba(99,102,241,0.15) 0%, transparent 70%)',
        pointerEvents: 'none',
    },
    heroContent: { position: 'relative', zIndex: 1 },
    heroBadge: {
        display: 'inline-flex', alignItems: 'center', gap: 6,
        padding: '4px 12px', borderRadius: 99, fontSize: '0.72rem', fontWeight: 600,
        background: 'rgba(99,102,241,0.12)', color: 'var(--accent-indigo, #6366f1)',
        border: '1px solid rgba(99,102,241,0.2)', marginBottom: 14,
        letterSpacing: '0.02em', textTransform: 'uppercase',
    },
    heroTitle: {
        margin: '0 0 10px', fontSize: '1.65rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.02em',
    },
    heroSubtitle: {
        margin: 0, fontSize: '0.92rem', lineHeight: 1.7,
        color: 'var(--text-secondary)', maxWidth: 520,
    },

    /* Quick links grid */
    quickGrid: {
        display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
        gap: 10, marginBottom: 32,
    },
    quickCard: {
        display: 'flex', alignItems: 'center', gap: 14,
        padding: '14px 16px', borderRadius: 10,
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
        cursor: 'pointer', textAlign: 'left', fontFamily: 'inherit',
    },
    quickCardIcon: {
        width: 38, height: 38, borderRadius: 8, flexShrink: 0,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        background: 'rgba(99,102,241,0.08)',
    },
    quickCardText: { flex: 1, minWidth: 0 },
    quickCardTitle: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 2 },
    quickCardDesc: { fontSize: '0.76rem', color: 'var(--text-secondary)', lineHeight: 1.4 },

    /* Page header */
    pageHeader: { marginBottom: 24 },
    pageTitle: {
        margin: '0 0 8px', fontSize: '1.5rem', fontWeight: 700,
        color: 'var(--text-primary)', letterSpacing: '-0.02em',
    },
    pageDesc: {
        margin: '0 0 14px', fontSize: '0.9rem', lineHeight: 1.6,
        color: 'var(--text-secondary)',
    },
    pageHeaderBar: {
        height: 3, width: 48, borderRadius: 2,
        background: 'linear-gradient(90deg, var(--accent-indigo, #6366f1), var(--accent-teal, #14b8a6))',
    },

    /* Section header */
    sectionHeader: {
        margin: '28px 0 12px', fontSize: '0.78rem', fontWeight: 700,
        color: 'var(--text-secondary)', textTransform: 'uppercase',
        letterSpacing: '0.08em',
    },

    /* Card */
    card: {
        padding: '18px 22px', borderRadius: 10, marginBottom: 16,
        background: 'var(--bg-secondary)',
        border: '1px solid var(--border-color)',
    },

    /* Paragraph */
    paragraph: {
        fontSize: '0.87rem', lineHeight: 1.7, color: 'var(--text-secondary)',
        margin: '0 0 14px',
    },

    /* Tip */
    tip: {
        display: 'flex', gap: 12, padding: '12px 16px', borderRadius: 8,
        background: 'rgba(99,102,241,0.06)',
        border: '1px solid rgba(99,102,241,0.15)',
        marginTop: 14,
    },
    tipIconWrap: {
        flexShrink: 0, marginTop: 1,
        color: 'var(--accent-indigo, #6366f1)',
    },
    tipLabel: {
        fontSize: '0.72rem', fontWeight: 700, color: 'var(--accent-indigo, #6366f1)',
        textTransform: 'uppercase', letterSpacing: '0.06em', marginBottom: 3,
    },
    tipText: {
        fontSize: '0.83rem', lineHeight: 1.6, color: 'var(--text-secondary)',
    },

    /* Steps */
    steps: {
        display: 'flex', flexDirection: 'column', gap: 0,
    },
    stepRow: {
        display: 'flex', gap: 14, padding: '10px 0',
        borderBottom: '1px solid var(--border-color)',
    },
    stepNumber: {
        width: 26, height: 26, borderRadius: 7, flexShrink: 0,
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        fontSize: '0.75rem', fontWeight: 700,
        background: 'rgba(99,102,241,0.1)', color: 'var(--accent-indigo, #6366f1)',
    },
    stepContent: { flex: 1, minWidth: 0, paddingTop: 2 },
    stepTitle: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 2 },
    stepDesc: { fontSize: '0.8rem', lineHeight: 1.5, color: 'var(--text-secondary)' },

    /* DL */
    dl: { display: 'flex', flexDirection: 'column', gap: 0 },
    dlItem: {
        padding: '10px 0',
        borderBottom: '1px solid var(--border-color)',
    },
    dt: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)', marginBottom: 3 },
    dd: { fontSize: '0.82rem', lineHeight: 1.6, color: 'var(--text-secondary)' },

    /* UL */
    ul: { display: 'flex', flexDirection: 'column', gap: 8 },
    ulItem: { display: 'flex', gap: 10, alignItems: 'flex-start' },
    ulDot: {
        width: 5, height: 5, borderRadius: '50%', flexShrink: 0, marginTop: 7,
        background: 'var(--accent-indigo, #6366f1)', opacity: 0.6,
    },
    ulText: { fontSize: '0.84rem', lineHeight: 1.6, color: 'var(--text-secondary)' },

    /* Status grid (Test Runs) */
    statusGrid: { display: 'flex', flexDirection: 'column', gap: 8 },
    statusItem: {
        display: 'flex', alignItems: 'center', gap: 12, padding: '10px 14px',
        borderRadius: 8, border: '1px solid',
    },
    statusDot: { width: 8, height: 8, borderRadius: '50%', flexShrink: 0 },
    statusName: { fontSize: '0.84rem', fontWeight: 600, marginBottom: 1 },
    statusDesc: { fontSize: '0.78rem', color: 'var(--text-secondary)' },

    /* Format grid (AI Import) */
    formatGrid: { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 },
    formatCard: {
        padding: '14px 16px', borderRadius: 8,
        background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)',
    },
    formatHeader: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 },
    formatName: { fontSize: '0.86rem', fontWeight: 600, color: 'var(--text-primary)' },
    formatTag: {
        fontSize: '0.68rem', padding: '2px 7px', borderRadius: 4,
        background: 'rgba(99,102,241,0.1)', color: 'var(--accent-indigo, #6366f1)',
        fontFamily: 'monospace',
    },
    formatDesc: { fontSize: '0.78rem', lineHeight: 1.5, color: 'var(--text-secondary)' },
};
