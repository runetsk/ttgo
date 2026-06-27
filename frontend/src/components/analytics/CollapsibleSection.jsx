import React, { useState, useRef } from 'react';

export default function CollapsibleSection({ title, description, defaultExpanded = false, onFirstExpand, children }) {
    const [expanded, setExpanded] = useState(defaultExpanded);
    const hasExpandedOnce = useRef(defaultExpanded);

    const toggle = () => {
        const next = !expanded;
        setExpanded(next);
        if (next && !hasExpandedOnce.current) {
            hasExpandedOnce.current = true;
            if (onFirstExpand) onFirstExpand();
        }
    };

    return (
        <div className="analytics-collapsible">
            <button className="analytics-collapsible-header" data-expanded={expanded} onClick={toggle} type="button">
                <span className="analytics-collapsible-chevron" data-expanded={expanded}>
                    &#9654;
                </span>
                <span className="analytics-collapsible-title">{title}</span>
                {description && (
                    <span className="analytics-collapsible-description">{description}</span>
                )}
            </button>
            <div
                className="analytics-collapsible-body"
                style={{
                    display: expanded ? 'block' : 'none',
                }}
            >
                {(expanded || hasExpandedOnce.current) && children}
            </div>
        </div>
    );
}
