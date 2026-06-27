import React, { useState, useEffect } from 'react';
import { getRunFolders } from '../../api';
import { formatDate } from './utils';

const PRESETS = [
    { label: '7d', days: 7 },
    { label: '14d', days: 14 },
    { label: '30d', days: 30 },
    { label: '60d', days: 60 },
    { label: '90d', days: 90 },
];

function daysAgo(n) {
    const d = new Date();
    d.setDate(d.getDate() - n);
    d.setHours(0, 0, 0, 0);
    return d;
}

export default function AnalyticsFilters({ value, onChange }) {
    const [folders, setFolders] = useState([]);
    const [activePreset, setActivePreset] = useState(30);

    useEffect(() => {
        getRunFolders()
            .then(data => setFolders(Array.isArray(data) ? data : data?.folders || []))
            .catch(() => setFolders([]));
    }, []);

    const handlePreset = (days) => {
        setActivePreset(days);
        onChange({
            ...value,
            startDate: formatDate(daysAgo(days)),
            endDate: formatDate(new Date()),
        });
    };

    const handleCustomDate = (field, val) => {
        setActivePreset(null);
        onChange({ ...value, [field]: val });
    };

    const handleFolder = (folderId) => {
        onChange({ ...value, folderId: folderId || '' });
    };

    const handleClear = () => {
        setActivePreset(30);
        onChange({
            startDate: formatDate(daysAgo(30)),
            endDate: formatDate(new Date()),
            folderId: '',
        });
    };

    return (
        <div className="analytics-filters">
            <div className="analytics-filters-row">
                <div className="analytics-filters-presets">
                    {PRESETS.map(p => (
                        <button
                            key={p.days}
                            className={`analytics-preset-btn ${activePreset === p.days ? 'active' : ''}`}
                            onClick={() => handlePreset(p.days)}
                            type="button"
                        >
                            {p.label}
                        </button>
                    ))}
                </div>

                <div className="analytics-filters-dates">
                    <input
                        type="date"
                        className="modern-input analytics-date-input"
                        value={value.startDate || ''}
                        onChange={e => handleCustomDate('startDate', e.target.value)}
                    />
                    <span className="analytics-date-separator">to</span>
                    <input
                        type="date"
                        className="modern-input analytics-date-input"
                        value={value.endDate || ''}
                        onChange={e => handleCustomDate('endDate', e.target.value)}
                    />
                </div>

                <select
                    className="modern-select analytics-folder-select"
                    value={value.folderId || ''}
                    onChange={e => handleFolder(e.target.value)}
                >
                    <option value="">All Folders</option>
                    {folders.map(f => (
                        <option key={f.id} value={f.id}>{f.name}</option>
                    ))}
                </select>

                <button className="action-btn analytics-clear-btn" onClick={handleClear} type="button">
                    Clear
                </button>
            </div>
        </div>
    );
}
