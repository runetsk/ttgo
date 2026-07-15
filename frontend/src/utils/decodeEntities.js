// Decode the HTML entities bluemonday produces (& < > " ') back to plain text,
// for the plain-text AI-draft views (compare modal, step list, draft editor).
// Node-safe (pure string ops) so draftDiff.js stays testable under node:test.
// The decoded text is only ever rendered as PLAIN text (React re-escapes it),
// so reviving markup here cannot cause XSS. `&amp;` is decoded last so a
// double-escaped input like "&amp;lt;" decodes exactly one level.
export function decodeEntities(s) {
    if (!s) return s;
    return String(s)
        .replace(/&#39;/g, "'")
        .replace(/&#34;/g, '"')
        .replace(/&quot;/g, '"')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>')
        .replace(/&nbsp;/g, ' ')
        .replace(/&amp;/g, '&');
}
