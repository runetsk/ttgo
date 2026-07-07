import React, { useState, useEffect } from 'react';
import { getAssignableUsers, assignRun } from '../api';
import { toast } from '../toast';

// Compact assignee dropdown for a run. Lazily loads the assignable-users list
// on mount. Calls onAssigned(newAssigneeId) after a successful change.
export default function AssigneePicker({ runId, assigneeId, assigneeName, onAssigned }) {
    const [users, setUsers] = useState(null);
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        let cancelled = false;
        getAssignableUsers()
            .then(list => { if (!cancelled) setUsers(list); })
            .catch(() => { if (!cancelled) setUsers([]); });
        return () => { cancelled = true; };
    }, []);

    const handleChange = async (e) => {
        const next = e.target.value || null;
        setSaving(true);
        try {
            await assignRun(runId, next);
            if (onAssigned) onAssigned(next);
        } catch {
            toast.error('Failed to assign run');
        } finally {
            setSaving(false);
        }
    };

    // Show the current assignee even if they are not in the assignable list (inactive).
    const options = users || [];
    const hasCurrent = assigneeId && options.some(u => u.id === assigneeId);

    return (
        <select
            className="modern-select"
            data-testid="run-assignee-picker"
            value={assigneeId || ''}
            disabled={saving || users === null}
            onChange={handleChange}
            style={{ maxWidth: 200, fontSize: '0.8rem' }}
            title="Assignee"
        >
            <option value="">Unassigned</option>
            {!hasCurrent && assigneeId && (
                <option value={assigneeId}>{assigneeName || assigneeId}</option>
            )}
            {options.map(u => (
                <option key={u.id} value={u.id}>{u.display_name || u.email}</option>
            ))}
        </select>
    );
}
