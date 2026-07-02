/* eslint-disable react-refresh/only-export-components -- context/hook file intentionally co-exports its Provider and hook; splitting would ripple imports across the app with no runtime benefit */
import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import api from '../api';

export const AuthContext = createContext(null);

/**
 * AuthProvider wraps the application and provides the current user and helpers.
 * On mount it calls GET /api/auth/me with _silent:true to restore session state.
 * Also listens for the 'auth:require-login' custom event (fired by the api.js
 * interceptor on 401) to open the login modal in-place instead of redirecting.
 */
export function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);
    const [loginModalOpen, setLoginModalOpen] = useState(false);

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

    const openLoginModal = useCallback(() => setLoginModalOpen(true), []);
    const closeLoginModal = useCallback(() => setLoginModalOpen(false), []);

    useEffect(() => {
        refetchUser();
    }, [refetchUser]);

    useEffect(() => {
        window.addEventListener('auth:require-login', openLoginModal);
        return () => window.removeEventListener('auth:require-login', openLoginModal);
    }, [openLoginModal]);

    return (
        <AuthContext.Provider value={{ user, loading, refetchUser, loginModalOpen, openLoginModal, closeLoginModal }}>
            {children}
        </AuthContext.Provider>
    );
}

/** Convenience hook — must be used inside <AuthProvider>. */
export function useAuth() {
    return useContext(AuthContext);
}
