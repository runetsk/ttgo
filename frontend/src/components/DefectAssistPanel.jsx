import React, { useEffect, useState } from 'react';
import { getTypeSafeUses, assistDefect } from '../api';
import { severityText, canApplySeverity, duplicateLabel, duplicatesSummary, defectLink } from '../utils/defectAssist';

// DefectAssistPanel offers "Check with TypeSafe.ai" in a defect form when defect assist is
// available: a severity suggestion to apply and open defects that may be the same problem.
// Nothing changes until the person clicks Apply. Renders nothing while the use is unavailable.
export default function DefectAssistPanel({ title, description, testCaseId, runResultId, errorMessage, severity, onApplySeverity, disabled }) {
    const [available, setAvailable] = useState(false);
    const [checking, setChecking] = useState(false);
    const [result, setResult] = useState(null);
    useEffect(() => {
        let alive = true;
        getTypeSafeUses().then((u) => { if (alive) setAvailable(!!u?.defect_assist); }).catch(() => {});
        return () => { alive = false; };
    }, []);
    if (!available) return null;

    const check = async () => {
        setChecking(true);
        try {
            setResult(await assistDefect({ title, description, test_case_id: testCaseId || '', run_result_id: runResultId || '', error_message: errorMessage || '' }));
        } catch {
            // toasted by the API interceptor
        } finally {
            setChecking(false);
        }
    };
    const sev = severityText(result);
    return (
        <div style={box} data-testid="defect-assist">
            <button type="button" className="action-btn" onClick={check} disabled={disabled || checking || !title?.trim()}
                data-testid="defect-assist-check" style={{ padding: '4px 12px', fontSize: '0.78rem' }}>
                {checking ? 'Checking…' : result ? 'Check again' : 'Check with TypeSafe.ai'}
            </button>
            {result && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 8 }}>
                    {sev && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }} data-testid="defect-assist-severity">
                            <span style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{sev}</span>
                            {canApplySeverity(result, severity) && (
                                <button type="button" className="action-btn" data-testid="defect-assist-apply"
                                    onClick={() => onApplySeverity?.(result.severity.value)} style={{ padding: '2px 10px', fontSize: '0.74rem' }}>
                                    Apply
                                </button>
                            )}
                        </div>
                    )}
                    <div data-testid="defect-assist-duplicates">
                        <div>{duplicatesSummary(result)}</div>
                        {result.duplicates?.length > 0 && (
                            <ul style={{ margin: '4px 0 0', paddingLeft: 18 }}>
                                {result.duplicates.map((d) => (
                                    <li key={d.defect_id}>
                                        <a href={defectLink(d.defect_id)} target="_blank" rel="noreferrer" data-testid={`defect-assist-dup-${d.defect_id}`}
                                            style={{ color: 'var(--aig-tone-indigo-fg)' }}>
                                            {duplicateLabel(d)}
                                        </a>
                                    </li>
                                ))}
                            </ul>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}

const box = {
    background: 'rgba(99,102,241,0.06)', border: '1px solid rgba(99,102,241,0.22)', borderRadius: 6,
    padding: '8px 10px', marginBottom: 12, fontSize: '0.8rem', color: 'var(--text-secondary)',
};
