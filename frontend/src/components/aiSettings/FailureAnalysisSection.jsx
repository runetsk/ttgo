import React, { useMemo, useState } from 'react';
import { useAIGeneration } from '../../contexts/AIGenerationContext';
import { diagramModel } from '../../utils/analysisFlow';
import AnalysisFlowDiagram from './AnalysisFlowDiagram';
import TypeSafeSettingsCard from './TypeSafeSettingsCard';
import AIFailureAnalysisSettings from '../AIFailureAnalysisSettings';

// FailureAnalysisSection is the Failure analysis tab of Settings → AI: the process diagram on top,
// then the TypeSafe.ai and AI Failure Analysis cards. The cards keep their own loading, validation
// and saving (registering with the page save bar) and report their state here, so the diagram
// follows unsaved edits.
export default function FailureAnalysisSection({ isAdmin }) {
    const { aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus } = useAIGeneration();
    const [trigger, setTrigger] = useState('manual');
    const [typesafeCard, setTypesafeCard] = useState({ status: 'loading' });
    const [faCard, setFaCard] = useState({ status: 'loading' });

    const model = useMemo(() => diagramModel({
        aiEnabled: aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger,
    }), [aiFeaturesEnabled, aiFeaturesStatus, providers, providersStatus, typesafeCard, faCard, trigger]);

    return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }} data-testid="failure-analysis-section">
            <AnalysisFlowDiagram model={model} trigger={trigger} onTriggerChange={setTrigger} />
            <TypeSafeSettingsCard isAdmin={isAdmin} onStateChange={setTypesafeCard} />
            <AIFailureAnalysisSettings isAdmin={isAdmin} onStateChange={setFaCard} />
        </div>
    );
}
