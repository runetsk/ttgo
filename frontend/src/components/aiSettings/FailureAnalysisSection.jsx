import React, { useMemo, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { diagramModel } from '../../utils/analysisFlow';
import AnalysisFlowDiagram from './AnalysisFlowDiagram';
import TypeSafeSettingsCard from './TypeSafeSettingsCard';
import AIFailureAnalysisSettings from '../AIFailureAnalysisSettings';
import { s } from './styles';

// FailureAnalysisSection groups what shapes failure analysis on the AI Generation tab: the process
// diagram on top, then the TypeSafe.ai and AI Failure Analysis cards. The cards keep their own
// loading, validation and saving and report their state here, so the diagram follows unsaved edits.
export default function FailureAnalysisSection({ isAdmin }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [trigger, setTrigger] = useState('manual');
    const [typesafeCard, setTypesafeCard] = useState({ status: 'loading' });
    const [faCard, setFaCard] = useState({ status: 'loading' });

    const model = useMemo(() => diagramModel({
        aiEnabled: aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger,
    }), [aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger]);

    return (
        <div style={{ ...s.page, marginTop: 40 }} data-testid="failure-analysis-section">
            <div style={s.pageHeader}>
                <div style={s.pageHeaderIcon}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                    </svg>
                </div>
                <div>
                    <h3 style={s.pageTitle}>AI Failure Analysis</h3>
                    <p style={s.pageDesc}>
                        How failing results are grouped, decided and explained, and which services see the failure text.
                    </p>
                </div>
            </div>
            <AnalysisFlowDiagram model={model} trigger={trigger} onTriggerChange={setTrigger} />
            <TypeSafeSettingsCard isAdmin={isAdmin} onStateChange={setTypesafeCard} />
            <AIFailureAnalysisSettings isAdmin={isAdmin} onStateChange={setFaCard} />
        </div>
    );
}
