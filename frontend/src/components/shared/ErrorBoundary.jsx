import { Component } from 'react';

export default class ErrorBoundary extends Component {
    constructor(props) {
        super(props);
        this.state = { hasError: false, error: null };
    }

    static getDerivedStateFromError(error) {
        return { hasError: true, error };
    }

    componentDidCatch(error, errorInfo) {
        console.error('[ErrorBoundary]', error, errorInfo);
    }

    handleReset = () => {
        this.setState({ hasError: false, error: null });
    };

    render() {
        if (this.state.hasError) {
            return (
                <div style={{
                    display: 'flex', flexDirection: 'column', alignItems: 'center',
                    justifyContent: 'center', height: '100vh', gap: 16,
                    color: 'var(--text-primary)', fontFamily: 'inherit',
                }}>
                    <div style={{
                        fontSize: '2rem', width: 56, height: 56, borderRadius: 16,
                        background: 'rgba(239,68,68,0.12)', border: '1px solid rgba(239,68,68,0.3)',
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                        ⚠
                    </div>
                    <h2 style={{ margin: 0, fontSize: '1.1rem' }}>Something went wrong</h2>
                    <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', margin: 0, maxWidth: 400, textAlign: 'center' }}>
                        An unexpected error occurred. Try refreshing or click the button below.
                    </p>
                    <button
                        onClick={this.handleReset}
                        style={{
                            padding: '8px 20px', borderRadius: 8,
                            background: 'var(--accent-indigo)', color: '#fff',
                            border: 'none', cursor: 'pointer', fontSize: '0.85rem',
                        }}
                    >
                        Try Again
                    </button>
                </div>
            );
        }
        return this.props.children;
    }
}
