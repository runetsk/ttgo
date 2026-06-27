/**
 * Strips all HTML tags from a string and returns plain text.
 * Uses the browser's DOM parser for reliable, safe extraction.
 * Used in list/sidebar views to display plain-text previews of
 * fields that may contain HTML markup (FR-009).
 *
 * @param {string} html - The HTML string to strip
 * @returns {string} Plain text with all tags removed
 */
export function stripHtml(html) {
  if (!html) return '';
  const doc = new DOMParser().parseFromString(html, 'text/html');
  return doc.body.textContent || '';
}
