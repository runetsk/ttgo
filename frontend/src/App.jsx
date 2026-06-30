import { useState, useEffect, useRef, useCallback, lazy, Suspense } from 'react'
import { Routes, Route, Navigate, useNavigate, useLocation, useParams } from 'react-router-dom'
import { getFolder } from './api'
import { useAbortController } from './hooks/useAbortController'
import { ToastProvider } from './components/Toast'
import { AuthProvider } from './contexts/AuthContext'
import { AIGenerationProvider, useAIGeneration } from './contexts/AIGenerationContext'
import AIDisabledNotice from './components/AIDisabledNotice'
import { WebSocketProvider } from './hooks/useWebSocket'
import ConnectionStatus from './components/ConnectionStatus'
import Sidebar from './components/Sidebar'
import LoginModal from './components/LoginModal'
import RunFolderSidebar from './components/RunFolderSidebar'
import AppNav from './components/AppNav'
import './App.css'

// Route-level code splitting — each page loads on demand
const TestGrid = lazy(() => import('./components/TestGrid'))
const CategoryManager = lazy(() => import('./components/CategoryManager'))
const TestCaseDetail = lazy(() => import('./components/TestCaseDetail'))
const TestRunList = lazy(() => import('./pages/TestRunList'))
const TestRunDetail = lazy(() => import('./pages/TestRunDetail'))
const SettingsPage = lazy(() => import('./pages/SettingsPage'))
const LoginPage = lazy(() => import('./pages/LoginPage'))
const AnalyticsDashboard = lazy(() => import('./pages/AnalyticsDashboard'))
const TraceabilityMatrix = lazy(() => import('./pages/TraceabilityMatrix'))
const RequirementsPage = lazy(() => import('./pages/RequirementsPage'))
const RequirementDetailPage = lazy(() => import('./pages/RequirementDetailPage'))
const AIGeneratePage = lazy(() => import('./pages/AIGeneratePage'))
const HelpPage = lazy(() => import('./pages/HelpPage'))
const DefectsPage = lazy(() => import('./pages/DefectsPage'))

function PageLoader() {
    return (
        <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            height: '60vh', color: 'var(--text-secondary)', fontSize: '0.85rem',
        }}>
            Loading…
        </div>
    );
}

function App() {
  const [selectedFolders, setSelectedFolders] = useState([])
  const navigate = useNavigate();
  const location = useLocation();
  const actionSource = useRef(null);
  // Theme state with localStorage persistence
  const [theme, setTheme] = useState(localStorage.getItem('theme') || 'dark');

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }, [theme]);

  // Sync selection to URL
  useEffect(() => {
    const source = actionSource.current;
    actionSource.current = null;

    if (source === 'url') {
      return;
    }

    // If the user explicitly clicked a folder in the sidebar, 
    // they definitely want to navigate to it, regardless of the current path.
    if (source === 'user') {
      if (selectedFolders.length === 1) {
        const id = selectedFolders[0].id;
        navigate(`/library/folders/${id}`);
        return;
      } else {
        // 0 or >1 (Bulk mode or Home)
        navigate('/library', { state: { preserveSelection: true } });
        return;
      }
    }

    // Fallback/auto-sync logic (for non-user actions like initial load)
    // On initial load (source is null), if we are deep linking, don't force redirect
    if (!source) {
      if (location.pathname.startsWith('/library/folders/')) return;
      if (location.pathname.startsWith('/library/tests/')) return;
    }

    if (location.pathname.startsWith('/library/tests/')) return;
    // When a folder is selected and a test pane is open, don't auto-nav away
    if (/^\/library\/folders\/[^/]+\/tests\//.test(location.pathname)) return;
    if (location.pathname.startsWith('/runs')) return;
    if (location.pathname === '/settings') return;
    if (location.pathname === '/analytics') return;
    if (location.pathname === '/categories') return;
    if (location.pathname === '/login') return;
    if (location.pathname === '/traceability') return;
    if (location.pathname === '/requirements') return;
    if (location.pathname.startsWith('/requirements/')) return;
    if (location.pathname === '/ai-generate') return;
    if (location.pathname === '/help') return;
    if (location.pathname === '/defects') return;

    if (selectedFolders.length === 1) {
      const id = selectedFolders[0].id;
      if (location.pathname !== `/library/folders/${id}`) {
        navigate(`/library/folders/${id}`);
      }
    } else {
      if (location.pathname !== '/library') {
        navigate('/library');
      }
    }
  }, [selectedFolders, navigate, location.pathname]);

  const toggleTheme = () => {
    setTheme(prev => prev === 'dark' ? 'light' : 'dark');
  };

  const handleFolderLoad = useCallback((folders) => {
    actionSource.current = 'url';
    setSelectedFolders(folders);
  }, []);

  const handleHomeLoad = useCallback(() => {
    // If we navigated here with 'preserveSelection', don't clear.
    if (location.state?.preserveSelection) {
      // Clear the state so future reloads don't preserve it? 
      // Actually navigate replace might be needed, but for now just don't clear.
      return;
    }
    actionSource.current = 'url';
    setSelectedFolders(prev => prev.length > 0 ? [] : prev);
  }, [location.state]);

  const handleUserSelect = useCallback((val) => {
    actionSource.current = 'user';
    setSelectedFolders(val);
  }, []);

  const isLoginPage = location.pathname === '/login';

  const showSidebar = !isLoginPage && (
    location.pathname === '/library' ||
    location.pathname.startsWith('/library/folders/') ||
    location.pathname.startsWith('/library/tests/')
  );

  const isThreePane = /^\/library\/folders\/[^/]+\/tests\/[^/]+/.test(location.pathname);

  const showRunSidebar = !isLoginPage && location.pathname.startsWith('/runs');

  // Runs sidebar state — lifted here so RunFolderSidebar lives at app-container level
  const runFolderMatch = location.pathname.match(/^\/runs\/folders\/([^/]+)/);
  const runSelectedFolderId = runFolderMatch ? runFolderMatch[1] : null;
  const runDetailMatch = location.pathname.match(/^\/runs\/run\/([^/]+)/);
  const runSelectedRunId = runDetailMatch ? runDetailMatch[1] : null;
  const handleRunSelectFolder = useCallback((id) => {
    if (id === null) navigate('/runs');
    else if (id === 'uncategorised') navigate('/runs/folders/uncategorised');
    else navigate(`/runs/folders/${id}`);
  }, [navigate]);
  const [runRuns, setRunRuns] = useState([]);
  const [runListKey, setRunListKey] = useState(0);
  const handleRunDropped = useCallback(() => setRunListKey(k => k + 1), []);

  return (
    <ToastProvider>
    <div className="app-shell">
      <AppNav theme={theme} toggleTheme={toggleTheme} />

      <div className="app-container">
        {showSidebar && (
          <Sidebar
            onSelectFolders={handleUserSelect}
            selectedFolderIds={selectedFolders.map(f => f.id)}
          />
        )}
        {showRunSidebar && (
          <RunFolderSidebar
            selectedFolderId={runSelectedFolderId}
            selectedRunId={runSelectedRunId}
            onSelectFolder={handleRunSelectFolder}
            runs={runRuns}
            onRunDropped={handleRunDropped}
          />
        )}

        <main className="main-content">
          <div className={`content-area${isThreePane ? ' content-area-flush' : ''}`}>
            <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="/login" element={<LoginPage />} />
              <Route path="/" element={<Navigate to="/library" replace />} />
              <Route path="/library" element={<HomeViewWrapper onLoad={handleHomeLoad} selectedFolders={selectedFolders} />} />
              <Route path="/library/folders/:folderId" element={<FolderViewWrapper onFolderLoad={handleFolderLoad} />} />
              <Route path="/library/folders/:folderId/tests/:testId" element={<FolderWithTestPaneWrapper onFolderLoad={handleFolderLoad} />} />
              <Route path="/categories" element={<CategoryManagerPage />} />
              <Route path="/runs" element={<TestRunListPage selectedFolderId={null} onRunsLoaded={setRunRuns} runListKey={runListKey} />} />
              <Route path="/runs/folders/:runFolderId" element={<RunFolderViewWrapper onRunsLoaded={setRunRuns} runListKey={runListKey} />} />
              <Route path="/runs/run/:runId" element={<TestRunDetailPage />} />
              <Route path="/analytics" element={<AnalyticsDashboard />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/library/tests/:testId" element={<TestCaseDetailWrapper onFolderLoad={handleFolderLoad} />} />
              <Route path="/traceability" element={<TraceabilityMatrix />} />
              <Route path="/requirements" element={<RequirementsPage />} />
              <Route path="/requirements/:reqId" element={<RequirementDetailPage />} />
              <Route path="/ai-generate" element={<AIGenerateRoute />} />
              <Route path="/help" element={<HelpPage />} />
              <Route path="/defects" element={<DefectsPage />} />
            </Routes>
            </Suspense>
          </div>
        </main>
      </div>
    </div>
    </ToastProvider>
  )
}

function CategoryManagerPage() {
  // CategoryManager expects onUpdate, primarily to notify others. 
  // On a dedicated page, it might just be for self-refresh if implemented that way, 
  // or we can pass a no-op if it handles its own internal list refresh.
  // If CategoryManager relies on key-remount to refresh list, we can use state here.
  const [refreshKey, setRefreshKey] = useState(0);
  return (
    <div style={{ padding: 24, width: '100%' }}>
      <h2 style={{ marginBottom: 24 }}>Category Management</h2>
      <CategoryManager key={refreshKey} onUpdate={() => setRefreshKey(p => p + 1)} />
    </div>
  );
}

function TestRunListPage({ selectedFolderId, onRunsLoaded, runListKey }) {
  // display:contents removes the wrapper from the layout tree so flex sizing
  // from .content-area propagates to .test-grid-container inside TestRunList.
  return (
    <div data-testid="test-run-list-page" style={{ display: 'contents' }}>
      <TestRunList
        key={runListKey}
        selectedFolderId={selectedFolderId}
        onRunsLoaded={onRunsLoaded}
      />
    </div>
  );
}

function TestRunDetailPage() {
  return <TestRunDetail />;
}

function AIGenerateRoute() {
  const { aiFeaturesEnabled } = useAIGeneration();
  if (!aiFeaturesEnabled) return <AIDisabledNotice />;
  return <AIGeneratePage />;
}

function HomeViewWrapper({ onLoad, selectedFolders }) {
  useEffect(() => {
    onLoad();
  }, []);

  return (
    <TestGrid
      selectedFolders={selectedFolders}
    />
  );
}

function FolderViewWrapper({ onFolderLoad }) {
  const { folderId } = useParams();
  const navigate = useNavigate();
  const [folder, setFolder] = useState(null);
  const [folderError, setFolderError] = useState(false);
  const getSignal = useAbortController();

  useEffect(() => {
    if (!folderId) {
      setFolder(null);
      onFolderLoad([]);
      return;
    }
    const signal = getSignal();
    setFolderError(false);
    setFolder(null);
    getFolder(folderId, { _silent: true, signal }).then(f => {
      if (signal.aborted) return;
      setFolder(f);
      onFolderLoad([f]);
    }).catch(err => {
      if (signal.aborted) return;
      console.error(err);
      setFolderError(true);
      onFolderLoad([]);
    });
  }, [folderId, onFolderLoad, getSignal]);

  if (folderError) {
    return (
      <div style={{ flex: 1, marginTop: 24, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', gap: 16 }}>
        <span style={{ color: 'var(--text-secondary)', fontSize: '0.95rem' }}>This folder no longer exists.</span>
        <button className="action-btn" onClick={() => navigate('/library')}>← Back to Tests</button>
      </div>
    );
  }

  if (!folderId) {
    return null;
  }

  // Render TestGrid immediately with an id-only placeholder so getTests fires in
  // parallel with getFolder. Once folder metadata resolves, selectedFolders updates
  // with the same id, so TestGrid's testsLoadingRef dedup skips the refetch.
  const placeholderFolder = folder || { id: folderId, name: '' };
  return (
    <TestGrid
      key={folderId}
      selectedFolders={[placeholderFolder]}
    />
  );
}

function FolderWithTestPaneWrapper({ onFolderLoad }) {
  const { folderId, testId } = useParams();
  const navigate = useNavigate();
  const [folder, setFolder] = useState(null);
  const [folderError, setFolderError] = useState(false);
  const getSignal = useAbortController();

  const [paneWidth, setPaneWidth] = useState(() => {
    const saved = parseInt(localStorage.getItem('detailPaneWidth'), 10);
    return Number.isFinite(saved) ? saved : 480;
  });
  const isResizing = useRef(false);

  const startPaneResize = useCallback((e) => {
    e.preventDefault();
    isResizing.current = true;
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }, []);

  useEffect(() => {
    const resize = (e) => {
      if (!isResizing.current) return;
      const next = Math.max(320, Math.min(900, window.innerWidth - e.clientX));
      setPaneWidth(next);
    };
    const stop = () => {
      if (!isResizing.current) return;
      isResizing.current = false;
      document.body.style.cursor = 'default';
      document.body.style.userSelect = 'auto';
      setPaneWidth(w => {
        localStorage.setItem('detailPaneWidth', String(w));
        return w;
      });
    };
    window.addEventListener('mousemove', resize);
    window.addEventListener('mouseup', stop);
    return () => {
      window.removeEventListener('mousemove', resize);
      window.removeEventListener('mouseup', stop);
    };
  }, []);

  useEffect(() => {
    if (!folderId) return;
    const signal = getSignal();
    setFolderError(false);
    setFolder(null);
    getFolder(folderId, { _silent: true, signal }).then(f => {
      if (signal.aborted) return;
      setFolder(f);
      onFolderLoad([f]);
    }).catch(err => {
      if (signal.aborted) return;
      console.error(err);
      setFolderError(true);
      onFolderLoad([]);
    });
  }, [folderId, onFolderLoad, getSignal]);

  const handlePaneClose = useCallback(() => {
    navigate(`/library/folders/${folderId}`);
  }, [folderId, navigate]);

  if (folderError) {
    return (
      <div style={{ flex: 1, marginTop: 24, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', gap: 16 }}>
        <span style={{ color: 'var(--text-secondary)', fontSize: '0.95rem' }}>This folder no longer exists.</span>
        <button className="action-btn" onClick={() => navigate('/library')}>← Back to Tests</button>
      </div>
    );
  }

  const placeholderFolder = folder || { id: folderId, name: '' };
  return (
    <div className="three-pane-layout">
      <div className="three-pane-grid">
        <TestGrid
          key={folderId}
          selectedFolders={[placeholderFolder]}
          selectedTestId={testId}
        />
      </div>
      <TestCaseDetail
        inlinePane={true}
        onClose={handlePaneClose}
        paneWidth={paneWidth}
        onPaneResizeStart={startPaneResize}
      />
    </div>
  );
}

function TestCaseDetailWrapper({ onFolderLoad }) {
  const navigate = useNavigate();
  const [folderExists, setFolderExists] = useState(null); // null=unknown, true=ok, false=missing

  const handleTestLoad = useCallback((test) => {
    if (test && test.folder_id) {
      getFolder(test.folder_id).then(f => {
        setFolderExists(true);
        onFolderLoad([f]);
      }).catch(err => {
        console.error("Failed to load folder for sidebar sync", err);
        setFolderExists(false);
        onFolderLoad([]);
      });
    } else {
      setFolderExists(false);
      onFolderLoad([]);
    }
  }, [onFolderLoad]);

  const handleClose = (folderId) => {
    if (folderId && folderExists === true) {
      navigate(`/library/folders/${folderId}`);
    } else {
      navigate('/library');
    }
  };

  return <TestCaseDetail onTestLoad={handleTestLoad} onClose={handleClose} folderMissing={folderExists === false} />
}

function RunFolderViewWrapper({ onRunsLoaded, runListKey }) {
  const { runFolderId } = useParams();
  const folderId = runFolderId === 'uncategorised' ? 'uncategorised' : runFolderId;
  return (
    <div data-testid="test-run-list-page">
      <TestRunList key={`${folderId}-${runListKey}`} selectedFolderId={folderId} onRunsLoaded={onRunsLoaded} />
    </div>
  );
}

// Wrap App in AuthProvider so useAuth() is available everywhere.
function AppWithProviders() {
  return (
    <AuthProvider>
      <WebSocketProvider>
        <AIGenerationProvider>
          <App />
          <LoginModal />
          <ConnectionStatus />
        </AIGenerationProvider>
      </WebSocketProvider>
    </AuthProvider>
  );
}

export default AppWithProviders
