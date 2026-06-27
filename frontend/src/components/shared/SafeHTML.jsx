import { sanitizeHTML } from '../../utils/sanitize';

/**
 * Renders server-provided HTML with client-side sanitization (defense-in-depth).
 * Use instead of raw dangerouslySetInnerHTML.
 */
export default function SafeHTML({ html, as: Tag = 'div', ...props }) {
    return <Tag {...props} dangerouslySetInnerHTML={{ __html: sanitizeHTML(html) }} />;
}
