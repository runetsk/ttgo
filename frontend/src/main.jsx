import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import ErrorBoundary from './components/shared/ErrorBoundary'
import './index.css'
import App from './App.jsx'

// A data router (for useBlocker) with one catch-all route: App keeps declaring every page in its
// own <Routes>, which React Router supports as descendant routes under a splat.
const router = createBrowserRouter([
  { path: '*', element: <ErrorBoundary><App /></ErrorBoundary> },
])

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <RouterProvider router={router} />
  </StrictMode>,
)
