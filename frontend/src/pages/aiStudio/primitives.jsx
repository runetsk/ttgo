import { AIC, MONO, Icon } from './constants';
import { decodeEntities } from '../../utils/decodeEntities';

// ── Primitive components ─────────────────────────────────────────────────────
export function SectionLabel({ children, right }) {
    return (
        <div style={{
            display: 'flex', justifyContent: 'space-between', alignItems: 'center',
            fontSize: 10.5, textTransform: 'uppercase', letterSpacing: '0.1em',
            color: AIC.muted, fontWeight: 700, marginBottom: 10,
        }}>
            <span>{children}</span>
            {right}
        </div>
    );
}

export function Pill({ tone = 'neutral', children, style }) {
    const tones = {
        indigo: { bg: 'rgba(99,102,241,0.14)', bd: 'rgba(99,102,241,0.3)', fg: AIC.indigoSoft },
        teal: { bg: 'rgba(20,184,166,0.14)', bd: 'rgba(20,184,166,0.3)', fg: '#5eead4' },
        green: { bg: 'rgba(34,197,94,0.12)', bd: 'rgba(34,197,94,0.3)', fg: '#86efac' },
        red: { bg: 'rgba(239,68,68,0.12)', bd: 'rgba(239,68,68,0.3)', fg: '#fca5a5' },
        amber: { bg: 'rgba(234,179,8,0.12)', bd: 'rgba(234,179,8,0.3)', fg: '#fde047' },
        neutral: { bg: 'var(--aig-surface-tint-strong)', bd: AIC.border, fg: AIC.dim },
    };
    const t = tones[tone] || tones.neutral;
    return (
        <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 4,
            padding: '2px 8px', borderRadius: 999,
            fontSize: 10.5, fontWeight: 600, letterSpacing: '0.02em',
            background: t.bg, border: `1px solid ${t.bd}`, color: t.fg,
            ...(style || {}),
        }}>{children}</span>
    );
}

export function StatusPill({ status }) {
    if (status === 'accepted') return <Pill tone="green">{Icon.check(10)} Accepted</Pill>;
    if (status === 'rejected') return <Pill tone="red">{Icon.x(10)} Rejected</Pill>;
    if (status === 'superseded') return <Pill tone="neutral">Superseded</Pill>;
    return <Pill tone="indigo">{Icon.sparkles(10)} AI Draft</Pill>;
}

export function ReqChip({ id }) {
    if (!id) return null;
    return (
        <span style={{
            display: 'inline-flex', alignItems: 'center', gap: 4,
            fontFamily: MONO,
            fontSize: 11, padding: '2px 6px', borderRadius: 4,
            background: 'rgba(99,102,241,0.1)', border: '1px solid rgba(99,102,241,0.25)',
            color: AIC.indigoSoft, fontWeight: 500,
        }}>{id}</span>
    );
}

export function Segmented({ options, value, onChange }) {
    return (
        <div style={{
            display: 'inline-flex', background: AIC.bg1, border: `1px solid ${AIC.border2}`,
            padding: 3, borderRadius: 7, gap: 2,
        }}>
            {options.map(o => (
                <button key={o.value}
                    onClick={() => onChange(o.value)}
                    style={{
                        padding: '5px 11px', fontSize: 12, fontFamily: 'inherit', cursor: 'pointer',
                        border: 'none', borderRadius: 5,
                        background: value === o.value ? AIC.bg3 : 'transparent',
                        color: value === o.value ? AIC.text : AIC.dim,
                        fontWeight: value === o.value ? 600 : 400,
                    }}>{o.label}</button>
            ))}
        </div>
    );
}

export function AIBtn({ variant = 'default', onClick, disabled, children, style, title }) {
    const base = {
        padding: '7px 13px', fontSize: 12.5, borderRadius: 7, cursor: disabled ? 'not-allowed' : 'pointer',
        fontFamily: 'inherit', display: 'inline-flex', alignItems: 'center', gap: 6,
        transition: 'all .15s',
        opacity: disabled ? 0.5 : 1,
    };
    const variants = {
        default: { background: AIC.bg3, border: `1px solid ${AIC.border}`, color: AIC.text },
        primary: {
            background: 'linear-gradient(135deg, #6366f1, #4f46e5)', border: 'none', color: '#fff',
            fontWeight: 600, boxShadow: '0 4px 14px rgba(99,102,241,0.35)',
        },
        ghost: { background: 'transparent', border: `1px solid transparent`, color: AIC.text },
        success: { background: 'var(--aig-success-bg)', border: '1px solid var(--aig-success-border)', color: 'var(--aig-success-fg)' },
        danger: { background: 'var(--aig-danger-bg)', border: '1px solid var(--aig-danger-border)', color: 'var(--aig-danger-fg)' },
    };
    return (
        <button onClick={onClick} disabled={disabled} title={title}
            style={{ ...base, ...variants[variant], ...(style || {}) }}>
            {children}
        </button>
    );
}

export function FilterTab({ active, onClick, children }) {
    return (
        <button onClick={onClick} style={{
            padding: '4px 10px', fontSize: 12, borderRadius: 6, border: 'none', cursor: 'pointer',
            background: active ? AIC.bg3 : 'transparent', color: active ? AIC.text : AIC.dim,
            fontWeight: active ? 600 : 400, fontFamily: 'inherit',
        }}>{children}</button>
    );
}

export function GeneratingDots() {
    return (
        <span style={{ display: 'inline-flex', gap: 4 }}>
            {[0, 1, 2].map(i => (
                <span key={i} style={{
                    width: 6, height: 6, borderRadius: '50%',
                    background: 'linear-gradient(135deg, #6366f1, #14b8a6)',
                    animation: `aigAiBlip 1.1s ease-in-out ${i * 0.14}s infinite`,
                }} />
            ))}
        </span>
    );
}

export function DraftSkeleton({ delay }) {
    return (
        <div style={{
            padding: '14px 16px', borderRadius: 8, background: AIC.bg2,
            border: `1px solid ${AIC.border}`, margin: '0 20px 8px',
            animation: `aigSlideUp .5s ease ${delay}s both`, overflow: 'hidden', position: 'relative',
        }}>
            <div style={{
                height: 13, width: '55%',
                background: 'linear-gradient(90deg, rgba(99,102,241,0.15), rgba(20,184,166,0.15), rgba(99,102,241,0.15))',
                backgroundSize: '200% 100%', animation: 'aigShimmer 1.5s linear infinite',
                borderRadius: 3, marginBottom: 9,
            }} />
            <div style={{ height: 9, width: '80%', background: 'var(--aig-surface-tint-strong)', borderRadius: 3, marginBottom: 5 }} />
            <div style={{ height: 9, width: '70%', background: 'var(--aig-surface-tint-strong)', borderRadius: 3 }} />
        </div>
    );
}

export function Stepper({ steps }) {
    if (!steps || steps.length === 0) {
        return <div style={{ color: AIC.muted, fontSize: 12 }}>No steps yet.</div>;
    }
    return (
        <div>
            <div style={{
                display: 'grid', gridTemplateColumns: '26px 1fr 1fr', gap: 12,
                padding: '0 0 6px 0',
                fontSize: 9.5, color: AIC.muted, fontWeight: 600,
                letterSpacing: '0.08em', textTransform: 'uppercase',
                borderBottom: `1px solid ${AIC.border}`,
            }}>
                <div></div>
                <div>Action</div>
                <div>Expected</div>
            </div>
            {steps.map((s, i) => (
                <div key={i} style={{
                    display: 'grid', gridTemplateColumns: '26px 1fr 1fr', gap: 12, alignItems: 'start',
                    padding: '10px 0',
                    borderBottom: i < steps.length - 1 ? `1px solid ${AIC.border}` : 'none',
                }}>
                    <div style={{
                        width: 22, height: 22, borderRadius: '50%',
                        background: 'var(--aig-accent-soft-bg)',
                        border: '1px solid var(--aig-accent-soft-border)',
                        color: AIC.indigoSoft, fontSize: 11, fontWeight: 700,
                        display: 'flex', alignItems: 'center', justifyContent: 'center',
                        fontFamily: MONO,
                    }}>{i + 1}</div>
                    <div style={{ fontSize: 12.5, color: AIC.text, lineHeight: 1.45 }}>{decodeEntities(s.action)}</div>
                    <div style={{ fontSize: 12.5, color: AIC.text, lineHeight: 1.45 }}>
                        {decodeEntities(s.expected_result || '')}
                    </div>
                </div>
            ))}
        </div>
    );
}

export function MiniStat({ label, value }) {
    return (
        <div style={{ padding: '8px 10px', background: AIC.bg2, border: `1px solid ${AIC.border}`, borderRadius: 7 }}>
            <div style={{
                fontSize: 10, color: AIC.muted, textTransform: 'uppercase',
                letterSpacing: '0.08em', fontWeight: 600,
            }}>{label}</div>
            <div style={{
                fontSize: 15, color: AIC.text, fontWeight: 600,
                fontVariantNumeric: 'tabular-nums', marginTop: 2,
            }}>{value}</div>
        </div>
    );
}
