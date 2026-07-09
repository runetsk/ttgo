import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { auth } from '../api';
import { useAuth } from '../contexts/AuthContext';

export default function LoginPage() {
    const navigate = useNavigate();
    const { user, loading, refetchUser } = useAuth();

    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [confirm, setConfirm] = useState('');
    const [error, setError] = useState('');
    const [lockedUntil, setLockedUntil] = useState(null);
    const [submitting, setSubmitting] = useState(false);
    // null = probe in flight; render nothing until we know which form to show
    // so a fresh instance doesn't flash "Sign In" before switching to setup.
    const [needsSetup, setNeedsSetup] = useState(null);
    // Where to go after auth. AuthGate stashes where the visitor was headed
    // before bouncing them here; default to the library. Captured once, before
    // login flips `user`, so both the already-signed-in guard and the
    // post-submit navigation agree on the same destination.
    const [redirectTarget] = useState(() => sessionStorage.getItem('redirectAfterLogin') || '/library');

    useEffect(() => {
        let cancelled = false;
        auth.needsSetup()
            .then(res => { if (!cancelled) setNeedsSetup(Boolean(res?.needs_setup)); })
            .catch(() => { if (!cancelled) setNeedsSetup(false); });
        return () => { cancelled = true; };
    }, []);

    // If already logged in, redirect away. Rendered declaratively — calling
    // navigate() during render triggers React's setState-in-render error.
    if (!loading && user) {
        return <Navigate to={redirectTarget} replace />;
    }

    if (needsSetup === null) return null;

    const navigateAfterAuth = () => {
        sessionStorage.removeItem('redirectAfterLogin');
        navigate(redirectTarget, { replace: true });
    };

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setLockedUntil(null);

        if (needsSetup) {
            if (password.length < 8) {
                setError('Password must be at least 8 characters.');
                return;
            }
            if (password !== confirm) {
                setError('Passwords do not match.');
                return;
            }
            setSubmitting(true);
            try {
                await auth.setup(email, password);
                await refetchUser();
                navigateAfterAuth();
            } catch (err) {
                if (err?.response?.status === 403) {
                    // Someone else completed setup first — fall back to sign-in.
                    setNeedsSetup(false);
                    setError('Setup already completed — sign in instead.');
                } else {
                    setError(err?.response?.data?.error || 'Could not create the admin account.');
                }
            } finally {
                setSubmitting(false);
            }
            return;
        }

        setSubmitting(true);
        try {
            await auth.login(email, password);
            await refetchUser();
            navigateAfterAuth();
        } catch (err) {
            const status = err?.response?.status;
            if (status === 423) {
                const unlockTime = err?.response?.data?.locked_until;
                if (unlockTime) {
                    setLockedUntil(new Date(unlockTime).toLocaleTimeString());
                } else {
                    setError('Account is temporarily locked. Please try again later.');
                }
            } else {
                setError('Invalid email or password.');
            }
        } finally {
            setSubmitting(false);
        }
    };

    const labelStyle = { display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' };

    return (
        <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: '100vh',
            background: 'var(--bg-primary, #0f1117)',
        }}>
            <div className="glass-panel" style={{ width: 380, padding: 40 }}>
                <h2 style={{ marginTop: 0, marginBottom: 8, textAlign: 'center' }}>
                    {needsSetup ? 'Create admin account' : 'Sign In'}
                </h2>
                <p style={{ textAlign: 'center', color: 'var(--text-secondary)', marginBottom: 32, fontSize: '0.9rem' }}>
                    {needsSetup
                        ? 'First run — create the administrator account for this TestTracker instance.'
                        : 'TestTracker'}
                </p>

                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 16 }}>
                        <label style={labelStyle}>Email</label>
                        <input
                            className="modern-input"
                            type="email"
                            value={email}
                            onChange={e => setEmail(e.target.value)}
                            placeholder="admin@example.com"
                            required
                            autoFocus
                            style={{ width: '100%' }}
                        />
                    </div>

                    <div style={{ marginBottom: needsSetup ? 16 : 24 }}>
                        <label style={labelStyle}>Password</label>
                        <input
                            className="modern-input"
                            type="password"
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            placeholder="••••••••"
                            required
                            style={{ width: '100%' }}
                        />
                    </div>

                    {needsSetup && (
                        <div style={{ marginBottom: 24 }}>
                            <label style={labelStyle}>Confirm password</label>
                            <input
                                className="modern-input"
                                type="password"
                                value={confirm}
                                onChange={e => setConfirm(e.target.value)}
                                placeholder="••••••••"
                                required
                                style={{ width: '100%' }}
                            />
                        </div>
                    )}

                    {error && (
                        <div style={{
                            marginBottom: 16,
                            padding: '10px 14px',
                            borderRadius: 6,
                            background: 'rgba(239,68,68,0.1)',
                            color: 'var(--accent-red, #ef4444)',
                            fontSize: '0.875rem',
                        }}>
                            {error}
                        </div>
                    )}

                    {lockedUntil && (
                        <div style={{
                            marginBottom: 16,
                            padding: '10px 14px',
                            borderRadius: 6,
                            background: 'rgba(245,158,11,0.1)',
                            color: 'var(--accent-amber, #f59e0b)',
                            fontSize: '0.875rem',
                        }}>
                            Account locked until {lockedUntil}. Too many failed attempts.
                        </div>
                    )}

                    <button
                        type="submit"
                        className="primary-btn"
                        disabled={submitting}
                        style={{ width: '100%', padding: '10px 0' }}
                    >
                        {needsSetup
                            ? (submitting ? 'Creating…' : 'Create account')
                            : (submitting ? 'Signing in…' : 'Sign In')}
                    </button>
                </form>
            </div>
        </div>
    );
}
