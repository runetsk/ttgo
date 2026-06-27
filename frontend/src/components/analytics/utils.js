/**
 * Formats a Date object as YYYY-MM-DD string.
 */
export function formatDate(d) {
    return d.toISOString().slice(0, 10);
}

/**
 * Formats a date string as a relative timestamp (e.g., "3 hours ago").
 * Returns the full ISO date as fallback.
 */
export function relativeTime(dateStr) {
    if (!dateStr) return '—';
    const date = new Date(dateStr);
    if (isNaN(date.getTime())) return dateStr;

    const now = new Date();
    const diffMs = now - date;

    if (diffMs < 0) return 'just now';
    if (diffMs < 60000) return 'just now';

    const minutes = Math.floor(diffMs / 60000);
    if (minutes < 60) return `${minutes}m ago`;

    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;

    const days = Math.floor(hours / 24);
    if (days < 30) return `${days}d ago`;

    const months = Math.floor(days / 30);
    if (months < 12) return `${months}mo ago`;

    return `${Math.floor(months / 12)}y ago`;
}

/**
 * Format milliseconds as human-readable duration.
 */
export function formatDuration(ms) {
    if (!ms && ms !== 0) return '—';
    if (ms < 1000) return `${ms}ms`;

    const seconds = Math.floor(ms / 1000);
    if (seconds < 60) return `${seconds}s`;

    const minutes = Math.floor(seconds / 60);
    const remainSec = seconds % 60;
    if (minutes < 60) return `${minutes}m ${remainSec}s`;

    const hours = Math.floor(minutes / 60);
    const remainMin = minutes % 60;
    return `${hours}h ${remainMin}m ${remainSec}s`;
}
