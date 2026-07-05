// Shared end-of-run summary writer. k6 calls the main script's exported
// handleSummary(data) after the run and writes the returned {path: content}
// map — REPLACING the default terminal summary, which is why textSummary is
// re-added under the 'stdout' key. This supersedes the runner's deprecated
// --summary-export flag.
//
// Shape note: this file's summary.json nests metric values under `.values`
// (data.metrics[name].values['p(95)']) — different from the old
// --summary-export layout. scripts/analyze.sh reads server.log, not this
// file, so only ad-hoc summary consumers are affected.
import { textSummary } from './vendor/k6-summary.js';

export function summaryFiles(data) {
  const files = { stdout: textSummary(data, { indent: ' ', enableColors: true }) };
  if (__ENV.TTGO_SUMMARY_PATH) {
    files[__ENV.TTGO_SUMMARY_PATH] = JSON.stringify(data, null, 2);
  }
  return files;
}

export function handleSummary(data) {
  return summaryFiles(data);
}
