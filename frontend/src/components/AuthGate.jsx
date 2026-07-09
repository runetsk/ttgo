import { useEffect } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { shouldRedirectToLogin } from '../utils/authFlow';

/**
 * AuthGate — the app's single redirect to the sign-in surface. When auth state
 * resolves to "no session", it sends the visitor to /login (which shows sign-in,
 * or the first-run "Create admin account" form on a brand-new instance) instead
 * of popping an in-place modal. The current location is stashed so LoginPage can
 * return the user there after they sign in. Renders nothing.
 */
export default function AuthGate() {
    const { user, loading } = useAuth();
    const navigate = useNavigate();
    const location = useLocation();

    useEffect(() => {
        if (shouldRedirectToLogin({ loading, user, pathname: location.pathname })) {
            // Remember where we were so LoginPage can send us back after sign-in.
            sessionStorage.setItem('redirectAfterLogin', location.pathname + location.search);
            navigate('/login', { replace: true });
        }
    }, [loading, user, location.pathname, location.search, navigate]);

    return null;
}
