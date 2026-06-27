import { useState, useEffect, useRef } from 'react';
import { auth } from '../api';
import { useAuth } from '../contexts/AuthContext';

/**
 * LoginModal — shown in-place (without navigation) when a 401 is received.
 * On success it refreshes the auth state and closes; the user stays on the
 * same page they were on.
 */
export default function LoginModal() {
    const { loginModalOpen, closeLoginModal, refetchUser } = useAuth();

    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [lockedUntil, setLockedUntil] = useState(null);
    const [submitting, setSubmitting] = useState(false);

    const emailRef = useRef(null);

    // Reset form whenever modal opens and focus the email field
    useEffect(() => {
        if (loginModalOpen) {
            setEmail('');
            setPassword('');
            setError('');
            setLockedUntil(null);
            setTimeout(() => emailRef.current?.focus(), 50);
        }
    }, [loginModalOpen]);

    // Close on Escape key
    useEffect(() => {
        if (!loginModalOpen) return;
        const onKey = (e) => { if (e.key === 'Escape') closeLoginModal(); };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [loginModalOpen, closeLoginModal]);

    if (!loginModalOpen) return null;

    const handleSubmit = async (e) => {
        e.preventDefault();
        setError('');
        setLockedUntil(null);
        setSubmitting(true);
        try {
            await auth.login(email, password);
            await refetchUser();
            closeLoginModal();
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
        <div
            onClick={closeLoginModal}
            style={{
                position: 'fixed',
                inset: 0,
                zIndex: 9999,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                background: 'rgba(0,0,0,0.6)',
                backdropFilter: 'blur(4px)',
            }}
        >
            <div
                onClick={(e) => e.stopPropagation()}
                className="glass-panel"
                style={{ width: 380, padding: 40, position: 'relative' }}
            >
                {/* Close button */}
                <button
                    onClick={closeLoginModal}
                    style={{
                        position: 'absolute',
                        top: 16,
                        right: 16,
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        color: 'var(--text-secondary)',
                        fontSize: '1.2rem',
                        lineHeight: 1,
                        padding: 4,
                    }}
                    aria-label="Close"
                >
                    ✕
                </button>

                <h2 style={{ marginTop: 0, marginBottom: 8, textAlign: 'center' }}>Sign In</h2>
                <p style={{ textAlign: 'center', color: 'var(--text-secondary)', marginBottom: 32, fontSize: '0.9rem' }}>
                    Your session has expired. Please sign in to continue.
                </p>

                <form onSubmit={handleSubmit}>
                    <div style={{ marginBottom: 16 }}>
                        <label style={{ display: 'block', marginBottom: 6, fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-primary)' }}>
                            Email
                        </label>
                        <input
                            ref={emailRef}
                            className="modern-input"
                            type="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="admin@example.com"
                            required
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
                            onChange={(e) => setPassword(e.target.value)}
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
