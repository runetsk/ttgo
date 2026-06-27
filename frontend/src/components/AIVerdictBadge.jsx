import React from 'react';

const COLORS = {
    product_bug:    { bg: '#3d2020', fg: '#ff9d9d' },
    flaky_test:     { bg: '#3d3320', fg: '#ffd780' },
    environment:    { bg: '#203d30', fg: '#90e0a8' },
    test_data:      { bg: '#2d2d3d', fg: '#a5b4fc' },
    infrastructure: { bg: '#203548', fg: '#8fcfff' },
    unknown:        { bg: '#2a2a2a', fg: '#a0a0a0' },
};

const LABELS = {
    product_bug:    'Product bug',
    flaky_test:     'Flaky',
    environment:    'Environment',
    test_data:      'Test data',
    infrastructure: 'Infrastructure',
    unknown:        'Unknown',
};

const CONF_SHORT = { low: 'low', medium: 'med', high: 'high' };

export default function AIVerdictBadge({ verdict, confidence, dedupGroup, style }) {
    const color = COLORS[verdict] || COLORS.unknown;
    return (
        <span style={{
            background: color.bg, color: color.fg,
            padding: '2px 8px', borderRadius: 10, fontSize: 11,
            whiteSpace: 'nowrap', ...style,
        }}>
            {LABELS[verdict] || verdict}
            {confidence ? ` · ${CONF_SHORT[confidence] || confidence}` : ''}
            {dedupGroup ? <span style={{ marginLeft: 6, opacity: 0.7 }}>↳</span> : null}
        </span>
    );
}
