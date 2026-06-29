import React from 'react';

// Shared sidebar folder icons — used by both the library sidebar (FolderNode)
// and the test-runs sidebar (RunFolderSidebar) so the two stay visually in sync.

export const ChevronSvg = () => (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="9 18 15 12 9 6" />
    </svg>
);

export const FolderSvg = ({ open }) => (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        {open ? (
            <>
                <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2H3z" />
                <path d="M3 9h18l-2 9a2 2 0 0 1-2 1.5H5a2 2 0 0 1-2-1.5z" />
            </>
        ) : (
            <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
        )}
    </svg>
);

// "All Runs" entry icon — a layered/stack glyph, distinct from a folder so the
// aggregate "all" entry stays visually separable from real folders.
export const AllRunsSvg = () => (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polygon points="12 2 2 7 12 12 22 7 12 2" />
        <polyline points="2 17 12 22 22 17" />
        <polyline points="2 12 12 17 22 12" />
    </svg>
);
