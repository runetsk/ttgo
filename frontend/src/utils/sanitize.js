/**
 * Lightweight client-side HTML sanitizer using DOMParser.
 * Defense-in-depth: backend already sanitizes with bluemonday.
 * Strips <script>, <iframe>, <object>, <embed>, <form>, on* attributes, and javascript: URIs.
 */
const DANGEROUS_TAGS = new Set(['SCRIPT', 'IFRAME', 'OBJECT', 'EMBED', 'FORM', 'LINK', 'META']);
const DANGEROUS_ATTR_RE = /^on/i;
const DANGEROUS_URI_RE = /^\s*javascript:/i;

export function sanitizeHTML(html) {
    if (!html || typeof html !== 'string') return '';

    const doc = new DOMParser().parseFromString(html, 'text/html');
    const walk = (node) => {
        const children = [...node.childNodes];
        for (const child of children) {
            if (child.nodeType === Node.ELEMENT_NODE) {
                if (DANGEROUS_TAGS.has(child.tagName)) {
                    child.remove();
                    continue;
                }
                // Remove dangerous attributes
                for (const attr of [...child.attributes]) {
                    if (DANGEROUS_ATTR_RE.test(attr.name) || DANGEROUS_URI_RE.test(attr.value)) {
                        child.removeAttribute(attr.name);
                    }
                }
                walk(child);
            }
        }
    };
    walk(doc.body);
    return doc.body.innerHTML;
}
