// Debounce drafts per result and serialize writes so a delayed step save cannot
// overwrite a newer verdict. Failed drafts remain retryable when navigation flushes.
export function createExecutionSaves(write, { delay = 500, onError = () => {} } = {}) {
    const jobs = new Map();
    const jobFor = (id) => {
        if (!jobs.has(id)) jobs.set(id, { tail: Promise.resolve(), draft: null, failed: null, timer: null, active: 0, version: 0 });
        return jobs.get(id);
    };
    const cancelTimer = (job) => {
        if (job.timer) clearTimeout(job.timer);
        job.timer = null;
    };
    const enqueue = (id, entry) => {
        const job = jobFor(id);
        job.active++;
        const result = job.tail.catch(() => {}).then(() => write(id, entry.patch)).then(
            (value) => { if (entry.version === job.version) job.failed = null; return value; },
            (error) => {
                if (entry.version === job.version) { job.failed = entry; onError(error); }
                throw error;
            },
        ).finally(() => { job.active--; });
        job.tail = result;
        // Timers and navigation may inspect the result later; always attach a
        // rejection handler immediately, while returning the rejection to callers.
        result.catch(() => {});
        return result;
    };
    const save = (id, patch) => {
        const job = jobFor(id);
        cancelTimer(job);
        job.draft = null;
        job.failed = null;
        return enqueue(id, { patch, version: ++job.version });
    };
    const flush = (id) => Promise.all([...jobs].filter(([key]) => !id || key === id).map(([key, job]) => {
        cancelTimer(job);
        const entry = job.draft || job.failed;
        job.draft = null;
        job.failed = null;
        return entry ? enqueue(key, entry) : job.tail;
    }));
    return {
        save,
        flush,
        hasPending: () => [...jobs.values()].some(job => job.draft || job.failed || job.active > 0),
        schedule(id, patch) {
            const job = jobFor(id);
            cancelTimer(job);
            job.failed = null;
            job.draft = { patch, version: ++job.version };
            job.timer = setTimeout(() => { flush(id).catch(() => {}); }, delay);
        },
    };
}
