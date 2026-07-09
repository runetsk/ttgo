// Auth routing helpers.
//
// The app has a single sign-in surface: the /login page. LoginPage decides what
// to show there — the normal sign-in form, or the first-run "Create admin
// account" form when the instance has no users yet (GET /api/auth/needs-setup).
// There is no in-place sign-in modal; anyone who needs to authenticate is sent
// to /login, and returned to where they were afterwards (see AuthGate).

/**
 * Decide whether an unauthenticated visitor should be redirected to /login.
 *
 * Returns true only once auth state is resolved (loading === false), there is
 * no active session (user is falsy), and we are not already on /login.
 *
 * @param {{loading: boolean, user: unknown, pathname: string}} state
 * @returns {boolean}
 */
export function shouldRedirectToLogin({ loading, user, pathname }) {
    return loading === false && !user && pathname !== '/login';
}
