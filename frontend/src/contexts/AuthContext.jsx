/* eslint-disable react-refresh/only-export-components -- context/hook file intentionally co-exports its Provider and hook; splitting would ripple imports across the app with no runtime benefit */
import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import api from '../api';

export const AuthContext = createContext(null);

/**
 * AuthProvider wraps the application and provides the current user and helpers.
 * On mount it calls GET /api/auth/me with _silent:true to restore session state.
 * It also listens for the 'auth:require-login' custom event (fired by the api.js
 * interceptor on a 401) and clears the user, so AuthGate redirects to the
 * /login page instead of the app popping an in-place sign-in modal.
 */
export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);

    const refetchUser = useCallback(async () => {
        try {
            const res = await api.get('/auth/me', { _silent: true });
            setUser(res.data?.user ?? null);
        } catch {
            setUser(null);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        refetchUser();
    }, [refetchUser]);

    // A 401 on any protected call means the session is gone. Clear the user;
    // AuthGate then redirects to /login (sign-in, or first-run setup).
    useEffect(() => {
        const onRequireLogin = () => setUser(null);
        window.addEventListener('auth:require-login', onRequireLogin);
        return () => window.removeEventListener('auth:require-login', onRequireLogin);
    }, []);

    return (
        <AuthContext.Provider value={{ user, loading, refetchUser }}>
            {children}
        </AuthContext.Provider>
    );
}

/** Convenience hook — must be used inside <AuthProvider>. */
export function useAuth() {
    return useContext(AuthContext);
}
