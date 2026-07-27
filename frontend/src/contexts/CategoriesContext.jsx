/* eslint-disable react-refresh/only-export-components -- context/hook file intentionally co-exports its Provider and hook; splitting would ripple imports across the app with no runtime benefit */
import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { getAllCategories } from '../api';
import { useAuth } from './AuthContext';

export const CategoriesContext = createContext(null);

const toList = (data) => (Array.isArray(data?.categories) ? data.categories : []);

/**
 * CategoriesProvider — one shared copy of the full category list.
 *
 * The list is small, changes rarely, and feeds every category picker and filter
 * in the app (test grid, run list, run results, test-case detail). Each of those
 * used to fetch it independently, so a single page load fired several identical
 * /categories requests; they also used the endpoint's default page size, which
 * silently hid every category past the tenth.
 *
 * The paginated /categories management screen still calls getCategories directly
 * — it needs page/search parameters this cache can't serve — but it calls
 * refresh() after create/delete so the pickers pick the change up.
 */
export function CategoriesProvider({ children }) {
    const { user } = useAuth();
    const [categories, setCategories] = useState([]);

    // Both paths are gated on `user`: the sign-in page mounts this provider too,
    // and an unauthenticated request would 401 and trip the api.js
    // require-login handler.
    useEffect(() => {
        if (!user) return;
        let cancelled = false;
        getAllCategories()
            .then(data => { if (!cancelled) setCategories(toList(data)); })
            .catch(() => { if (!cancelled) setCategories([]); });
        return () => { cancelled = true; };
    }, [user]);

    /** Re-read the list after something mutates categories. */
    const refresh = useCallback(async () => {
        if (!user) return;
        try {
            setCategories(toList(await getAllCategories()));
        } catch {
            setCategories([]);
        }
    }, [user]);

    return (
        <CategoriesContext.Provider value={{ categories, refresh }}>
            {children}
        </CategoriesContext.Provider>
    );
}

/** Convenience hook — must be used inside <CategoriesProvider>. */
export function useCategories() {
    return useContext(CategoriesContext);
}
