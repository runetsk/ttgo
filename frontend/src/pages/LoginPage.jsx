import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { auth } from '../api';
import { useAuth } from '../contexts/AuthContext';

export default function LoginPage() {
    const navigate = useNavigate();
    const { user, loading, refetchUser } = useAuth();

    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [lockedUntil, setLockedUntil] = useState(null);
    const [submitting, setSubmitting] = useState(false);

    // If already logged in, redirect away
    if (!loading && user) {
        navigate('/library', { replace: true });
        return null;
    }

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setLockedUntil(null);
        setSubmitting(true);
        try {
            await auth.login(email, password);
            await refetchUser();
            const redirect = sessionStorage.getItem('redirectAfterLogin') || '/library';
            sessionStorage.removeItem('redirectAfterLogin');
            navigate(redirect, { replace: true });
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

    return (
        <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: '100vh',
            background: 'var(--bg-primary, #0f1117)',
        }}>
            <div className="glass-panel" style={{ width: 380, padding: 40 }}>
                <h2 style={{ marginTop: 0, marginBottom: 8, textAlign: 'center' }}>Sign In</h2>
                <p style={{ textAlign: 'center', color: 'var(--text-secondary)', marginBottom: 32, fontSize: '0.9rem' }}>
                    TestTracker
                </p>

                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Email
                        </label>
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

                    <div style={{ marginBottom: 24 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Password
                        </label>
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
                        {submitting ? 'Signing in…' : 'Sign In'}
                    </button>
                </form>
            </div>
        </div>
    );
}
